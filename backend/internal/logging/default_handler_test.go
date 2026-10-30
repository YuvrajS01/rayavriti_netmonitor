package logging

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
)

func TestPersistingHandler_PersistsWarnAndAbove(t *testing.T) {
	runtime := NewRuntimeControls(slog.LevelInfo)
	var mu sync.Mutex
	var got []PersistedEvent
	runtime.SetPersistFunc(func(_ context.Context, evt PersistedEvent) {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, evt)
	})
	root := &Logger{runtime: runtime}
	inner := slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: runtime})
	log := slog.New(newPersistingHandler(inner, root)).With("component", "scheduler")

	log.Info("poll ok", "device_id", 1)
	log.Warn("queue full", "device_id", 2)
	log.Debug("noise")

	if len(got) != 1 {
		t.Fatalf("persisted %d events, want 1 (only Warn)", len(got))
	}
	evt := got[0]
	if evt.Message != "queue full" || evt.Component != "scheduler" {
		t.Fatalf("unexpected event: %+v", evt)
	}
	if evt.DeviceID == nil || *evt.DeviceID != 2 {
		t.Fatalf("device_id not carried into persisted event: %+v", evt.DeviceID)
	}
}
