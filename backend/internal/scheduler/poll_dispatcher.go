package scheduler

import (
	"container/heap"
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

type DeviceState int

const (
	StateHealthy     DeviceState = iota
	StateUnreachable
	StatePaused
)

type ScheduleEntry struct {
	DeviceID   int64
	Device     models.Device
	Priority   int
	Interval   time.Duration
	NextPollAt time.Time
	State      DeviceState
	Failures   int
	index      int
}

type ScheduleHeap []*ScheduleEntry

func (h ScheduleHeap) Len() int           { return len(h) }
func (h ScheduleHeap) Less(i, j int) bool  { return h[i].NextPollAt.Before(h[j].NextPollAt) }
func (h ScheduleHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i]; h[i].index = i; h[j].index = j }
func (h *ScheduleHeap) Push(x any)         { *h = append(*h, x.(*ScheduleEntry)); x.(*ScheduleEntry).index = len(*h) - 1 }
func (h *ScheduleHeap) Pop() any           { old := *h; n := len(old); item := old[n-1]; old[n-1] = nil; item.index = -1; *h = old[:n-1]; return item }

type PollDispatcher struct {
	pool      *WorkerPool
	schedule  *ScheduleHeap
	deviceMap map[int64]*ScheduleEntry
	mu        sync.RWMutex
	interval  time.Duration
	wg        sync.WaitGroup
	cancel    context.CancelFunc
}

func NewPollDispatcher(pool *WorkerPool, defaultInterval time.Duration) *PollDispatcher {
	h := &ScheduleHeap{}
	heap.Init(h)
	return &PollDispatcher{
		pool:      pool,
		schedule:  h,
		deviceMap: make(map[int64]*ScheduleEntry),
		interval:  defaultInterval,
	}
}

func (d *PollDispatcher) Start(ctx context.Context) {
	ctx, d.cancel = context.WithCancel(ctx)
	d.wg.Add(1)
	go d.run(ctx)
	slog.Info("poll dispatcher started")
}

func (d *PollDispatcher) Stop() {
	if d.cancel != nil {
		d.cancel()
	}
	d.wg.Wait()
	slog.Info("poll dispatcher stopped")
}

func (d *PollDispatcher) Upsert(device models.Device, priority int, interval time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if interval <= 0 {
		interval = d.interval
	}
	minInterval := 5 * time.Second
	if interval < minInterval {
		interval = minInterval
	}

	entry, exists := d.deviceMap[device.ID]
	if exists {
		entry.Device = device
		entry.Priority = priority
		entry.Interval = interval
		if entry.State != StatePaused {
			entry.NextPollAt = time.Now().Add(entry.effectiveInterval())
		}
		heap.Fix(d.schedule, entry.index)
	} else {
		entry = &ScheduleEntry{
			DeviceID:   device.ID,
			Device:     device,
			Priority:   priority,
			Interval:   interval,
			NextPollAt: time.Now(),
			State:      StateHealthy,
		}
		d.deviceMap[device.ID] = entry
		heap.Push(d.schedule, entry)
	}
}

func (d *PollDispatcher) Remove(deviceID int64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	entry, exists := d.deviceMap[deviceID]
	if !exists {
		return
	}
	heap.Remove(d.schedule, entry.index)
	delete(d.deviceMap, deviceID)
}

func (d *PollDispatcher) Pause(deviceID int64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	entry, exists := d.deviceMap[deviceID]
	if !exists {
		return
	}
	entry.State = StatePaused
}

func (d *PollDispatcher) Resume(deviceID int64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	entry, exists := d.deviceMap[deviceID]
	if !exists {
		return
	}
	entry.State = StateHealthy
	entry.Failures = 0
	entry.NextPollAt = time.Now()
	heap.Fix(d.schedule, entry.index)
}

func (d *PollDispatcher) RecordSuccess(deviceID int64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	entry, exists := d.deviceMap[deviceID]
	if !exists {
		return
	}
	if entry.State == StateUnreachable {
		entry.State = StateHealthy
		slog.Info("device recovered from unreachable state", "deviceID", deviceID)
	}
	entry.Failures = 0
	entry.NextPollAt = time.Now().Add(entry.Interval)
	heap.Fix(d.schedule, entry.index)
}

func (d *PollDispatcher) RecordFailure(deviceID int64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	entry, exists := d.deviceMap[deviceID]
	if !exists {
		return
	}
	entry.Failures++
	if entry.Failures >= 3 {
		entry.State = StateUnreachable
		slog.Warn("device marked unreachable", "deviceID", deviceID, "failures", entry.Failures)
	}
	entry.NextPollAt = time.Now().Add(entry.effectiveInterval())
	heap.Fix(d.schedule, entry.index)
}

func (d *PollDispatcher) Count() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.schedule.Len()
}

func (d *PollDispatcher) UnreachableCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	count := 0
	for _, entry := range d.deviceMap {
		if entry.State == StateUnreachable {
			count++
		}
	}
	return count
}

func (d *PollDispatcher) PausedCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	count := 0
	for _, entry := range d.deviceMap {
		if entry.State == StatePaused {
			count++
		}
	}
	return count
}

func (d *PollDispatcher) run(ctx context.Context) {
	defer d.wg.Done()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			d.dispatchDue()
			d.rescheduleTimer(timer)
		}
	}
}

func (d *PollDispatcher) dispatchDue() {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	for d.schedule.Len() > 0 {
		entry := (*d.schedule)[0]
		if entry.NextPollAt.After(now) {
			break
		}
		heap.Pop(d.schedule)

		if entry.State == StatePaused {
			heap.Push(d.schedule, entry)
			continue
		}

		priority := entry.Priority
		if entry.State == StateUnreachable {
			priority = 2
		}

		d.pool.Enqueue(PollJob{
			Device:     entry.Device,
			Priority:   priority,
			ScheduleAt: entry.NextPollAt,
			Attempt:    entry.Failures,
		})

		entry.NextPollAt = now.Add(entry.effectiveInterval())
		heap.Push(d.schedule, entry)
	}
}

func (d *PollDispatcher) rescheduleTimer(timer *time.Timer) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.schedule.Len() == 0 {
		timer.Reset(time.Second)
		return
	}

	now := time.Now()
	next := (*d.schedule)[0].NextPollAt
	wait := next.Sub(now)
	if wait < 0 {
		wait = 0
	}
	timer.Reset(wait)
}

func (e *ScheduleEntry) effectiveInterval() time.Duration {
	switch {
	case e.Failures >= 10:
		return e.Interval * 8
	case e.Failures >= 3:
		return e.Interval * 4
	case e.Failures >= 2:
		return e.Interval * 2
	default:
		return e.Interval
	}
}