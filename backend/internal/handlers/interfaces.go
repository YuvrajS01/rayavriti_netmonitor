package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// InterfaceHandler serves per-interface throughput collected by SNMP polls.
// Routes are mounted behind DeviceScopeGuard, so the device is already known
// to be visible to the caller.
type InterfaceHandler struct {
	pool *pgxpool.Pool
}

func NewInterfaceHandler(pool *pgxpool.Pool) *InterfaceHandler {
	return &InterfaceHandler{pool: pool}
}

// Latest returns the most recent sample of each interface seen in the last
// hour, ordered by ifIndex.
func (h *InterfaceHandler) Latest(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(r, "id")
	if !ok {
		httputil.SendError(w, http.StatusBadRequest, "invalid id")
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT DISTINCT ON (if_index) time, device_id, if_index, if_name, in_bps, out_bps, utilization, oper_status, speed_bps
		FROM interface_metrics
		WHERE device_id = $1 AND time > NOW() - INTERVAL '1 hour'
		ORDER BY if_index, time DESC`, id)
	if err != nil {
		httputil.SendInternalError(w, err)
		return
	}
	samples, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.InterfaceSample, error) {
		var s models.InterfaceSample
		var util *float32
		var oper int16
		err := row.Scan(&s.Time, &s.DeviceID, &s.IfIndex, &s.Name, &s.InBps, &s.OutBps, &util, &oper, &s.SpeedBps)
		if util != nil {
			u := float64(*util)
			s.Utilization = &u
		}
		s.OperStatus = int(oper)
		return s, err
	})
	if err != nil {
		httputil.SendInternalError(w, err)
		return
	}
	if samples == nil {
		samples = []models.InterfaceSample{}
	}
	httputil.SendOK(w, samples)
}

type interfacePoint struct {
	T              time.Time `json:"t"`
	InBps          float64   `json:"inBps"`
	OutBps         float64   `json:"outBps"`
	MaxInBps       float64   `json:"maxInBps"`
	MaxOutBps      float64   `json:"maxOutBps"`
	MaxUtilization *float64  `json:"maxUtilization"`
}

// History returns bucketed throughput for one interface: averages for the
// line and maxima so short bursts stay visible at coarse buckets.
func (h *InterfaceHandler) History(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(r, "id")
	if !ok {
		httputil.SendError(w, http.StatusBadRequest, "invalid id")
		return
	}
	ifIndex, err := strconv.Atoi(chi.URLParam(r, "ifIndex"))
	if err != nil || ifIndex < 0 {
		httputil.SendError(w, http.StatusBadRequest, "invalid interface index")
		return
	}
	q := r.URL.Query()
	to := time.Now()
	from := to.Add(-6 * time.Hour)
	if v := q.Get("from"); v != "" {
		if from, err = time.Parse(time.RFC3339, v); err != nil {
			httputil.SendError(w, http.StatusBadRequest, "from must be RFC3339")
			return
		}
	}
	if v := q.Get("to"); v != "" {
		if to, err = time.Parse(time.RFC3339, v); err != nil {
			httputil.SendError(w, http.StatusBadRequest, "to must be RFC3339")
			return
		}
	}
	if !from.Before(to) {
		httputil.SendError(w, http.StatusBadRequest, "from must be before to")
		return
	}
	bucket := q.Get("bucket")
	if bucket == "" {
		bucket = pickBucket(to.Sub(from))
	}
	width, ok := historyBuckets[bucket]
	if !ok {
		httputil.SendError(w, http.StatusBadRequest, "bucket must be one of 1m, 5m, 15m, 1h, 1d")
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT time_bucket($4::interval, time) AS b,
		       AVG(in_bps), AVG(out_bps), MAX(in_bps), MAX(out_bps), MAX(utilization)
		FROM interface_metrics
		WHERE device_id = $1 AND if_index = $2 AND time >= $3 AND time < $5
		GROUP BY b ORDER BY b`, id, ifIndex, from, width.String(), to)
	if err != nil {
		httputil.SendInternalError(w, err)
		return
	}
	points, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (interfacePoint, error) {
		var p interfacePoint
		var util *float32
		err := row.Scan(&p.T, &p.InBps, &p.OutBps, &p.MaxInBps, &p.MaxOutBps, &util)
		if util != nil {
			u := float64(*util)
			p.MaxUtilization = &u
		}
		return p, err
	})
	if err != nil {
		httputil.SendInternalError(w, err)
		return
	}
	if points == nil {
		points = []interfacePoint{}
	}
	httputil.SendOK(w, points)
}
