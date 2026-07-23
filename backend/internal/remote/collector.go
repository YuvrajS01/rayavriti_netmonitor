package remote

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/websocket"
)

var pooledTransport = &http.Transport{
	MaxIdleConnsPerHost: 5,
	IdleConnTimeout:     90 * time.Second,
}

type Collector struct {
	store         *Store
	hub           *websocket.Hub
	timeout       time.Duration
	retentionDays int
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	maxConcurrent int
}

func NewCollector(store *Store, hub *websocket.Hub, timeout time.Duration, retentionDays int, maxConcurrent ...int) *Collector {
	cc := 10
	if len(maxConcurrent) > 0 && maxConcurrent[0] > 0 {
		cc = maxConcurrent[0]
	}
	return &Collector{store: store, hub: hub, timeout: timeout, retentionDays: retentionDays, maxConcurrent: cc}
}
func (c *Collector) Start(ctx context.Context) {
	if c == nil {
		return
	}
	ctx, c.cancel = context.WithCancel(ctx)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		c.poll(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.poll(ctx)
			}
		}
	}()
}
func (c *Collector) Stop() {
	if c != nil && c.cancel != nil {
		c.cancel()
		c.wg.Wait()
	}
}
func (c *Collector) poll(ctx context.Context) {
	instances, err := c.store.Due(ctx)
	if err != nil {
		return
	}

	sem := make(chan struct{}, c.maxConcurrent)
	var wg sync.WaitGroup
	for _, instance := range instances {
		if instance.LastSeenAt != nil && instance.LastSeenAt.Add(time.Duration(instance.PollIntervalS)*time.Second).After(time.Now()) {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(inst Instance) {
			defer wg.Done()
			defer func() { <-sem }()
			c.collect(ctx, inst)
		}(instance)
	}
	wg.Wait()

	if c.retentionDays > 0 {
		_ = c.store.PruneSnapshots(ctx, c.retentionDays)
	}
}
func (c *Collector) client(skip bool) *http.Client {
	transport := pooledTransport
	if skip {
		transport = pooledTransport.Clone()
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return &http.Client{Timeout: c.timeout, Transport: transport}
}
func (c *Collector) fetch(ctx context.Context, instance Instance, path string) (json.RawMessage, float64, error) {
	key, err := c.store.credential(ctx, instance.ID)
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, instance.URL+path, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("X-Api-Key", key)
	started := time.Now()
	resp, err := c.client(instance.TLSSkipVerify).Do(req)
	latency := float64(time.Since(started).Microseconds()) / 1000
	if err != nil {
		return nil, latency, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, latency, fmt.Errorf("remote returned %s", resp.Status)
	}
	var raw json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, latency, err
	}
	return raw, latency, nil
}
func unwrap(raw json.RawMessage) json.RawMessage {
	var body struct {
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal(raw, &body) == nil && len(body.Data) > 0 {
		return body.Data
	}
	return raw
}
func (c *Collector) collect(ctx context.Context, instance Instance) {
	health, latency, err := c.fetch(ctx, instance, "/health")
	if err != nil {
		_ = c.store.SetStatus(ctx, instance.ID, "offline", err.Error(), false)
		c.event(websocket.EventType("remote:status"), instance.ID, map[string]any{"status": "offline", "error": err.Error()})
		return
	}
	if identity, _, identityErr := c.fetch(ctx, instance, "/api/v1/sync/identity"); identityErr == nil {
		var payload struct {
			Fingerprint string `json:"fingerprint"`
		}
		_ = json.Unmarshal(unwrap(identity), &payload)
		if payload.Fingerprint != "" {
			_ = c.store.SetFingerprint(ctx, instance.ID, payload.Fingerprint)
		}
	}
	devices, _, _ := c.fetch(ctx, instance, "/api/v1/devices")
	alerts, _, _ := c.fetch(ctx, instance, "/api/v1/alerts?status=active&limit=200")
	snapshot := summarize(instance.ID, health, devices, alerts, latency)
	_ = c.store.SaveSnapshot(ctx, snapshot)
	_ = c.store.SetStatus(ctx, instance.ID, "online", "", true)
	c.event(websocket.EventType("remote:status"), instance.ID, map[string]any{"status": "online", "latencyMs": latency})
	c.event(websocket.EventType("remote:metrics"), instance.ID, snapshot)
}
func (c *Collector) event(event websocket.EventType, id int64, data any) {
	if c.hub != nil {
		c.hub.Broadcast(websocket.Message{Type: event, Data: map[string]any{"instanceId": id, "data": data}})
	}
}
func summarize(id int64, health, devices, alerts json.RawMessage, latency float64) Snapshot {
	snapshot := Snapshot{InstanceID: id, LatencyMS: latency, RawData: health}
	var h struct {
		Version string `json:"version"`
	}
	_ = json.Unmarshal(unwrap(health), &h)
	snapshot.Version = h.Version
	var ds []struct {
		Status string `json:"status"`
	}
	_ = json.Unmarshal(unwrap(devices), &ds)
	snapshot.DeviceCount = len(ds)
	for _, device := range ds {
		switch device.Status {
		case "up", "online":
			snapshot.DeviceUpCount++
		case "down", "offline":
			snapshot.DeviceDownCount++
		}
	}
	var al struct {
		Alerts []struct {
			Severity string `json:"severity"`
		} `json:"alerts"`
	}
	if json.Unmarshal(unwrap(alerts), &al) == nil {
		snapshot.AlertCount = len(al.Alerts)
		for _, alert := range al.Alerts {
			if alert.Severity == "critical" {
				snapshot.CriticalAlerts++
			}
		}
	}
	if snapshot.DeviceCount == 0 {
		snapshot.HealthScore = 100
	} else {
		snapshot.HealthScore = 100 - (float64(snapshot.DeviceDownCount) / float64(snapshot.DeviceCount) * 100)
	}
	return snapshot
}
func (c *Collector) Proxy(ctx context.Context, id int64, path string) (json.RawMessage, error) {
	instance, err := c.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	raw, _, err := c.fetch(ctx, *instance, path)
	return raw, err
}
