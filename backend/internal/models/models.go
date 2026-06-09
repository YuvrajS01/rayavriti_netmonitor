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
	Port               int       `json:"port"`
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
	SensorID     *int64         `json:"sensorId,omitempty"`
	Timestamp    time.Time      `json:"timestamp"`
	CreatedAt    time.Time      `json:"createdAt"`
	Status       string         `json:"status"`
	ResponseTime *float64       `json:"responseTime,omitempty"`
	PacketLoss   *float64       `json:"packetLoss,omitempty"`
	CPUUsage     *float64       `json:"cpuUsage,omitempty"`
	MemoryUsage  *float64       `json:"memoryUsage,omitempty"`
	Bandwidth    *float64       `json:"bandwidth,omitempty"`
	CustomValue  *float64       `json:"customValue,omitempty"`
	Value        *float64       `json:"value,omitempty"`
	Message      string         `json:"message,omitempty"`
	Protocol     string         `json:"protocol,omitempty"`
	DeviceName   string         `json:"deviceName,omitempty"`
	Details      map[string]any `json:"details,omitempty"`
}

type MetricQuery struct {
	DeviceID    *int64    `json:"deviceId,omitempty"`
	From        time.Time `json:"from"`
	To          time.Time `json:"to"`
	Status      string    `json:"status,omitempty"`
	Limit       int       `json:"limit,omitempty"`
	Aggregation string    `json:"aggregation,omitempty"` // avg, max, min, p95
	BucketMin   int       `json:"bucketMin,omitempty"`   // bucket size in minutes
}

type ReportMetricRow struct {
	DeviceID     int64    `json:"deviceId"`
	DeviceName   string   `json:"deviceName"`
	Protocol     string   `json:"protocol"`
	Status       string   `json:"status"`
	ResponseTime *float64 `json:"responseTime,omitempty"`
	Value        *float64 `json:"value,omitempty"`
	Message      string   `json:"message,omitempty"`
	Timestamp    string   `json:"timestamp"`
}

type ReportTimeseriesPoint struct {
	BucketTime  string  `json:"bucketTime"`
	SampleCount int     `json:"sampleCount"`
	AvgResponse float64 `json:"avgResponse"`
	DownCount   int     `json:"downCount"`
	WarnCount   int     `json:"warnCount"`
}

type DeviceBreakdown struct {
	DeviceID    int64   `json:"deviceId"`
	DeviceName  string  `json:"deviceName"`
	Protocol    string  `json:"protocol"`
	SampleCount int     `json:"sampleCount"`
	DownCount   int     `json:"downCount"`
	WarnCount   int     `json:"warnCount"`
	AvgResponse float64 `json:"avgResponse"`
	MinResponse float64 `json:"minResponse"`
	MaxResponse float64 `json:"maxResponse"`
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

type AlertCounts struct {
	Active       int `json:"active"`
	Acknowledged int `json:"acknowledged"`
	Resolved     int `json:"resolved"`
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

type FlowTimeseriesPoint struct {
	BucketTime   string `json:"bucketTime"`
	TotalBytes   int64  `json:"totalBytes"`
	TotalPackets int64  `json:"totalPackets"`
	FlowCount    int64  `json:"flowCount"`
}

type FlowSummaryStats struct {
	TotalFlows         int64 `json:"totalFlows"`
	TotalBytes         int64 `json:"totalBytes"`
	TotalPackets       int64 `json:"totalPackets"`
	UniqueSources      int64 `json:"uniqueSources"`
	UniqueDestinations int64 `json:"uniqueDestinations"`
}

type IPCount struct {
	IP    string `json:"ip"`
	Count int64  `json:"count"`
}

type CaptureSession struct {
	ID            int64      `json:"id"`
	InterfaceName string     `json:"interfaceName"`
	Filter        string     `json:"filter"`
	Status        string     `json:"status"` // running, stopped, error
	StartedBy     string     `json:"startedBy,omitempty"`
	TotalPackets  int64      `json:"totalPackets"`
	TotalBytes    int64      `json:"totalBytes"`
	Protocols     map[string]int64 `json:"protocols,omitempty"`
	StartedAt     time.Time  `json:"startedAt"`
	StoppedAt     *time.Time `json:"stoppedAt,omitempty"`
	ErrorMessage  string     `json:"errorMessage,omitempty"`
}

type CaptureSessionStats struct {
	TotalPackets int64 `json:"totalPackets"`
	TotalBytes   int64 `json:"totalBytes"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

type CaptureStats struct {
	TotalPackets int64            `json:"totalPackets"`
	TotalBytes   int64            `json:"totalBytes"`
	Protocols    map[string]int64 `json:"protocols"`
	TopSrcIPs    []IPCount        `json:"topSrcIps"`
	TopDstIPs    []IPCount        `json:"topDstIps"`
}

type CapturePacket struct {
	ID            int64   `json:"id"`
	SessionID     int64   `json:"sessionId"`
	Timestamp     float64 `json:"timestamp"`
	SrcIP         string  `json:"srcIp"`
	DstIP         string  `json:"dstIp"`
	SrcPort       int     `json:"srcPort"`
	DstPort       int     `json:"dstPort"`
	Protocol      string  `json:"protocol"`
	Length        int     `json:"length"`
	Flags         string  `json:"flags,omitempty"`
	Payload       string  `json:"payload,omitempty"`
}

type PortScanResult struct {
	ID            int64      `json:"id"`
	DeviceID      int64      `json:"deviceId"`
	Port          int        `json:"port"`
	Protocol      string     `json:"protocol"`
	State         string     `json:"state"` // open, closed, filtered
	Service       string     `json:"service,omitempty"`
	ResponseTime  *float64   `json:"responseTime,omitempty"`
	FirstSeen     time.Time  `json:"firstSeen"`
	LastSeen      time.Time  `json:"lastSeen"`
	LastChangedAt time.Time  `json:"lastChangedAt"`
	ScannedAt     time.Time  `json:"scannedAt"`
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
	ID             int64                `json:"id"`
	Name           string               `json:"name"`
	Description    string               `json:"description"`
	Enabled        bool                 `json:"enabled"`
	Severity       string               `json:"severity"` // critical, warning, info
	ScopeType      string               `json:"scopeType"` // global, device
	ScopeValue     string               `json:"scopeValue,omitempty"`
	DeviceID       *int64               `json:"deviceId,omitempty"` // nil = all devices
	ConditionLogic string               `json:"conditionLogic"` // all, any
	CooldownSec    int                  `json:"cooldownSec"`
	AutoResolve    bool                 `json:"autoResolve"`
	CreatedBy      *int64               `json:"createdBy,omitempty"`
	Conditions     []AlertRuleCondition `json:"conditions"`
	ChannelIDs     []int64              `json:"channelIds"`
	CreatedAt      time.Time            `json:"createdAt"`
	UpdatedAt      time.Time            `json:"updatedAt"`
}

type AlertRuleCondition struct {
	ID              int64          `json:"id"`
	RuleID          int64          `json:"ruleId"`
	Type            string         `json:"type"` // threshold, status_change, absence, anomaly
	MetricField     string         `json:"metricField"` // status, response_time, packet_loss, cpu, memory
	Operator        string         `json:"operator"` // gt, lt, gte, lte, eq, neq
	Value           string         `json:"value"`
	DurationSeconds int            `json:"durationSeconds"` // sustained duration before firing
	Config          map[string]any `json:"config,omitempty"`
}

type NotificationChannel struct {
	ID        int64          `json:"id"`
	Name      string         `json:"name"`
	Type      string         `json:"type"` // webhook, email, slack
	Enabled   bool           `json:"enabled"`
	Config    map[string]any `json:"config"`
	CreatedAt time.Time      `json:"createdAt"`
}

type AlertHistory struct {
	ID        int64          `json:"id"`
	AlertID   int64          `json:"alertId"`
	RuleID    *int64         `json:"ruleId,omitempty"`
	Action    string         `json:"action"` // fired, notified, acknowledged, resolved, auto_resolved
	Actor     string         `json:"actor,omitempty"` // system, user:admin, rule:5
	Details   map[string]any `json:"details,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
}

type AlertRuleState struct {
	RuleID            int64          `json:"ruleId"`
	DeviceID          int64          `json:"deviceId"`
	State             string         `json:"state"` // idle, pending, firing, notified, acknowledged, resolved
	FirstMetAt        *time.Time     `json:"firstMetAt,omitempty"`
	LastEvaluatedAt   *time.Time     `json:"lastEvaluatedAt,omitempty"`
	LastFiredAt       *time.Time     `json:"lastFiredAt,omitempty"`
	LastResolvedAt    *time.Time     `json:"lastResolvedAt,omitempty"`
	ActiveAlertID     *int64         `json:"activeAlertId,omitempty"`
	ConditionSnapshot map[string]any `json:"conditionSnapshot,omitempty"`
}
