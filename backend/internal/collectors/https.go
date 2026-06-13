package collectors

import (
	"context"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

type HTTPSCollector struct{}

func (HTTPSCollector) Name() string { return "https" }

func (HTTPSCollector) Collect(ctx context.Context, device *models.Device) (*Result, error) {
	// Delegate to HTTPCollector — it already detects https based on protocol/port
	return HTTPCollector{}.Collect(ctx, device)
}
