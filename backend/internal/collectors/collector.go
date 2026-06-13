package collectors

import (
	"context"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

type Result struct {
	Status       string
	ResponseTime *float64
	PacketLoss   *float64
	CPUUsage     *float64
	MemoryUsage  *float64
	Bandwidth    *float64
	Details      map[string]any
}

type Collector interface {
	Name() string
	Collect(ctx context.Context, device *models.Device) (*Result, error)
}

type Registry struct {
	m map[string]Collector
}

func NewRegistry() *Registry { return &Registry{m: map[string]Collector{}} }

func (r *Registry) Register(c Collector) { r.m[c.Name()] = c }

func (r *Registry) Get(name string) (Collector, bool) {
	c, ok := r.m[name]
	return c, ok
}

func f64(v float64) *float64 { return &v }
