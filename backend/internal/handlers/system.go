package handlers

import (
	"math"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/httputil"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
)

type SystemHandler struct{}

func NewSystemHandler() *SystemHandler { return &SystemHandler{} }

func (h *SystemHandler) Info(w http.ResponseWriter, r *http.Request) {
	cpuPercent := 0.0
	cpuPercents, err := cpu.Percent(time.Second, false)
	if err == nil && len(cpuPercents) > 0 {
		cpuPercent = math.Round(cpuPercents[0]*10) / 10
	}
	cpuCores, _ := cpu.Counts(true)

	cpuModel := ""
	cpuInfo, err := cpu.Info()
	if err == nil && len(cpuInfo) > 0 {
		cpuModel = cpuInfo[0].ModelName
	}

	memStat, err := mem.VirtualMemory()
	var memUsedGB, memTotalGB, memPercent float64
	if err == nil && memStat != nil {
		memTotalGB = math.Round(float64(memStat.Total)/(1024*1024*1024)*10) / 10
		memUsedGB = math.Round(float64(memStat.Used)/(1024*1024*1024)*10) / 10
		memPercent = math.Round(memStat.UsedPercent*10) / 10
	}

	diskStat, err := disk.Usage("/")
	var diskUsedGB, diskTotalGB, diskPercent float64
	if err == nil && diskStat != nil {
		diskTotalGB = math.Round(float64(diskStat.Total)/(1024*1024*1024)*10) / 10
		diskUsedGB = math.Round(float64(diskStat.Used)/(1024*1024*1024)*10) / 10
		diskPercent = math.Round(diskStat.UsedPercent*10) / 10
	}

	loadAvg, err := load.Avg()
	var load1, load5, load15 float64
	if err == nil && loadAvg != nil {
		load1 = math.Round(loadAvg.Load1*100) / 100
		load5 = math.Round(loadAvg.Load5*100) / 100
		load15 = math.Round(loadAvg.Load15*100) / 100
	}

	uptimeSec, _ := host.Uptime()
	hostname, _ := os.Hostname()

	httputil.SendOK(w, map[string]any{
		"cpu": map[string]any{
			"usage": cpuPercent,
			"cores": cpuCores,
			"model": cpuModel,
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
		"uptime":       uptimeSec,
		"loadAvg":      []float64{load1, load5, load15},
		"hostname":     hostname,
		"goVersion":    runtime.Version(),
		"numCPU":       runtime.NumCPU(),
		"numGoroutine": runtime.NumGoroutine(),
	})
}
