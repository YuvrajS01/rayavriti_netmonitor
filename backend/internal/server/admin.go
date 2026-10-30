package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rayavriti/netmonitor-backend/internal/metrics"
)

// AdminServer exposes operational endpoints (/metrics, optional pprof) on a
// listener separate from the public API, so they can be bound to loopback or
// a management network without going through API auth.
type AdminServer struct {
	srv *http.Server
}

// StartAdmin starts the admin listener on addr. It returns nil when addr is
// empty (disabled).
func StartAdmin(addr string, enablePprof bool) *AdminServer {
	if addr == "" {
		return nil
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{}))
	if enablePprof {
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}
	a := &AdminServer{srv: &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}}
	go func() {
		if err := a.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("admin listener failed", "addr", addr, "error", err)
		}
	}()
	slog.Info("Admin listener started", "addr", addr, "pprof", enablePprof)
	return a
}

// Shutdown stops the admin listener. Safe on a nil receiver.
func (a *AdminServer) Shutdown(ctx context.Context) error {
	if a == nil {
		return nil
	}
	return a.srv.Shutdown(ctx)
}
