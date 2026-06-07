package collectors

import (
	"context"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

type SystemCollector struct{}

func (SystemCollector) Name() string { return "system" }

func (SystemCollector) Collect(_ context.Context, _ *models.Device) (*Result, error) {
	return &Result{Status: "up", Details: map[string]any{"note": "system metrics stub"}}, nil
}
