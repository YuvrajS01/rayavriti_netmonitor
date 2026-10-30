package collectors

import (
	"context"
	"net/http"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// HTTPSCollector polls HTTPS endpoints. Like HTTPCollector, client is only
// overridden in tests.
type HTTPSCollector struct {
	client *http.Client
}

func (HTTPSCollector) Name() string { return "https" }

func (c HTTPSCollector) Collect(ctx context.Context, device *models.Device) (*Result, error) {
	// Delegate to HTTPCollector — it already detects https based on protocol/port
	return HTTPCollector(c).Collect(ctx, device)
}
