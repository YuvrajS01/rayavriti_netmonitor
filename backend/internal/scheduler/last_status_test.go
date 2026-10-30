package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

func TestPollDispatcher_StampsLastStatusOnJobs(t *testing.T) {
	seen := make(chan string, 1)
	wp := NewWorkerPool(WorkerPoolConfig{WorkerCount: 1}, func(ctx context.Context, job PollJob) PollResult {
		seen <- job.Device.Status
		return PollResult{Device: job.Device, FinishedAt: time.Now()}
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wp.Start(ctx)
	defer wp.Stop()

	d := NewPollDispatcher(wp, time.Second)
	// The DB snapshot says "up", but the last poll observed "down".
	d.Upsert(models.Device{ID: 1, Status: "up"}, 1, time.Hour)
	d.RecordStatus(1, "down")
	d.dispatchDue()

	select {
	case status := <-seen:
		if status != "down" {
			t.Fatalf("job carried status %q, want the last polled status %q", status, "down")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("job was not dispatched")
	}
}
