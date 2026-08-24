package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKeyedMutex_SameKeySerialized(t *testing.T) {
	t.Parallel()
	km := newKeyedMutex()

	// Two goroutines contending on the same key: the critical sections must
	// never overlap.
	var active, maxActive int64
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			unlock := km.lock("a:b")
			defer unlock()
			cur := atomic.AddInt64(&active, 1)
			for {
				old := atomic.LoadInt64(&maxActive)
				if cur <= old || atomic.CompareAndSwapInt64(&maxActive, old, cur) {
					break
				}
			}
			time.Sleep(time.Millisecond)
			atomic.AddInt64(&active, -1)
		}()
		go func() {
			defer wg.Done()
			unlock := km.lock("a:b")
			defer unlock()
			cur := atomic.AddInt64(&active, 1)
			for {
				old := atomic.LoadInt64(&maxActive)
				if cur <= old || atomic.CompareAndSwapInt64(&maxActive, old, cur) {
					break
				}
			}
			time.Sleep(time.Millisecond)
			atomic.AddInt64(&active, -1)
		}()
	}
	wg.Wait()
	assert.Equal(t, int64(1), atomic.LoadInt64(&maxActive))
}

func TestKeyedMutex_DifferentKeysConcurrent(t *testing.T) {
	t.Parallel()
	km := newKeyedMutex()

	var active int64
	var maxActive int64
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			unlock := km.lock(string(rune('a' + i)))
			defer unlock()
			cur := atomic.AddInt64(&active, 1)
			for {
				old := atomic.LoadInt64(&maxActive)
				if cur <= old || atomic.CompareAndSwapInt64(&maxActive, old, cur) {
					break
				}
			}
			time.Sleep(2 * time.Millisecond)
			atomic.AddInt64(&active, -1)
		}(i)
	}
	wg.Wait()
	// With 8 distinct keys all held concurrently, concurrency should exceed 1.
	assert.Greater(t, atomic.LoadInt64(&maxActive), int64(1))
}

func TestRuleLockKey(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "42:7", ruleLockKey(42, 7))
	assert.Equal(t, "1:1", ruleLockKey(1, 1))
}

func TestAlertEngine_StopWithoutStart(t *testing.T) {
	t.Parallel()
	db := &mockDB{}
	engine := NewAlertEngine(db, nil, nil)
	// Notification queue is open but never drained; Stop must not deadlock.
	engine.sendNotifications(context.Background(), &models.AlertRule{ID: 1, ChannelIDs: []int64{1}}, &models.Alert{ID: 1})
	engine.Stop()
}

func TestAlertEngine_NotificationQueueFullDrops(t *testing.T) {
	t.Parallel()
	db := &mockDB{}
	engine := NewAlertEngine(db, nil, nil)
	// Drain nothing; fill the bounded queue past capacity.
	rule := &models.AlertRule{ID: 1, ChannelIDs: []int64{1}}
	for i := 0; i < notifQueueSize+10; i++ {
		engine.sendNotifications(context.Background(), rule, &models.Alert{ID: int64(i)})
	}
	// sendNotifications must never block.
	engine.Stop()
}

func TestAlertEngine_AsyncNotificationDelivery(t *testing.T) {
	// Not parallel: exercises the worker pool.
	var hits int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	db := &mockDB{
		getNotificationChannelsFn: func(ctx context.Context) ([]models.NotificationChannel, error) {
			return []models.NotificationChannel{
				{ID: 1, Type: "webhook", Enabled: true, Config: map[string]any{"url": server.URL}},
			}, nil
		},
	}
	engine := NewAlertEngine(db, nil, NewNotifier())
	engine.Start(context.Background())
	defer engine.Stop()

	rule := &models.AlertRule{ID: 1, ChannelIDs: []int64{1}}
	alert := &models.Alert{ID: 7, DeviceID: 3, DeviceName: "Router-1", Severity: "critical", Message: "Down"}

	engine.sendNotifications(context.Background(), rule, alert)

	require.Eventually(t, func() bool {
		return atomic.LoadInt64(&hits) == 1
	}, 3*time.Second, 20*time.Millisecond)
}

func TestAlertEngine_NotificationWorkerRespectsContext(t *testing.T) {
	t.Parallel()
	engine := NewAlertEngine(&mockDB{}, nil, NewNotifier())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// The worker pool must terminate cleanly once ctx is cancelled even when
	// the queue is left open; this guards against goroutine leaks.
	engine.Start(ctx)
	done := make(chan struct{})
	go func() {
		engine.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("workers did not exit after context cancellation")
	}
}
