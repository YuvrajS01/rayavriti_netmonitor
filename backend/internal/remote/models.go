package remote

import (
	"encoding/json"
	"time"
)

type Instance struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	URL           string     `json:"url"`
	LocationLabel string     `json:"locationLabel,omitempty"`
	Tags          []string   `json:"tags"`
	PollIntervalS int        `json:"pollIntervalS"`
	TLSSkipVerify bool       `json:"tlsSkipVerify"`
	Status        string     `json:"status"`
	LastSeenAt    *time.Time `json:"lastSeenAt,omitempty"`
	LastError     string     `json:"lastError,omitempty"`
	Fingerprint   string     `json:"fingerprint,omitempty"`
	ServiceMode   string     `json:"serviceMode,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

type CreateInstance struct {
	Name          string   `json:"name"`
	URL           string   `json:"url"`
	APIKey        string   `json:"apiKey"`
	LocationLabel string   `json:"locationLabel"`
	Tags          []string `json:"tags"`
	PollIntervalS int      `json:"pollIntervalS"`
	TLSSkipVerify bool     `json:"tlsSkipVerify"`
}

type Snapshot struct {
	ID              int64           `json:"id"`
	InstanceID      int64           `json:"instanceId"`
	Timestamp       time.Time       `json:"timestamp"`
	DeviceCount     int             `json:"deviceCount"`
	DeviceUpCount   int             `json:"deviceUpCount"`
	DeviceDownCount int             `json:"deviceDownCount"`
	AlertCount      int             `json:"alertCount"`
	CriticalAlerts  int             `json:"criticalAlerts"`
	HealthScore     float64         `json:"healthScore"`
	LatencyMS       float64         `json:"latencyMs"`
	Version         string          `json:"version,omitempty"`
	RawData         json.RawMessage `json:"rawData,omitempty"`
}

type Overview struct {
	TotalInstances int `json:"totalInstances"`
	Online         int `json:"online"`
	Offline        int `json:"offline"`
	Degraded       int `json:"degraded"`
	AlertCount     int `json:"alertCount"`
}
