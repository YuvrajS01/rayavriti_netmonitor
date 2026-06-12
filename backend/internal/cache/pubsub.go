package cache

import (
	"context"
	"encoding/json"
	"log/slog"
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
	rdb        *Redis
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
}
