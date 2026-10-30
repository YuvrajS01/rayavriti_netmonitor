package engine

import (
	"net/http"
	"time"
)

// newTestNotifier returns a Notifier that may reach httptest servers on
// loopback, which the production SSRF guard deliberately refuses.
func newTestNotifier() *Notifier {
	return &Notifier{client: &http.Client{Timeout: 10 * time.Second}}
}
