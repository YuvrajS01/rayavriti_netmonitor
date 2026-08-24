package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"
)

const wsBroadcastChannel = "nm:ws:broadcast"

// WSMessage represents a WebSocket message for the Pub/Sub bridge.
type WSMessage struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id,omitempty"`
	Data      any    `json:"data"`
}

// BroadcastFunc is called when a message is received from another instance.
type BroadcastFunc func(msg WSMessage)

type PubSubBridge struct {
	rdb         *Redis
	broadcastFn BroadcastFunc
}

func NewPubSubBridge(rdb *Redis, broadcastFn BroadcastFunc) *PubSubBridge {
	return &PubSubBridge{rdb: rdb, broadcastFn: broadcastFn}
}

func (p *PubSubBridge) Publish(ctx context.Context, msg WSMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return p.rdb.Client().Publish(ctx, wsBroadcastChannel, data).Err()
}

func (p *PubSubBridge) Subscribe(ctx context.Context) {
	// Reconnect loop with exponential backoff so a transient Redis
	// disconnect or a panic in the subscriber doesn't permanently kill
	// cross-instance WS broadcast (N5).
	const maxBackoff = 30 * time.Second
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		err := p.subscribeOnce(ctx)
		if err != nil {
			slog.Warn("Redis Pub/Sub subscriber disconnected, reconnecting", "error", err, "backoff", backoff)
		} else {
			slog.Info("Redis Pub/Sub subscriber stopped, reconnecting")
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// subscribeOnce subscribes to the WS broadcast channel and processes
// messages until the channel closes or a panic occurs. Returns the
// error (if any) so the caller can decide to reconnect.
func (p *PubSubBridge) subscribeOnce(ctx context.Context) (err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic recovered in pub/sub subscriber", "panic", r, "stack", string(debug.Stack()))
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	sub := p.rdb.Client().Subscribe(ctx, wsBroadcastChannel)
	ch := sub.Channel()
	slog.Info("Redis Pub/Sub subscriber started", "channel", wsBroadcastChannel)
	for msg := range ch {
		var wsMsg WSMessage
		if err := json.Unmarshal([]byte(msg.Payload), &wsMsg); err != nil {
			slog.Warn("Failed to unmarshal Pub/Sub message", "error", err)
			continue
		}
		p.broadcastFn(wsMsg)
	}
	slog.Info("Redis Pub/Sub subscriber stopped")
	return nil
}
