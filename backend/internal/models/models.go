package models

import (
	"encoding/json"
	"time"
)

type Device struct {
	ID                 int64     `json:"id"`
	Name               string    `json:"name"`
	IPAddress          string    `json:"ipAddress"`
	Protocol           string    `json:"protocol"`
	Enabled            bool      `json:"enabled"`
	Status             string    `json:"status"`
	Tags               []string  `json:"tags"`
	SNMPCommunity      string    `json:"snmpCommunity,omitempty"`
	SNMPVersion        string    `json:"snmpVersion,omitempty"`
	SNMPPort           int       `json:"snmpPort,omitempty"`
	HTTPPath           string    `json:"httpPath,omitempty"`
	HTTPExpectedStatus int       `json:"httpExpectedStatus,omitempty"`
	Interval           int       `json:"interval"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
	LocationID         *int64    `json:"locationId,omitempty"`
	ParentDeviceID     *int64    `json:"parentDeviceId,omitempty"`
	RackPosition       string    `json:"rackPosition,omitempty"`
	AssetTag           string    `json:"assetTag,omitempty"`
	MACAddress         string    `json:"macAddress,omitempty"`
	Manufacturer       string    `json:"manufacturer,omitempty"`
	Model              string    `json:"model,omitempty"`
	DeviceCategory     string    `json:"deviceCategory,omitempty"`
	Notes              string    `json:"notes,omitempty"`
}

type Metric struct {
	ID           int64          `json:"id"`
	DeviceID     int64          `json:"deviceId"`
	Timestamp    time.Time      `json:"timestamp"`
	Status       string         `json:"status"`
	ResponseTime *float64       `json:"responseTime,omitempty"`
	PacketLoss   *float64       `json:"packetLoss,omitempty"`
	CPUUsage     *float64       `json:"cpuUsage,omitempty"`
	MemoryUsage  *float64       `json:"memoryUsage,omitempty"`
	Bandwidth    *float64       `json:"bandwidth,omitempty"`
	CustomValue  *float64       `json:"customValue,omitempty"`
	Details      map[string]any `json:"details,omitempty"`
}

type Alert struct {
	ID             int64      `json:"id"`
	DeviceID       int64      `json:"deviceId"`
	DeviceName     string     `json:"deviceName"`
	Severity       string     `json:"severity"` // critical | warning | info
	Message        string     `json:"message"`
	Status         string     `json:"status"` // active | acknowledged | resolved
	CreatedAt      time.Time  `json:"createdAt"`
	AcknowledgedAt *time.Time `json:"acknowledgedAt,omitempty"`
	ResolvedAt     *time.Time `json:"resolvedAt,omitempty"`
	AcknowledgedBy *string    `json:"acknowledgedBy,omitempty"`
	ResolvedBy     *string    `json:"resolvedBy,omitempty"`
	RuleID         *int64     `json:"ruleId,omitempty"`
}

type User struct {
	ID           int64      `json:"id"`
	Username     string     `json:"username"`
	PasswordHash string     `json:"-"`
	Role         string     `json:"role"`
	DisplayName  string     `json:"displayName,omitempty"`
	Email        string     `json:"email,omitempty"`
	Phone        string     `json:"phone,omitempty"`
	Enabled      bool       `json:"enabled"`
	LastLoginAt  *time.Time `json:"lastLoginAt,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	RoleID       *int64     `json:"roleId,omitempty"`
}

type APIKey struct {
	ID          int64      `json:"id"`
	UserID      int64      `json:"userId"`
	KeyHash     string     `json:"-"`
	Description string     `json:"description"`
	CreatedAt   time.Time  `json:"createdAt"`
	LastUsedAt  *time.Time `json:"lastUsedAt,omitempty"`
}

type Flow struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	SrcIP     string    `json:"srcIp"`
	DstIP     string    `json:"dstIp"`
	SrcPort   int       `json:"srcPort"`
	DstPort   int       `json:"dstPort"`
	Protocol  string    `json:"protocol"`
	Bytes     int64     `json:"bytes"`
	Packets   int64     `json:"packets"`
	Duration  float64   `json:"duration"`
}

type IPCount struct {
	IP    string `json:"ip"`
	Count int64  `json:"count"`
}

type CaptureStats struct {
	TotalPackets int64             `json:"totalPackets"`
	TotalBytes   int64             `json:"totalBytes"`
	Protocols    map[string]int64  `json:"protocols"`
	TopSrcIPs    []IPCount         `json:"topSrcIps"`
	TopDstIPs    []IPCount         `json:"topDstIps"`
}

type Dashboard struct {
	ID        int64           `json:"id"`
	UserID    int64           `json:"userId"`
	Name      string          `json:"name"`
	Layout    json.RawMessage `json:"layout"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

type HealthStatus struct {
	Status     string            `json:"status"`
	Version    string            `json:"version"`
	Uptime     float64           `json:"uptime"`
	Database   string            `json:"database"`
	Collectors map[string]string `json:"collectors"`
}

type PagedResult[T any] struct {
	Data     []T `json:"data"`
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}

type Sensor struct {
	ID        int64          `json:"id"`
	DeviceID  int64          `json:"deviceId"`
	Name      string         `json:"name"`
	Type      string         `json:"type"` // ping, http, port, snmp, system
	Enabled   bool           `json:"enabled"`
	Interval  int            `json:"interval"` // seconds
	Config    map[string]any `json:"config,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

type AlertRule struct {
	ID          int64                `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Enabled     bool                 `json:"enabled"`
	Severity    string               `json:"severity"` // critical, warning, info
	DeviceID    *int64               `json:"deviceId,omitempty"` // nil = all devices
	CooldownSec int                  `json:"cooldownSec"`
	Conditions  []AlertRuleCondition `json:"conditions"`
	ChannelIDs  []int64              `json:"channelIds"`
	CreatedAt   time.Time            `json:"createdAt"`
	UpdatedAt   time.Time            `json:"updatedAt"`
}

type AlertRuleCondition struct {
	ID          int64   `json:"id"`
	RuleID      int64   `json:"ruleId"`
	Type        string  `json:"type"` // threshold, status_change, absence
	Field       string  `json:"field"` // status, response_time, packet_loss, cpu, memory
	Operator    string  `json:"operator"` // gt, lt, gte, lte, eq, neq
	Threshold   float64 `json:"threshold"`
	DurationSec int     `json:"durationSec"` // sustained duration before firing
}

type NotificationChannel struct {
	ID        int64          `json:"id"`
	Name      string         `json:"name"`
	Type      string         `json:"type"` // webhook, email, slack
	Enabled   bool           `json:"enabled"`
	Config    map[string]any `json:"config"`
	CreatedAt time.Time      `json:"createdAt"`
}
