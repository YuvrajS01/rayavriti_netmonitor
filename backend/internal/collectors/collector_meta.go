package collectors

import (
	"context"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

type CollectorMeta interface {
	DefaultTimeout() time.Duration
	Weight() int
}

type TypedCollector struct {
	name    string
	collect func(ctx context.Context, device *models.Device) (*Result, error)
	timeout time.Duration
	weight  int
}

func NewTypedCollector(name string, fn func(ctx context.Context, device *models.Device) (*Result, error), timeout time.Duration, weight int) *TypedCollector {
	return &TypedCollector{
		name:    name,
		collect: fn,
		timeout: timeout,
		weight:  weight,
	}
}

func (tc *TypedCollector) Name() string {
	return tc.name
}

func (tc *TypedCollector) Collect(ctx context.Context, device *models.Device) (*Result, error) {
	return tc.collect(ctx, device)
}

func (tc *TypedCollector) DefaultTimeout() time.Duration {
	return tc.timeout
}

func (tc *TypedCollector) Weight() int {
	return tc.weight
}
