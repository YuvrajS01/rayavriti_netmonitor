package collectors

import (
	"context"
	"errors"
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
	// Table walks address columns as <entry>.<column>, so these are the
	// ifEntry / ifXEntry OIDs (table + ".1"). Using the table OIDs walked
	// the wrong subtrees and returned no interfaces from real agents.
	oidIfTable  = ".1.3.6.1.2.1.2.2.1"
	oidIfXTable = ".1.3.6.1.2.1.31.1.1.1"
)

var hrStorageRAM = ".1.3.6.1.2.1.25.2.1.2"
var hrStorageFixedDisk = ".1.3.6.1.2.1.25.2.1.4"

type SNMPCollector struct{}

func (SNMPCollector) Name() string { return "snmp" }

// DefaultTimeout bounds a whole SNMP poll. Without it the scheduler's 30s
// fallback applied, letting one unreachable agent hold a worker for 30s.
func (SNMPCollector) DefaultTimeout() time.Duration { return 15 * time.Second }

// Weight implements CollectorMeta.
func (SNMPCollector) Weight() int { return 3 }

func (SNMPCollector) Collect(ctx context.Context, device *models.Device) (*Result, error) {
	g, err := newSNMPClient(device)
	if err != nil {
		return &Result{Status: "down", Details: map[string]any{"error": err.Error()}}, nil
	}

	start := time.Now()
	if err := g.Connect(); err != nil {
		return &Result{Status: "down", Details: map[string]any{"error": err.Error()}}, nil
	}
	defer func() { _ = g.Conn.Close() }()

	// gosnmp has no context-aware I/O, so a watch goroutine closes the
	// connection when the caller's context expires; the in-flight Get/Walk
	// then fail immediately instead of occupying a worker past the deadline.
	watchdogDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = g.Conn.Close()
		case <-watchdogDone:
		}
	}()
	defer close(watchdogDone)

	type varbindResult struct {
		pdus []gosnmp.SnmpPDU
		err  error
	}

	// Collect SNMP data sequentially (gosnmp is not goroutine-safe)
	// sysUpTime doubles as the reachability probe. UDP "connects" always
	// succeed, so without this an unreachable agent was reported "up" with
	// zeroed metrics and every table walk below ran into its timeout.
	uptimeRes := varbindResult{}
	r, err := g.Get([]string{oidSysUpTime})
	if err != nil {
		return &Result{Status: "down", Details: map[string]any{"error": "snmp agent unreachable: " + err.Error()}}, nil
	}
	if r.Error != gosnmp.NoError || len(r.Variables) == 0 || !hasValue(r.Variables[0]) {
		return &Result{Status: "down", Details: map[string]any{"error": "snmp agent returned no sysUpTime"}}, nil
	}
	uptimeRes.pdus = r.Variables

	pdus, err := collectSubtree(g, oidHRProcessorLoad, 20)
	cpuRes := varbindResult{pdus: pdus, err: err}

	t, err := collectTable(g, oidHRStorageTable, []int{2, 3, 4, 5, 6}, 20)
	storageRes := tableResult{table: t, err: err}

	t, err = collectTable(g, oidIfTable, []int{2, 5, 8, 10, 16}, 40)
	ifRes := tableResult{table: t, err: err}

	t, err = collectTable(g, oidIfXTable, []int{1, 6, 10, 15}, 40)
	ifXRes := tableResult{table: t, err: err}

	var neighbors []models.LLDPNeighbor
	var neighborsOK bool
	if now := time.Now(); lldpDue.due(device.ID, now) {
		neighbors, neighborsOK = collectLLDP(g, device.ID, now)
	}

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

	// Parse interfaces. ifXTable (64-bit counters, ifName) is optional:
	// some agents only implement ifTable.
	var interfaces []map[string]any
	var samples []models.InterfaceSample
	var totalBps *float64
	if ifRes.err == nil {
		xTable := ifXRes.table
		if ifXRes.err != nil {
			xTable = nil
		}
		interfaces = collectInterfaces(ifRes.table, xTable)
		samples = interfaceSamples(device.ID, time.Now(), ifRes.table, xTable)
		byIndex := make(map[int]models.InterfaceSample, len(samples))
		var sum float64
		for _, s := range samples {
			byIndex[s.IfIndex] = s
			sum += s.InBps + s.OutBps
		}
		for _, row := range interfaces {
			if idx, ok := row["index"].(int); ok {
				if s, ok := byIndex[idx]; ok {
					row["inBps"], row["outBps"] = s.InBps, s.OutBps
					if s.Utilization != nil {
						row["utilization"] = *s.Utilization
					}
				}
			}
		}
		if len(samples) > 0 {
			totalBps = &sum
		}
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
		Bandwidth:    totalBps,
		Interfaces:   samples,

		Neighbors:          neighbors,
		NeighborsCollected: neighborsOK,
	}, nil
}

type tableResult struct {
	table map[string]map[string]gosnmp.SnmpPDU
	err   error
}

// newSNMPClient builds a gosnmp client for the device's configured version:
// v1, v2c (the default) or v3 with USM credentials.
func newSNMPClient(device *models.Device) (*gosnmp.GoSNMP, error) {
	port := uint16(161)
	if device.SNMPPort > 0 {
		port = uint16(device.SNMPPort) //nolint:gosec // SNMP port is always valid
	}
	g := &gosnmp.GoSNMP{
		Target:  device.IPAddress,
		Port:    port,
		Version: gosnmp.Version2c,
		Timeout: 5 * time.Second,
		Retries: 1,
	}

	switch device.SNMPVersion {
	case "3":
		if device.SNMPv3 == nil {
			return nil, errors.New("snmpv3 credentials are not configured")
		}
		if err := device.SNMPv3.Validate(); err != nil {
			return nil, err
		}
		g.Version = gosnmp.Version3
		g.SecurityModel = gosnmp.UserSecurityModel
		g.ContextName = device.SNMPv3.ContextName
		g.MsgFlags, g.SecurityParameters = usmParams(device.SNMPv3)
		return g, nil
	case "1":
		g.Version = gosnmp.Version1
	}
	g.Community = "public"
	if device.SNMPCommunity != "" {
		g.Community = device.SNMPCommunity
	} else {
		slog.Debug("SNMP using default 'public' community string", "device", device.IPAddress)
	}
	return g, nil
}

var (
	snmpAuthProtocols = map[string]gosnmp.SnmpV3AuthProtocol{
		"MD5": gosnmp.MD5, "SHA": gosnmp.SHA, "SHA224": gosnmp.SHA224,
		"SHA256": gosnmp.SHA256, "SHA384": gosnmp.SHA384, "SHA512": gosnmp.SHA512,
	}
	snmpPrivProtocols = map[string]gosnmp.SnmpV3PrivProtocol{
		"DES": gosnmp.DES, "AES": gosnmp.AES, "AES192": gosnmp.AES192,
		"AES256": gosnmp.AES256, "AES192C": gosnmp.AES192C, "AES256C": gosnmp.AES256C,
	}
)

// usmParams maps a validated SNMPv3Config to gosnmp message flags and USM
// security parameters.
func usmParams(c *models.SNMPv3Config) (gosnmp.SnmpV3MsgFlags, *gosnmp.UsmSecurityParameters) {
	sp := &gosnmp.UsmSecurityParameters{UserName: c.SecurityName}
	switch c.SecurityLevel {
	case models.SNMPv3AuthPriv:
		sp.AuthenticationProtocol = snmpAuthProtocols[c.AuthProtocol]
		sp.AuthenticationPassphrase = c.AuthPassphrase
		sp.PrivacyProtocol = snmpPrivProtocols[c.PrivProtocol]
		sp.PrivacyPassphrase = c.PrivPassphrase
		return gosnmp.AuthPriv, sp
	case models.SNMPv3AuthNoPriv:
		sp.AuthenticationProtocol = snmpAuthProtocols[c.AuthProtocol]
		sp.AuthenticationPassphrase = c.AuthPassphrase
		sp.PrivacyProtocol = gosnmp.NoPriv
		return gosnmp.AuthNoPriv, sp
	default:
		sp.AuthenticationProtocol = gosnmp.NoAuth
		sp.PrivacyProtocol = gosnmp.NoPriv
		return gosnmp.NoAuthNoPriv, sp
	}
}

// walk visits every varbind under oid. SNMPv2c/v3 use GETBULK, fetching
// MaxRepetitions rows per round trip; v1 has no GETBULK and falls back to
// GETNEXT, one round trip per value (B4).
func walk(g *gosnmp.GoSNMP, oid string, maxRepetitions int, fn gosnmp.WalkFunc) error {
	if g.Version == gosnmp.Version1 {
		return g.Walk(oid, fn)
	}
	if maxRepetitions > 0 {
		g.MaxRepetitions = uint32(maxRepetitions) //nolint:gosec // small positive constant
	}
	return g.BulkWalk(oid, fn)
}

func collectSubtree(g *gosnmp.GoSNMP, oid string, maxRepetitions int) ([]gosnmp.SnmpPDU, error) {
	var results []gosnmp.SnmpPDU
	err := walk(g, oid, maxRepetitions, func(pdu gosnmp.SnmpPDU) error {
		results = append(results, pdu)
		return nil
	})
	return results, err
}

func collectTable(g *gosnmp.GoSNMP, oid string, columns []int, maxRepetitions int) (map[string]map[string]gosnmp.SnmpPDU, error) {
	table := make(map[string]map[string]gosnmp.SnmpPDU)
	for _, col := range columns {
		colOid := fmt.Sprintf("%s.%d", oid, col)
		err := walk(g, colOid, maxRepetitions, func(pdu gosnmp.SnmpPDU) error {
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

// hasValue reports whether a varbind carries data rather than an SNMP
// exception (noSuchObject, noSuchInstance, endOfMibView) or NULL.
func hasValue(pdu gosnmp.SnmpPDU) bool {
	switch pdu.Type {
	case gosnmp.NoSuchObject, gosnmp.NoSuchInstance, gosnmp.EndOfMibView, gosnmp.Null:
		return false
	}
	return true
}
