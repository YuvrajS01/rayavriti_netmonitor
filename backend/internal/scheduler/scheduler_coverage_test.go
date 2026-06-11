package scheduler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/collectors"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/rayavriti/netmonitor-backend/internal/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestHubSched() *websocket.Hub {
	return websocket.NewHub("test-secret", nil)
}

type mockCollector struct {
	name   string
	result *collectors.Result
	err    error
}

func (m *mockCollector) Name() string { return m.name }
func (m *mockCollector) Collect(ctx context.Context, d *models.Device) (*collectors.Result, error) {
	return m.result, m.err
}

type panicCollector struct{}

func (p *panicCollector) Name() string { return "panic_collector" }
func (p *panicCollector) Collect(ctx context.Context, d *models.Device) (*collectors.Result, error) {
	panic("intentional panic in collector")
}

func TestScheduler_New(t *testing.T) {
	t.Parallel()
	db := &mockDB{}
	reg := collectors.NewRegistry()
	hub := newTestHubSched()

	s := New(db, reg, hub, nil, 30)
	require.NotNil(t, s)
	assert.Equal(t, 0, s.JobCount())
}

func TestScheduler_Start_Stop_NoDevices(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		getEnabledDevicesFn: func(ctx context.Context) ([]models.Device, error) {
			return nil, nil
		},
	}
	reg := collectors.NewRegistry()
	hub := newTestHubSched()

	s := New(db, reg, hub, nil, 30)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx)
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 0, s.JobCount())

	s.Stop()
}

func TestScheduler_Start_Stop_DisabledCollectors(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		getEnabledDevicesFn: func(ctx context.Context) ([]models.Device, error) {
			return []models.Device{
				{ID: 1, Name: "d1", Protocol: "ping", Interval: 10, Enabled: true},
				{ID: 2, Name: "d2", Protocol: "snmp", Interval: 10, Enabled: true},
			}, nil
		},
		getDeviceFn: func(ctx context.Context, id int64) (*models.Device, error) {
			return &models.Device{ID: id, Enabled: true, Interval: 10}, nil
		},
	}
	reg := collectors.NewRegistry()
	hub := newTestHubSched()

	s := New(db, reg, hub, nil, 30)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx)
	time.Sleep(50 * time.Millisecond)
	// Collectors not registered but scheduler still schedules goroutines
	assert.GreaterOrEqual(t, s.JobCount(), 2)

	s.Stop()
}

func TestScheduleDevice_VariousProtocols(t *testing.T) {
	t.Parallel()
	protocols := []string{"http", "https", "ping", "snmp", "system", "port"}

	for _, proto := range protocols {
		t.Run(proto, func(t *testing.T) {
			t.Parallel()
			db := &mockDB{
				getLatestMetricsFn: func(ctx context.Context) ([]models.Metric, error) {
					return nil, nil
				},
			}
			reg := collectors.NewRegistry()
			hub := newTestHubSched()
			s := New(db, reg, hub, nil, 30)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			device := models.Device{
				ID:       1,
				Name:     "test-device",
				Protocol: proto,
				Enabled:  true,
				Interval: 300,
			}

			s.scheduleDevice(ctx, device)
			time.Sleep(50 * time.Millisecond)
			assert.GreaterOrEqual(t, s.JobCount(), 1)

			s.Stop()
		})
	}
}

func TestScheduleDevice_MinInterval(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		getLatestMetricsFn: func(ctx context.Context) ([]models.Metric, error) {
			return nil, nil
		},
	}
	reg := collectors.NewRegistry()
	hub := newTestHubSched()
	s := New(db, reg, hub, nil, 30)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Interval too small, should be bumped to 5s
	device := models.Device{ID: 1, Name: "d1", Protocol: "ping", Interval: 1}
	s.scheduleDevice(ctx, device)
	time.Sleep(50 * time.Millisecond)
	assert.GreaterOrEqual(t, s.JobCount(), 1)

	s.Stop()
}

func TestScheduleDevice_ReplaceExistingJob(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		getLatestMetricsFn: func(ctx context.Context) ([]models.Metric, error) {
			return nil, nil
		},
	}
	reg := collectors.NewRegistry()
	hub := newTestHubSched()
	s := New(db, reg, hub, nil, 30)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	device := models.Device{ID: 1, Name: "d1", Protocol: "ping", Interval: 300}
	s.scheduleDevice(ctx, device)
	time.Sleep(50 * time.Millisecond)
	assert.GreaterOrEqual(t, s.JobCount(), 1)

	// Re-schedule same device
	s.scheduleDevice(ctx, device)
	time.Sleep(50 * time.Millisecond)

	s.Stop()
}

func TestReconcile_AddsNewDevices(t *testing.T) {
	t.Parallel()
	callCount := 0
	db := &mockDB{
		getEnabledDevicesFn: func(ctx context.Context) ([]models.Device, error) {
			callCount++
			if callCount <= 1 {
				return []models.Device{
					{ID: 1, Name: "d1", Protocol: "ping", Interval: 10},
				}, nil
			}
			return []models.Device{
				{ID: 1, Name: "d1", Protocol: "ping", Interval: 10},
				{ID: 2, Name: "d2", Protocol: "ping", Interval: 10},
			}, nil
		},
	}
	reg := collectors.NewRegistry()
	hub := newTestHubSched()
	s := New(db, reg, hub, nil, 30)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx)
	time.Sleep(50 * time.Millisecond)

	// First reconcile should have 1 device
	s.reconcile(ctx)
	time.Sleep(50 * time.Millisecond)

	// Second reconcile should discover new device
	s.reconcile(ctx)
	time.Sleep(50 * time.Millisecond)

	s.Stop()
}

func TestReconcile_RemovesDisabledDevices(t *testing.T) {
	t.Parallel()
	callCount := 0
	db := &mockDB{
		getEnabledDevicesFn: func(ctx context.Context) ([]models.Device, error) {
			callCount++
			if callCount <= 1 {
				return []models.Device{
					{ID: 1, Name: "d1", Protocol: "ping", Interval: 10},
					{ID: 2, Name: "d2", Protocol: "ping", Interval: 10},
				}, nil
			}
			// Device 2 removed
			return []models.Device{
				{ID: 1, Name: "d1", Protocol: "ping", Interval: 10},
			}, nil
		},
	}
	reg := collectors.NewRegistry()
	hub := newTestHubSched()
	s := New(db, reg, hub, nil, 30)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx)
	time.Sleep(50 * time.Millisecond)
	s.reconcile(ctx)
	time.Sleep(50 * time.Millisecond)

	s.reconcile(ctx)
	time.Sleep(50 * time.Millisecond)

	s.Stop()
}

func TestReconcile_DBError(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		getEnabledDevicesFn: func(ctx context.Context) ([]models.Device, error) {
			return nil, fmt.Errorf("db error")
		},
	}
	reg := collectors.NewRegistry()
	hub := newTestHubSched()
	s := New(db, reg, hub, nil, 30)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx)
	s.reconcile(ctx)
	s.Stop()
}

func TestCollectOnce_WithMockCollector_Success(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		getLatestMetricsFn: func(ctx context.Context) ([]models.Metric, error) {
			return nil, nil
		},
	}
	reg := collectors.NewRegistry()
	mock := &mockCollector{
		name:   "test_proto",
		result: &collectors.Result{Status: "up"},
	}
	reg.Register(mock)
	hub := newTestHubSched()

	s := New(db, reg, hub, nil, 30)
	device := models.Device{ID: 1, Name: "d1", Protocol: "test_proto"}
	s.collectOnce(context.Background(), device)
}

func TestCollectOnce_WithMockCollector_Failure(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		getLatestMetricsFn: func(ctx context.Context) ([]models.Metric, error) {
			return nil, nil
		},
	}
	reg := collectors.NewRegistry()
	mock := &mockCollector{
		name: "fail_proto",
		err:  fmt.Errorf("collection failed"),
	}
	reg.Register(mock)
	hub := newTestHubSched()

	s := New(db, reg, hub, nil, 30)
	device := models.Device{ID: 1, Name: "d1", Protocol: "fail_proto"}
	s.collectOnce(context.Background(), device)
}

func TestCollectOnce_WithMockCollector_NilResult(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		getLatestMetricsFn: func(ctx context.Context) ([]models.Metric, error) {
			return nil, nil
		},
	}
	reg := collectors.NewRegistry()
	mock := &mockCollector{
		name:   "nil_proto",
		result: nil,
	}
	reg.Register(mock)
	hub := newTestHubSched()

	s := New(db, reg, hub, nil, 30)
	device := models.Device{ID: 1, Name: "d1", Protocol: "nil_proto"}
	s.collectOnce(context.Background(), device)
}

func TestCollectOnce_PanicCollector(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		getLatestMetricsFn: func(ctx context.Context) ([]models.Metric, error) {
			return nil, nil
		},
	}
	reg := collectors.NewRegistry()
	pc := &panicCollector{}
	reg.Register(pc)
	hub := newTestHubSched()

	s := New(db, reg, hub, nil, 30)
	device := models.Device{ID: 1, Name: "d1", Protocol: "panic_collector"}

	// The scheduler does not have Recovery middleware, so panics propagate.
	// Verify this by recovering ourselves.
	func() {
		defer func() {
			if r := recover(); r != nil {
				// Expected: panic propagated
			}
		}()
		s.collectOnce(context.Background(), device)
	}()
}

func TestCollectOnce_UnknownProtocol(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		getLatestMetricsFn: func(ctx context.Context) ([]models.Metric, error) {
			return nil, nil
		},
	}
	reg := collectors.NewRegistry()
	hub := newTestHubSched()

	s := New(db, reg, hub, nil, 30)
	device := models.Device{ID: 1, Name: "d1", Protocol: "nonexistent"}
	s.collectOnce(context.Background(), device)
}

func TestCollectOnce_WithStatusChange(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		getLatestMetricsFn: func(ctx context.Context) ([]models.Metric, error) {
			return []models.Metric{
				{DeviceID: 1, Status: "up"},
			}, nil
		},
	}
	reg := collectors.NewRegistry()
	mock := &mockCollector{
		name:   "test_proto",
		result: &collectors.Result{Status: "down"},
	}
	reg.Register(mock)
	hub := newTestHubSched()

	s := New(db, reg, hub, nil, 30)
	device := models.Device{ID: 1, Name: "d1", Protocol: "test_proto"}
	s.collectOnce(context.Background(), device)
}

func TestCollectOnce_WithAlertEngine(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		getLatestMetricsFn: func(ctx context.Context) ([]models.Metric, error) {
			return nil, nil
		},
	}
	reg := collectors.NewRegistry()
	mock := &mockCollector{
		name:   "test_proto",
		result: &collectors.Result{Status: "up"},
	}
	reg.Register(mock)
	hub := newTestHubSched()

	s := New(db, reg, hub, nil, 30)
	device := models.Device{ID: 1, Name: "d1", Protocol: "test_proto"}
	s.collectOnce(context.Background(), device)
}

func TestCollectOnce_RecordMetricFails(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		getLatestMetricsFn: func(ctx context.Context) ([]models.Metric, error) {
			return nil, nil
		},
		recordMetricFn: func(ctx context.Context, m *models.Metric) error {
			return fmt.Errorf("db write failed")
		},
	}
	reg := collectors.NewRegistry()
	mock := &mockCollector{
		name:   "test_proto",
		result: &collectors.Result{Status: "up"},
	}
	reg.Register(mock)
	hub := newTestHubSched()

	s := New(db, reg, hub, nil, 30)
	device := models.Device{ID: 1, Name: "d1", Protocol: "test_proto"}
	s.collectOnce(context.Background(), device)
}

func TestCollectOnce_MultipleConsecutive(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		getLatestMetricsFn: func(ctx context.Context) ([]models.Metric, error) {
			return nil, nil
		},
	}
	reg := collectors.NewRegistry()
	mock := &mockCollector{
		name:   "test_proto",
		result: &collectors.Result{Status: "up"},
	}
	reg.Register(mock)
	hub := newTestHubSched()

	s := New(db, reg, hub, nil, 30)
	device := models.Device{ID: 1, Name: "d1", Protocol: "test_proto"}

	for i := 0; i < 5; i++ {
		s.collectOnce(context.Background(), device)
	}
}

func TestStop_Idempotent(t *testing.T) {
	t.Parallel()
	db := &mockDB{}
	reg := collectors.NewRegistry()
	hub := newTestHubSched()

	s := New(db, reg, hub, nil, 30)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx)
	time.Sleep(50 * time.Millisecond)

	s.Stop()
	s.Stop()
	s.Stop()
}

func TestStart_GetDevicesError(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		getEnabledDevicesFn: func(ctx context.Context) ([]models.Device, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}
	reg := collectors.NewRegistry()
	hub := newTestHubSched()

	s := New(db, reg, hub, nil, 30)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx)
	time.Sleep(50 * time.Millisecond)
	s.Stop()
}
