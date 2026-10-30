package websocket

import "testing"

func TestHub_BroadcastAfterStopDoesNotPanic(t *testing.T) {
	h := NewHub("secret", nil, nil)
	go h.Run()
	h.Stop()
	h.Stop() // idempotent
	for i := 0; i < 1000; i++ {
		h.Broadcast(Message{Type: EventMetricUpdate})
		h.BroadcastLocal(Message{Type: EventMetricUpdate})
	}
}
