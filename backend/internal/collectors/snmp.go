package collectors

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

const (
	oidSysUpTime       = ".1.3.6.1.2.1.1.3.0"
	oidSysDescr        = ".1.3.6.1.2.1.1.1.0"
	oidSysName         = ".1.3.6.1.2.1.1.5.0"
	oidHRProcessorLoad = ".1.3.6.1.2.1.25.3.3.1.2"
	oidHRStorageTable  = ".1.3.6.1.2.1.25.2.3.1"
	oidHRStorageType   = ".1.3.6.1.2.1.25.2.3.1.2"
	oidHRStorageDescr  = ".1.3.6.1.2.1.25.2.3.1.3"
	oidHRStorageSize   = ".1.3.6.1.2.1.25.2.3.1.5"
	oidHRStorageUsed   = ".1.3.6.1.2.1.25.2.3.1.6"
	oidHRStorageUnits  = ".1.3.6.1.2.1.25.2.3.1.4"
	oidIfTable         = ".1.3.6.1.2.1.2.2"
	oidIfXTable        = ".1.3.6.1.2.1.31.1.1"
)

var hrStorageRAM = ".1.3.6.1.2.1.25.2.1.2"
var hrStorageFixedDisk = ".1.3.6.1.2.1.25.2.1.4"

type SNMPCollector struct{}

func (SNMPCollector) Name() string { return "snmp" }

func (SNMPCollector) Collect(ctx context.Context, device *models.Device) (*Result, error) {
	community := "public"
	if device.SNMPCommunity != "" {
		community = device.SNMPCommunity
	}
	if community == "public" {
		slog.Warn("SNMP using default 'public' community string", "device", device.IPAddress)
	}
	port := uint16(161)
	if device.SNMPPort > 0 {
		port = uint16(device.SNMPPort) //nolint:gosec // SNMP port is always valid
	}

	version := gosnmp.Version2c
	if device.SNMPVersion == "1" {
		version = gosnmp.Version1
	}

	g := &gosnmp.GoSNMP{
		Target:    device.IPAddress,
		Port:      port,
		Community: community,
		Version:   version,
		Timeout:   5 * time.Second,
		Retries:   1,
	}

	start := time.Now()
	if err := g.Connect(); err != nil {
		return &Result{Status: "down", Details: map[string]any{"error": err.Error()}}, nil
	}
	defer func() { _ = g.Conn.Close() }()

	type varbindResult struct {
		pdus []gosnmp.SnmpPDU
		err  error
	}

	// Collect SNMP data sequentially (gosnmp is not goroutine-safe)
	uptimeRes := varbindResult{}
	if r, err := g.Get([]string{oidSysUpTime}); err != nil {
		uptimeRes.err = err
	} else {
		uptimeRes.pdus = r.Variables
	}

	pdus, err := collectSubtree(g, oidHRProcessorLoad, 20)
	cpuRes := varbindResult{pdus: pdus, err: err}

	t, err := collectTable(g, oidHRStorageTable, []int{2, 3, 4, 5, 6}, 20)
	storageRes := tableResult{table: t, err: err}

	t, err = collectTable(g, oidIfTable, []int{2, 5, 8, 10, 16}, 20)
	ifRes := tableResult{table: t, err: err}

	t, err = collectTable(g, oidIfXTable, []int{1, 6, 10, 15}, 20)
	ifXRes := tableResult{table: t, err: err}

	elapsed := float64(time.Since(start).Milliseconds())

	// Parse uptime
	var uptimeTicks float64
	if uptimeRes.err == nil && len(uptimeRes.pdus) > 0 {
		uptimeTicks = pduToFloat64(uptimeRes.pdus[0])
	}
	uptimeSeconds := int64(uptimeTicks / 100)

	// Parse CPU loads
	var cpuAvg float64
	var cpuCores int
	if cpuRes.err == nil {
		var totalLoad float64
		var count int
		for _, pdu := range cpuRes.pdus {
			val := pduToFloat64(pdu)
			if !math.IsNaN(val) && !math.IsInf(val, 0) && val >= 0 {
				totalLoad += val
				count++
			}
		}
		cpuCores = count
		if count > 0 {
			cpuAvg = math.Round(totalLoad/float64(count)*10) / 10
		}
	}

	// Parse storage
	var memoryPercent, diskPercent float64
	var memoryUsedGB, memoryTotalGB, diskUsedGB, diskTotalGB float64

	if storageRes.err == nil && storageRes.table != nil {
		// Find storage type OIDs from the table
		ramTotal, ramUsed := sumStorageByType(storageRes.table, hrStorageRAM)
		diskTotal, diskUsed := sumStorageByType(storageRes.table, hrStorageFixedDisk)

		if ramTotal > 0 {
			memoryPercent = math.Round(ramUsed/ramTotal*1000) / 10
			memoryTotalGB = math.Round(ramTotal/(1024*1024*1024)*10) / 10
			memoryUsedGB = math.Round(ramUsed/(1024*1024*1024)*10) / 10
		}
		if diskTotal > 0 {
			diskPercent = math.Round(diskUsed/diskTotal*1000) / 10
			diskTotalGB = math.Round(diskTotal/(1024*1024*1024)*10) / 10
			diskUsedGB = math.Round(diskUsed/(1024*1024*1024)*10) / 10
		}
	}

	// Parse interfaces
	var interfaces []map[string]any
	if ifRes.err == nil && ifXRes.err == nil {
		interfaces = collectInterfaces(ifRes.table, ifXRes.table)
	}

	// Determine status
	status := "up"
	if cpuAvg > 90 || memoryPercent > 95 || diskPercent > 95 {
		status = "warning"
	}

	resourceInfo := map[string]any{
		"cpu": map[string]any{
			"usage": cpuAvg,
			"cores": cpuCores,
		},
		"memory": map[string]any{
			"used":    memoryUsedGB,
			"total":   memoryTotalGB,
			"percent": memoryPercent,
		},
		"disk": map[string]any{
			"used":    diskUsedGB,
			"total":   diskTotalGB,
			"percent": diskPercent,
		},
		"uptime":     uptimeSeconds,
		"interfaces": interfaces,
	}

	details := map[string]any{
		"snmp_version":    device.SNMPVersion,
		"resource_info":   resourceInfo,
		"uptime_seconds":  uptimeSeconds,
		"cpu_usage":       cpuAvg,
		"memory_percent":  memoryPercent,
		"disk_percent":    diskPercent,
		"interface_count": len(interfaces),
	}

	return &Result{
		Status:       status,
		ResponseTime: f64(elapsed),
		CPUUsage:     f64(cpuAvg),
		MemoryUsage:  f64(memoryPercent),
		Details:      details,
	}, nil
}

type tableResult struct {
	table map[string]map[string]gosnmp.SnmpPDU
	err   error
}

func collectSubtree(g *gosnmp.GoSNMP, oid string, maxRepetitions int) ([]gosnmp.SnmpPDU, error) {
	var results []gosnmp.SnmpPDU
	err := g.Walk(oid, func(pdu gosnmp.SnmpPDU) error {
		results = append(results, pdu)
		return nil
	})
	return results, err
}

func collectTable(g *gosnmp.GoSNMP, oid string, columns []int, maxRepetitions int) (map[string]map[string]gosnmp.SnmpPDU, error) {
	table := make(map[string]map[string]gosnmp.SnmpPDU)
	for _, col := range columns {
		colOid := fmt.Sprintf("%s.%d", oid, col)
		err := g.Walk(colOid, func(pdu gosnmp.SnmpPDU) error {
			// Extract row index from OID: oid.col.index
			oidStr := pdu.Name
			// Find the column number and index
			baseLen := len(oid) + 1 // +1 for the dot
			if len(oidStr) > baseLen {
				rest := oidStr[baseLen:]
				// Find the next dot separator (column.index)
				for i, ch := range rest {
					if ch == '.' {
						rowIdx := rest[i+1:]
						colStr := rest[:i]
						if table[rowIdx] == nil {
							table[rowIdx] = make(map[string]gosnmp.SnmpPDU)
						}
						table[rowIdx][colStr] = pdu
						break
					}
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return table, nil
}

func pduToFloat64(pdu gosnmp.SnmpPDU) float64 {
	switch v := pdu.Value.(type) {
	case uint64:
		return float64(v)
	case uint32:
		return float64(v)
	case uint:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case []byte:
		if len(v) <= 8 {
			// Parse as big-endian unsigned
			var val uint64
			for _, b := range v {
				val = (val << 8) | uint64(b)
			}
			return float64(val)
		}
		return 0
	default:
		// Try via gosnmp ToBigInt
		bi := gosnmp.ToBigInt(v)
		if bi != nil {
			return float64(bi.Int64())
		}
		return 0
	}
}

func sumStorageByType(table map[string]map[string]gosnmp.SnmpPDU, typeOid string) (totalBytes float64, usedBytes float64) {
	for _, row := range table {
		// Column 2 = hrStorageType
		typePdu, ok := row["2"]
		if !ok {
			continue
		}
		rowType := fmt.Sprintf("%v", typePdu.Value)
		if rowType != typeOid {
			continue
		}

		// Column 4 = hrStorageUnits, Column 5 = hrStorageSize, Column 6 = hrStorageUsed
		units := pduToFloat64(row["4"])
		size := pduToFloat64(row["5"])
		used := pduToFloat64(row["6"])

		if units > 0 && size > 0 {
			totalBytes += units * size
			usedBytes += units * used
		}
	}
	return
}

func collectInterfaces(baseTable, xTable map[string]map[string]gosnmp.SnmpPDU) []map[string]any {
	type iface struct {
		index      int
		name       string
		inOctets   int64
		outOctets  int64
		speed      int64
		operStatus int
	}

	var interfaces []iface

	for idx, baseRow := range baseTable {
		var i iface
		fmt.Sscanf(idx, "%d", &i.index) //nolint:errcheck,gosec

		// ifTable column 2 = ifDescr, column 5 = ifSpeed, column 8 = ifOperStatus, column 10 = ifInOctets, column 16 = ifOutOctets
		if pdu, ok := baseRow["2"]; ok {
			if b, ok := pdu.Value.([]byte); ok {
				i.name = string(b)
			} else {
				i.name = fmt.Sprintf("if%d", i.index)
			}
		}
		i.speed = int64(pduToFloat64(baseRow["5"]))
		i.operStatus = int(pduToFloat64(baseRow["8"]))
		i.inOctets = int64(pduToFloat64(baseRow["10"]))
		i.outOctets = int64(pduToFloat64(baseRow["16"]))

		// ifXTable overrides: column 1 = ifName, column 6 = ifHCInOctets, column 10 = ifHCOutOctets, column 15 = ifHighSpeed
		idxStr := fmt.Sprintf("%d", i.index)
		if xRow, ok := xTable[idxStr]; ok {
			if pdu, ok := xRow["1"]; ok {
				if b, ok := pdu.Value.([]byte); ok {
					i.name = string(b)
				}
			}
			if hcIn := int64(pduToFloat64(xRow["6"])); hcIn > 0 {
				i.inOctets = hcIn
			}
			if hcOut := int64(pduToFloat64(xRow["10"])); hcOut > 0 {
				i.outOctets = hcOut
			}
			if hs := int64(pduToFloat64(xRow["15"])); hs > 0 {
				i.speed = hs * 1000000 // ifHighSpeed is in Mbps
			}
		}

		// Only include active interfaces or those with traffic
		if i.operStatus == 1 || i.inOctets > 0 || i.outOctets > 0 {
			interfaces = append(interfaces, i)
		}
	}

	// Sort by total traffic (descending), take top 12
	sort.Slice(interfaces, func(i, j int) bool {
		totalI := interfaces[i].inOctets + interfaces[i].outOctets
		totalJ := interfaces[j].inOctets + interfaces[j].outOctets
		return totalI > totalJ
	})
	if len(interfaces) > 12 {
		interfaces = interfaces[:12]
	}

	var result []map[string]any
	for _, i := range interfaces {
		result = append(result, map[string]any{
			"index":      i.index,
			"name":       i.name,
			"inOctets":   i.inOctets,
			"outOctets":  i.outOctets,
			"speed":      i.speed,
			"operStatus": i.operStatus,
		})
	}
	return result
}

func protoNameFromNum(p int) string {
	switch p {
	case 1:
		return "ICMP"
	case 2:
		return "IGMP"
	case 6:
		return "TCP"
	case 17:
		return "UDP"
	case 47:
		return "GRE"
	case 50:
		return "ESP"
	case 51:
		return "AH"
	case 58:
		return "ICMPv6"
	case 89:
		return "OSPF"
	case 132:
		return "SCTP"
	default:
		return fmt.Sprintf("PROTO_%d", p)
	}
}

// NormalizeCounter converts SNMP counter values to int64.
func NormalizeCounter(value any) int64 {
	if value == nil {
		return 0
	}
	switch v := value.(type) {
	case uint64:
		return int64(v) //nolint:gosec // Counter value from SNMP, bounded by uint64
	case uint32:
		return int64(v)
	case int:
		return int64(v)
	case int64:
		return v
	case []byte:
		if len(v) <= 8 {
			var val uint64
			for _, b := range v {
				val = (val << 8) | uint64(b)
			}
			return int64(val)
		}
		return 0
	default:
		return int64(pduToFloat64(gosnmp.SnmpPDU{Value: value}))
	}
}

// GetNetflowProtoName returns a human-readable protocol name from a number.
func GetNetflowProtoName(num int) string {
	return protoNameFromNum(num)
}
