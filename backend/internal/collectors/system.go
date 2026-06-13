package collectors

import (
	"context"
	"math"
	"os"
	"runtime"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
)

type SystemCollector struct{}

func (SystemCollector) Name() string { return "system" }

func (SystemCollector) Collect(_ context.Context, _ *models.Device) (*Result, error) {
	// CPU usage (averaged over 1 second, per-CPU = false for total)
	cpuPercent := 0.0
	cpuPercents, err := cpu.Percent(time.Second, false)
	if err == nil && len(cpuPercents) > 0 {
		cpuPercent = math.Round(cpuPercents[0]*10) / 10
	}
	cpuCores, _ := cpu.Counts(true)

	// Memory
	memStat, err := mem.VirtualMemory()
	var memUsedGB, memTotalGB, memPercent float64
	if err == nil && memStat != nil {
		memTotalGB = math.Round(float64(memStat.Total)/(1024*1024*1024)*10) / 10
		memUsedGB = math.Round(float64(memStat.Used)/(1024*1024*1024)*10) / 10
		memPercent = math.Round(memStat.UsedPercent*10) / 10
	}

	// Disk
	diskStat, err := disk.Usage("/")
	var diskUsedGB, diskTotalGB, diskPercent float64
	if err == nil && diskStat != nil {
		diskTotalGB = math.Round(float64(diskStat.Total)/(1024*1024*1024)*10) / 10
		diskUsedGB = math.Round(float64(diskStat.Used)/(1024*1024*1024)*10) / 10
		diskPercent = math.Round(diskStat.UsedPercent*10) / 10
	}

	// Load average
	loadAvg, err := load.Avg()
	var load1, load5, load15 float64
	if err == nil && loadAvg != nil {
		load1 = math.Round(loadAvg.Load1*100) / 100
		load5 = math.Round(loadAvg.Load5*100) / 100
		load15 = math.Round(loadAvg.Load15*100) / 100
	}

	// Uptime
	uptimeSec, _ := host.Uptime()

	// Hostname
	hostname, _ := os.Hostname()

	systemInfo := map[string]any{
		"cpu": map[string]any{
			"usage": cpuPercent,
			"cores": cpuCores,
		},
		"memory": map[string]any{
			"used":    memUsedGB,
			"total":   memTotalGB,
			"percent": memPercent,
		},
		"disk": map[string]any{
			"used":    diskUsedGB,
			"total":   diskTotalGB,
			"percent": diskPercent,
		},
		"uptime": uptimeSec,
		"loadAvg": []float64{load1, load5, load15},
		"hostname": hostname,
		"goVersion": runtime.Version(),
		"numCPU": runtime.NumCPU(),
		"numGoroutine": runtime.NumGoroutine(),
	}

	status := "up"
	if cpuPercent > 90 || memPercent > 95 {
		status = "warning"
	}

	details := map[string]any{
		"cpu_usage":       cpuPercent,
		"cpu_cores":       cpuCores,
		"memory_percent":  memPercent,
		"disk_percent":    diskPercent,
		"uptime_seconds":  uptimeSec,
		"load_avg_1m":     load1,
		"load_avg_5m":     load5,
		"load_avg_15m":    load15,
		"system_info":     systemInfo,
	}

	return &Result{
		Status:       status,
		ResponseTime: f64(0),
		CPUUsage:     f64(cpuPercent),
		MemoryUsage:  f64(memPercent),
		Details:      details,
	}, nil
}
