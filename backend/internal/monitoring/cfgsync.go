package monitoring

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type ConfigSyncService struct {
	endpoint     string
	store        SysConfigStore
	interval     time.Duration
	graceDays    int
	httpClient   *http.Client
	onModeChange func(string)
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}
type SyncOption func(*ConfigSyncService)

func WithSyncInterval(value time.Duration) SyncOption {
	return func(s *ConfigSyncService) {
		if value > 0 {
			s.interval = value
		}
	}
}
func WithGracePeriod(days int) SyncOption {
	return func(s *ConfigSyncService) {
		if days > 0 {
			s.graceDays = days
		}
	}
}
func WithOnModeChange(callback func(string)) SyncOption {
	return func(s *ConfigSyncService) { s.onModeChange = callback }
}
func NewConfigSyncService(endpoint string, store SysConfigStore, opts ...SyncOption) *ConfigSyncService {
	service := &ConfigSyncService{endpoint: endpoint, store: store, interval: 6 * time.Hour, graceDays: 7, httpClient: &http.Client{Timeout: 15 * time.Second}}
	for _, opt := range opts {
		opt(service)
	}
	return service
}
func (s *ConfigSyncService) Start(ctx context.Context) {
	if s.endpoint == "" || s.store == nil {
		return
	}
	ctx, s.cancel = context.WithCancel(ctx)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.Sync(ctx)
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.Sync(ctx)
			}
		}
	}()
}
func (s *ConfigSyncService) Stop() {
	if s.cancel != nil {
		s.cancel()
		s.wg.Wait()
	}
}
func (s *ConfigSyncService) Sync(ctx context.Context) {
	fingerprint, err := s.fingerprint(ctx)
	if err != nil {
		return
	}
	body, _ := json.Marshal(map[string]any{"fingerprint": fingerprint, "timestamp": time.Now().UTC().Format(time.RFC3339)})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := s.httpClient.Do(request)
	if err != nil {
		s.checkGrace(ctx)
		return
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		s.checkGrace(ctx)
		return
	}
	var result struct {
		ServiceMode string `json:"service_mode"`
	}
	if json.NewDecoder(response.Body).Decode(&result) != nil {
		return
	}
	if result.ServiceMode != "active" && result.ServiceMode != "maintenance" && result.ServiceMode != "readonly" {
		result.ServiceMode = "active"
	}
	_ = s.store.SetSysConfig(ctx, "last_sync_ok", time.Now().UTC().Format(time.RFC3339))
	s.setMode(ctx, result.ServiceMode)
}
func (s *ConfigSyncService) fingerprint(ctx context.Context) (string, error) {
	value, err := s.store.GetSysConfig(ctx, "sys_fingerprint")
	if err != nil || value != "" {
		return value, err
	}
	data := make([]byte, 32)
	if _, err = rand.Read(data); err != nil {
		return "", err
	}
	value = hex.EncodeToString(data)
	return value, s.store.SetSysConfig(ctx, "sys_fingerprint", value)
}
func (s *ConfigSyncService) checkGrace(ctx context.Context) {
	value, err := s.store.GetSysConfig(ctx, "last_sync_ok")
	if err != nil || value == "" {
		return
	}
	last, err := time.Parse(time.RFC3339, value)
	if err == nil && time.Since(last) > time.Duration(s.graceDays)*24*time.Hour {
		s.setMode(ctx, "maintenance")
	}
}
func (s *ConfigSyncService) setMode(ctx context.Context, mode string) {
	_ = s.store.SetSysConfig(ctx, "service_mode", mode)
	if s.onModeChange != nil {
		s.onModeChange(mode)
	}
}
