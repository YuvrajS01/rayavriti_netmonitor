package metrics

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

// poolCollector exports pgxpool.Stat at scrape time.
type poolCollector struct {
	pool *pgxpool.Pool

	acquired, idle, total, max *prometheus.Desc
	acquireCount, emptyAcquire *prometheus.Desc
	acquireDuration            *prometheus.Desc
}

// RegisterPool exports connection-pool statistics for pool.
func RegisterPool(pool *pgxpool.Pool) {
	d := func(name, help string) *prometheus.Desc {
		return prometheus.NewDesc(prometheus.BuildFQName(namespace, "db_pool", name), help, nil, nil)
	}
	Registry.MustRegister(&poolCollector{
		pool:            pool,
		acquired:        d("acquired_conns", "Connections currently checked out."),
		idle:            d("idle_conns", "Idle connections."),
		total:           d("total_conns", "Total open connections."),
		max:             d("max_conns", "Configured maximum connections."),
		acquireCount:    d("acquire_total", "Successful connection acquires."),
		emptyAcquire:    d("empty_acquire_total", "Acquires that had to wait because the pool was empty."),
		acquireDuration: d("acquire_duration_seconds_total", "Total time spent waiting to acquire connections."),
	})
}

func (c *poolCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{c.acquired, c.idle, c.total, c.max, c.acquireCount, c.emptyAcquire, c.acquireDuration} {
		ch <- d
	}
}

func (c *poolCollector) Collect(ch chan<- prometheus.Metric) {
	s := c.pool.Stat()
	ch <- prometheus.MustNewConstMetric(c.acquired, prometheus.GaugeValue, float64(s.AcquiredConns()))
	ch <- prometheus.MustNewConstMetric(c.idle, prometheus.GaugeValue, float64(s.IdleConns()))
	ch <- prometheus.MustNewConstMetric(c.total, prometheus.GaugeValue, float64(s.TotalConns()))
	ch <- prometheus.MustNewConstMetric(c.max, prometheus.GaugeValue, float64(s.MaxConns()))
	ch <- prometheus.MustNewConstMetric(c.acquireCount, prometheus.CounterValue, float64(s.AcquireCount()))
	ch <- prometheus.MustNewConstMetric(c.emptyAcquire, prometheus.CounterValue, float64(s.EmptyAcquireCount()))
	ch <- prometheus.MustNewConstMetric(c.acquireDuration, prometheus.CounterValue, s.AcquireDuration().Seconds())
}
