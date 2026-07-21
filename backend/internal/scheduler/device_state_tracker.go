package scheduler

import (
	"sync"
	"time"
)

type DeviceHealth struct {
	DeviceID          int64
	CurrentStatus     string
	ConsecutiveFails  int
	LastSuccessAt     time.Time
	LastFailAt        time.Time
	LastResponseTime  time.Duration
	AvgResponseTime   time.Duration
	State             DeviceState
	BackoffMultiplier int
	responseHistory   []time.Duration
}

type DeviceStateTracker struct {
	states map[int64]*DeviceHealth
	mu     sync.RWMutex
}

func NewDeviceStateTracker() *DeviceStateTracker {
	return &DeviceStateTracker{
		states: make(map[int64]*DeviceHealth),
	}
}

func (dst *DeviceStateTracker) RecordSuccess(deviceID int64, responseTime time.Duration) {
	dst.mu.Lock()
	defer dst.mu.Unlock()

	h, exists := dst.states[deviceID]
	if !exists {
		h = &DeviceHealth{DeviceID: deviceID}
		dst.states[deviceID] = h
	}

	h.CurrentStatus = "up"
	h.ConsecutiveFails = 0
	h.LastSuccessAt = time.Now()
	h.LastResponseTime = responseTime
	h.State = StateHealthy
	h.BackoffMultiplier = 1

	h.responseHistory = append(h.responseHistory, responseTime)
	if len(h.responseHistory) > 10 {
		h.responseHistory = h.responseHistory[len(h.responseHistory)-10:]
	}
	var total time.Duration
	for _, t := range h.responseHistory {
		total += t
	}
	h.AvgResponseTime = total / time.Duration(len(h.responseHistory))
}

func (dst *DeviceStateTracker) RecordFailure(deviceID int64, err error) {
	dst.mu.Lock()
	defer dst.mu.Unlock()

	h, exists := dst.states[deviceID]
	if !exists {
		h = &DeviceHealth{DeviceID: deviceID}
		dst.states[deviceID] = h
	}

	h.CurrentStatus = "down"
	h.ConsecutiveFails++
	h.LastFailAt = time.Now()

	switch {
	case h.ConsecutiveFails >= 10:
		h.State = StateUnreachable
		h.BackoffMultiplier = 8
	case h.ConsecutiveFails >= 3:
		h.State = StateUnreachable
		h.BackoffMultiplier = 4
	case h.ConsecutiveFails >= 2:
		h.State = StateHealthy
		h.BackoffMultiplier = 2
	default:
		h.State = StateHealthy
		h.BackoffMultiplier = 1
	}
}

func (dst *DeviceStateTracker) GetState(deviceID int64) *DeviceHealth {
	dst.mu.RLock()
	defer dst.mu.RUnlock()
	return dst.states[deviceID]
}

func (dst *DeviceStateTracker) GetUnreachableDevices() []int64 {
	dst.mu.RLock()
	defer dst.mu.RUnlock()

	var ids []int64
	for _, h := range dst.states {
		if h.State == StateUnreachable {
			ids = append(ids, h.DeviceID)
		}
	}
	return ids
}

func (dst *DeviceStateTracker) GetHealthyCount() int {
	dst.mu.RLock()
	defer dst.mu.RUnlock()
	count := 0
	for _, h := range dst.states {
		if h.State == StateHealthy {
			count++
		}
	}
	return count
}

func (dst *DeviceStateTracker) GetUnreachableCount() int {
	dst.mu.RLock()
	defer dst.mu.RUnlock()
	count := 0
	for _, h := range dst.states {
		if h.State == StateUnreachable {
			count++
		}
	}
	return count
}

func (dst *DeviceStateTracker) GetPausedCount() int {
	dst.mu.RLock()
	defer dst.mu.RUnlock()
	count := 0
	for _, h := range dst.states {
		if h.State == StatePaused {
			count++
		}
	}
	return count
}

func (dst *DeviceStateTracker) Pause(deviceID int64) {
	dst.mu.Lock()
	defer dst.mu.Unlock()
	if h, exists := dst.states[deviceID]; exists {
		h.State = StatePaused
	}
}

func (dst *DeviceStateTracker) Resume(deviceID int64) {
	dst.mu.Lock()
	defer dst.mu.Unlock()
	if h, exists := dst.states[deviceID]; exists {
		h.State = StateHealthy
		h.ConsecutiveFails = 0
		h.BackoffMultiplier = 1
	}
}

func (dst *DeviceStateTracker) Remove(deviceID int64) {
	dst.mu.Lock()
	defer dst.mu.Unlock()
	delete(dst.states, deviceID)
}

func (dst *DeviceStateTracker) Count() int {
	dst.mu.RLock()
	defer dst.mu.RUnlock()
	return len(dst.states)
}