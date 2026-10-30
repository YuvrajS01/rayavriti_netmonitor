// Package metrics defines the Prometheus collectors NetMonitor exports about
// itself. Everything is registered on Registry (not the global default), which
// is served by the admin listener in server.StartAdmin.
package metrics

import (
	"runtime"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const namespace = "netmonitor"

// Registry holds every NetMonitor collector plus Go runtime and process stats.
var Registry = prometheus.NewRegistry()

var factory = promauto.With(Registry)

func init() {
	Registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
}

var (
	// BuildInfo is always 1; its labels identify the running build.
	BuildInfo = factory.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace, Name: "build_info",
		Help: "Build information (value is always 1).",
	}, []string{"version", "goversion"})

	// Polling.
	PollsTotal = factory.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace, Subsystem: "poller", Name: "polls_total",
		Help: "Completed device polls by protocol and resulting status.",
	}, []string{"protocol", "status"})
	PollDuration = factory.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace, Subsystem: "poller", Name: "poll_duration_seconds",
		Help:    "Wall-clock duration of a device poll, by protocol.",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30},
	}, []string{"protocol"})
	PollsDropped = factory.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace, Subsystem: "poller", Name: "enqueue_dropped_total",
		Help: "Poll jobs dropped because their priority queue was full.",
	}, []string{"priority"})
	PollsSkippedInFlight = factory.NewCounter(prometheus.CounterOpts{
		Namespace: namespace, Subsystem: "poller", Name: "skipped_in_flight_total",
		Help: "Due polls skipped because the device's previous poll had not finished.",
	})

	// Result pipeline.
	PipelineBatchDuration = factory.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace, Subsystem: "pipeline", Name: "batch_duration_seconds",
		Help:    "Time to persist and evaluate one batch of poll results.",
		Buckets: prometheus.DefBuckets,
	})
	PipelineBatchSize = factory.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace, Subsystem: "pipeline", Name: "batch_size",
		Help:    "Number of poll results per pipeline batch.",
		Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500},
	})

	// WebSocket.
	WSMessagesDropped = factory.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace, Subsystem: "websocket", Name: "messages_dropped_total",
		Help: "WebSocket messages dropped: broadcast_full (hub queue) or slow_client (per-client buffer).",
	}, []string{"reason"})

	// Notifications.
	NotificationsTotal = factory.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace, Subsystem: "notifications", Name: "deliveries_total",
		Help: "Notification delivery attempts by channel type and result (sent, failed, dropped).",
	}, []string{"channel", "result"})

	// HTTP API.
	HTTPRequestDuration = factory.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace, Subsystem: "http", Name: "request_duration_seconds",
		Help:    "HTTP request latency by method, route pattern and status class.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route", "code"})
)

// SetBuildInfo records the running version.
func SetBuildInfo(version string) {
	BuildInfo.WithLabelValues(version, runtime.Version()).Set(1)
}

// RegisterGaugeFunc registers a gauge whose value is read at scrape time,
// for state another package already tracks (queue depth, client counts).
func RegisterGaugeFunc(subsystem, name, help string, fn func() float64) {
	factory.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: namespace, Subsystem: subsystem, Name: name, Help: help,
	}, fn)
}
