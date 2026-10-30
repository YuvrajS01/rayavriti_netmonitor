package scheduler

import (
	"container/heap"
	"context"
	"testing"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

func TestPollDispatcher_SkipsDevicesStillInFlight(t *testing.T) {
	// Pool is never started, so enqueued jobs stay queued (in flight).
	wp := NewWorkerPool(WorkerPoolConfig{WorkerCount: 1}, nil)
	d := NewPollDispatcher(wp, time.Second)
	d.Upsert(models.Device{ID: 1}, 1, 5*time.Second)

	forceDue := func() {
		d.mu.Lock()
		d.deviceMap[1].NextPollAt = time.Now().Add(-time.Millisecond)
		heap.Fix(d.schedule, d.deviceMap[1].index)
		d.mu.Unlock()
	}

	d.dispatchDue()
	forceDue()
	d.dispatchDue()
	if got := len(wp.normalQ); got != 1 {
		t.Fatalf("queued %d jobs for one in-flight device, want 1", got)
	}

	d.Done(1)
	forceDue()
	d.dispatchDue()
	if got := len(wp.normalQ); got != 2 {
		t.Fatalf("queued %d jobs after the previous poll finished, want 2", got)
	}
}

func TestPollDispatcher_SplaysInitialLoadOnly(t *testing.T) {
	wp := NewWorkerPool(WorkerPoolConfig{WorkerCount: 1}, nil)
	d := NewPollDispatcher(wp, time.Second)
	interval := time.Minute
	now := time.Now()

	d.BeginInitialLoad()
	for id := int64(1); id <= 50; id++ {
		d.Upsert(models.Device{ID: id}, 1, interval)
	}
	d.EndInitialLoad()
	d.Upsert(models.Device{ID: 999}, 1, interval)

	spread := 0
	for id := int64(1); id <= 50; id++ {
		at := d.deviceMap[id].NextPollAt
		if at.Before(now) || at.After(now.Add(interval+time.Second)) {
			t.Fatalf("device %d scheduled outside its interval: %v", id, at.Sub(now))
		}
		if at.Sub(now) > 5*time.Second {
			spread++
		}
	}
	if spread < 40 {
		t.Fatalf("only %d/50 devices were spread past 5s; start-up splay is not working", spread)
	}
	if late := d.deviceMap[999].NextPollAt.Sub(now); late > time.Second {
		t.Fatalf("device added after start-up waited %v, want immediate", late)
	}
}

func TestWorkerPool_StopLetsInFlightPollFinish(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	results := make(chan PollResult, 1)
	wp := NewWorkerPool(WorkerPoolConfig{WorkerCount: 1}, func(ctx context.Context, job PollJob) PollResult {
		close(started)
		<-release
		return PollResult{Device: job.Device, Status: "up", Error: ctx.Err()}
	})
	wp.SetResultHandler(func(pr PollResult) { results <- pr })
	wp.Start(context.Background())
	wp.Enqueue(PollJob{Device: models.Device{ID: 7}, Priority: 1})
	<-started

	stopped := make(chan struct{})
	go func() { wp.Stop(); close(stopped) }()
	time.Sleep(50 * time.Millisecond)
	close(release)
	<-stopped

	select {
	case pr := <-results:
		if pr.Error != nil {
			t.Fatalf("in-flight poll saw a cancelled context: %v", pr.Error)
		}
	default:
		t.Fatal("result of the in-flight poll was not delivered")
	}
}
