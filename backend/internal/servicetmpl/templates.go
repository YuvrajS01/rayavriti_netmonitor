package servicetmpl

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (int64, error)
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) Row
	Begin(ctx context.Context) (Tx, error)
}

type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Close()
}

type Row interface {
	Scan(dest ...any) error
}

type Tx interface {
	Exec(ctx context.Context, sql string, args ...any) (int64, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

type Service struct {
	pool *pgxpool.Pool
}

type CheckConfig struct {
	Type     string         `json:"type"`
	Name     string         `json:"name"`
	Enabled  bool           `json:"enabled"`
	Interval int            `json:"interval"`
	Config   map[string]any `json:"config"`
}

type AlertDef struct {
	Name        string   `json:"name"`
	Severity    string   `json:"severity"`
	MetricField string   `json:"metricField"`
	Operator    string   `json:"operator"`
	Value       string   `json:"value"`
	Duration    int      `json:"durationSeconds"`
	Channels    []string `json:"channels"`
}

type Template struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Category    string        `json:"category"`
	DeviceProto string        `json:"deviceProtocol"`
	DevicePort  int           `json:"devicePort"`
	CategoryTag string        `json:"deviceCategory"`
	Checks      []CheckConfig `json:"checks"`
	Alerts      []AlertDef    `json:"alerts"`
}

type ApplyRequest struct {
	Template   string `json:"template"`
	Host       string `json:"host"`
	LocationID *int64 `json:"locationId,omitempty"`
	ContactID  *int64 `json:"contactId,omitempty"`
	Name       string `json:"name,omitempty"`
}

type ApplyResult struct {
	DeviceID   int64   `json:"deviceId"`
	DeviceName string  `json:"deviceName"`
	SensorIDs  []int64 `json:"sensorIds"`
	RuleIDs    []int64 `json:"ruleIds"`
}

var templates = map[string]*Template{
	"college_erp": {
		Name:        "College ERP",
		Description: "Monitors ERP system availability, login page, response time, and SSL certificate",
		Category:    "Academic Services",
		DeviceProto: "http",
		DevicePort:  443,
		CategoryTag: "server",
		Checks: []CheckConfig{
			{Type: "http", Name: "HTTP Health Check", Enabled: true, Interval: 60, Config: map[string]any{"path": "/", "expected_status": 200}},
			{Type: "http", Name: "Login Page Check", Enabled: true, Interval: 60, Config: map[string]any{"path": "/login", "expected_status": 200, "keyword": "login"}},
			{Type: "port", Name: "HTTPS Port", Enabled: true, Interval: 30, Config: map[string]any{"port": 443}},
		},
		Alerts: []AlertDef{
			{Name: "ERP Down", Severity: "critical", MetricField: "status", Operator: "eq", Value: "down", Duration: 0},
			{Name: "ERP Slow Response", Severity: "warning", MetricField: "response_time", Operator: "gt", Value: "5000", Duration: 120},
			{Name: "ERP SSL Expiring", Severity: "warning", MetricField: "ssl_expiry_days", Operator: "lt", Value: "14", Duration: 0},
		},
	},
	"moodle_lms": {
		Name:        "Moodle LMS",
		Description: "Monitors Moodle learning management system availability and response time",
		Category:    "Academic Services",
		DeviceProto: "http",
		DevicePort:  443,
		CategoryTag: "server",
		Checks: []CheckConfig{
			{Type: "http", Name: "Moodle Health Check", Enabled: true, Interval: 60, Config: map[string]any{"path": "/login/index.php", "expected_status": 200}},
			{Type: "http", Name: "Moodle API Check", Enabled: true, Interval: 120, Config: map[string]any{"path": "/webservice/rest/server.php", "expected_status": 200}},
		},
		Alerts: []AlertDef{
			{Name: "Moodle Down", Severity: "critical", MetricField: "status", Operator: "eq", Value: "down", Duration: 0},
			{Name: "Moodle Slow Response", Severity: "warning", MetricField: "response_time", Operator: "gt", Value: "8000", Duration: 120},
		},
	},
	"email_server": {
		Name:        "Email Server",
		Description: "Monitors SMTP, IMAP, IMAPS, and POP3S ports on the email server",
		Category:    "Core Infrastructure",
		DeviceProto: "port",
		DevicePort:  25,
		CategoryTag: "server",
		Checks: []CheckConfig{
			{Type: "port", Name: "SMTP", Enabled: true, Interval: 30, Config: map[string]any{"port": 25}},
			{Type: "port", Name: "IMAP", Enabled: true, Interval: 30, Config: map[string]any{"port": 143}},
			{Type: "port", Name: "IMAPS", Enabled: true, Interval: 30, Config: map[string]any{"port": 993}},
			{Type: "port", Name: "POP3S", Enabled: true, Interval: 30, Config: map[string]any{"port": 995}},
		},
		Alerts: []AlertDef{
			{Name: "SMTP Port Down", Severity: "critical", MetricField: "status", Operator: "eq", Value: "down", Duration: 0},
			{Name: "IMAP Port Down", Severity: "critical", MetricField: "status", Operator: "eq", Value: "down", Duration: 0},
			{Name: "IMAPS Port Down", Severity: "critical", MetricField: "status", Operator: "eq", Value: "down", Duration: 0},
		},
	},
	"dns_server": {
		Name:        "DNS Server",
		Description: "Monitors DNS resolution and port 53 availability",
		Category:    "Core Infrastructure",
		DeviceProto: "port",
		DevicePort:  53,
		CategoryTag: "server",
		Checks: []CheckConfig{
			{Type: "port", Name: "DNS TCP", Enabled: true, Interval: 30, Config: map[string]any{"port": 53}},
			{Type: "port", Name: "DNS UDP", Enabled: true, Interval: 30, Config: map[string]any{"port": 53, "protocol": "udp"}},
		},
		Alerts: []AlertDef{
			{Name: "DNS Server Down", Severity: "critical", MetricField: "status", Operator: "eq", Value: "down", Duration: 0},
		},
	},
	"radius_ldap": {
		Name:        "RADIUS/LDAP Server",
		Description: "Monitors RADIUS and LDAP authentication services",
		Category:    "Core Infrastructure",
		DeviceProto: "port",
		DevicePort:  1812,
		CategoryTag: "server",
		Checks: []CheckConfig{
			{Type: "port", Name: "RADIUS Auth", Enabled: true, Interval: 30, Config: map[string]any{"port": 1812}},
			{Type: "port", Name: "RADIUS Accounting", Enabled: true, Interval: 30, Config: map[string]any{"port": 1813}},
			{Type: "port", Name: "LDAP", Enabled: true, Interval: 30, Config: map[string]any{"port": 389}},
			{Type: "port", Name: "LDAPS", Enabled: true, Interval: 30, Config: map[string]any{"port": 636}},
		},
		Alerts: []AlertDef{
			{Name: "RADIUS Down", Severity: "critical", MetricField: "status", Operator: "eq", Value: "down", Duration: 0},
			{Name: "LDAP Down", Severity: "critical", MetricField: "status", Operator: "eq", Value: "down", Duration: 0},
		},
	},
	"proxy_server": {
		Name:        "Proxy Server",
		Description: "Monitors HTTP proxy availability and port",
		Category:    "Core Infrastructure",
		DeviceProto: "http",
		DevicePort:  3128,
		CategoryTag: "server",
		Checks: []CheckConfig{
			{Type: "port", Name: "Proxy Port", Enabled: true, Interval: 30, Config: map[string]any{"port": 3128}},
			{Type: "http", Name: "Proxy Health", Enabled: true, Interval: 60, Config: map[string]any{"path": "/", "expected_status": 200}},
		},
		Alerts: []AlertDef{
			{Name: "Proxy Down", Severity: "critical", MetricField: "status", Operator: "eq", Value: "down", Duration: 0},
		},
	},
	"cctv_nvr": {
		Name:        "CCTV/NVR System",
		Description: "Monitors CCTV NVR management interface and RTSP stream",
		Category:    "Security",
		DeviceProto: "http",
		DevicePort:  80,
		CategoryTag: "cctv",
		Checks: []CheckConfig{
			{Type: "http", Name: "NVR Web Interface", Enabled: true, Interval: 60, Config: map[string]any{"path": "/", "expected_status": 200}},
			{Type: "port", Name: "RTSP Stream", Enabled: true, Interval: 30, Config: map[string]any{"port": 554}},
		},
		Alerts: []AlertDef{
			{Name: "NVR Down", Severity: "critical", MetricField: "status", Operator: "eq", Value: "down", Duration: 0},
			{Name: "RTSP Unreachable", Severity: "warning", MetricField: "status", Operator: "eq", Value: "down", Duration: 60},
		},
	},
	"biometric_server": {
		Name:        "Biometric Server",
		Description: "Monitors biometric/attendance system availability",
		Category:    "Security",
		DeviceProto: "http",
		DevicePort:  80,
		CategoryTag: "server",
		Checks: []CheckConfig{
			{Type: "port", Name: "Management Port", Enabled: true, Interval: 30, Config: map[string]any{"port": 80}},
			{Type: "http", Name: "Web Interface", Enabled: true, Interval: 60, Config: map[string]any{"path": "/", "expected_status": 200}},
		},
		Alerts: []AlertDef{
			{Name: "Biometric Server Down", Severity: "critical", MetricField: "status", Operator: "eq", Value: "down", Duration: 0},
		},
	},
	"ups_monitoring": {
		Name:        "UPS Monitoring",
		Description: "Monitors UPS battery status, load, and input voltage via SNMP",
		Category:    "Power",
		DeviceProto: "snmp",
		DevicePort:  161,
		CategoryTag: "ups",
		Checks: []CheckConfig{
			{Type: "snmp", Name: "SNMP Reachable", Enabled: true, Interval: 30, Config: map[string]any{"community": "public", "oid": "1.3.6.1.2.1.33.1.2.1"}},
		},
		Alerts: []AlertDef{
			{Name: "UPS SNMP Unreachable", Severity: "critical", MetricField: "status", Operator: "eq", Value: "down", Duration: 0},
			{Name: "UPS On Battery", Severity: "warning", MetricField: "ups_status", Operator: "eq", Value: "on_battery", Duration: 0},
			{Name: "UPS Battery Low", Severity: "critical", MetricField: "battery_capacity", Operator: "lt", Value: "20", Duration: 0},
		},
	},
	"printer_network": {
		Name:        "Network Printer",
		Description: "Monitors network printer via raw TCP, IPP, and SNMP",
		Category:    "Peripherals",
		DeviceProto: "port",
		DevicePort:  9100,
		CategoryTag: "printer",
		Checks: []CheckConfig{
			{Type: "port", Name: "Raw TCP (9100)", Enabled: true, Interval: 60, Config: map[string]any{"port": 9100}},
			{Type: "port", Name: "IPP (631)", Enabled: true, Interval: 60, Config: map[string]any{"port": 631}},
		},
		Alerts: []AlertDef{
			{Name: "Printer Offline", Severity: "warning", MetricField: "status", Operator: "eq", Value: "down", Duration: 120},
		},
	},
	"wifi_controller": {
		Name:        "WiFi Controller",
		Description: "Monitors wireless LAN controller management interface and AP status",
		Category:    "Network",
		DeviceProto: "http",
		DevicePort:  443,
		CategoryTag: "router",
		Checks: []CheckConfig{
			{Type: "http", Name: "WLC Web Interface", Enabled: true, Interval: 60, Config: map[string]any{"path": "/", "expected_status": 200}},
			{Type: "snmp", Name: "AP Count", Enabled: true, Interval: 120, Config: map[string]any{"community": "public", "oid": "1.3.6.1.4.1.14179.2.1.3.1.3"}},
		},
		Alerts: []AlertDef{
			{Name: "WLC Down", Severity: "critical", MetricField: "status", Operator: "eq", Value: "down", Duration: 0},
			{Name: "WLC High CPU", Severity: "warning", MetricField: "cpu", Operator: "gt", Value: "90", Duration: 300},
		},
	},
	"file_server": {
		Name:        "File Server",
		Description: "Monitors SMB, NFS, and HTTP file browsing services",
		Category:    "Core Infrastructure",
		DeviceProto: "port",
		DevicePort:  445,
		CategoryTag: "server",
		Checks: []CheckConfig{
			{Type: "port", Name: "SMB", Enabled: true, Interval: 30, Config: map[string]any{"port": 445}},
			{Type: "port", Name: "NFS", Enabled: true, Interval: 30, Config: map[string]any{"port": 2049}},
		},
		Alerts: []AlertDef{
			{Name: "SMB Down", Severity: "critical", MetricField: "status", Operator: "eq", Value: "down", Duration: 0},
			{Name: "NFS Down", Severity: "warning", MetricField: "status", Operator: "eq", Value: "down", Duration: 60},
		},
	},
	"database_server": {
		Name:        "Database Server",
		Description: "Monitors MySQL, PostgreSQL, and MongoDB port availability",
		Category:    "Core Infrastructure",
		DeviceProto: "port",
		DevicePort:  5432,
		CategoryTag: "server",
		Checks: []CheckConfig{
			{Type: "port", Name: "PostgreSQL", Enabled: true, Interval: 30, Config: map[string]any{"port": 5432}},
			{Type: "port", Name: "MySQL", Enabled: true, Interval: 30, Config: map[string]any{"port": 3306}},
			{Type: "port", Name: "MongoDB", Enabled: true, Interval: 30, Config: map[string]any{"port": 27017}},
		},
		Alerts: []AlertDef{
			{Name: "PostgreSQL Down", Severity: "critical", MetricField: "status", Operator: "eq", Value: "down", Duration: 0},
			{Name: "MySQL Down", Severity: "critical", MetricField: "status", Operator: "eq", Value: "down", Duration: 0},
		},
	},
}

func ListTemplates() []Template {
	result := make([]Template, 0, len(templates))
	for _, t := range templates {
		result = append(result, *t)
	}
	return result
}

func GetTemplate(name string) (*Template, error) {
	t, ok := templates[name]
	if !ok {
		return nil, fmt.Errorf("template %q not found", name)
	}
	return t, nil
}

func (s *Service) Apply(ctx context.Context, req ApplyRequest) (*ApplyResult, error) {
	tmpl, err := GetTemplate(req.Template)
	if err != nil {
		return nil, err
	}

	deviceName := req.Name
	if deviceName == "" {
		deviceName = fmt.Sprintf("%s (%s)", tmpl.Name, req.Host)
	}

	now := time.Now()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var deviceID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO devices (name, ip_address, protocol, port, enabled, tags, device_category, location_id, notes, created_at, updated_at)
		VALUES ($1, $2, $3, $4, true, $5, $6, $7, $8, $9, $10)
		RETURNING id`,
		deviceName, req.Host, tmpl.DeviceProto, tmpl.DevicePort, true,
		fmt.Sprintf(`["%s"]`, tmpl.CategoryTag), tmpl.CategoryTag, req.LocationID,
		fmt.Sprintf("Auto-created from template: %s", req.Template), now, now,
	).Scan(&deviceID)
	if err != nil {
		return nil, fmt.Errorf("create device: %w", err)
	}

	result := &ApplyResult{DeviceID: deviceID, DeviceName: deviceName}

	for _, chk := range tmpl.Checks {
		var sensorID int64
		err = tx.QueryRow(ctx, `
			INSERT INTO sensors (device_id, name, type, enabled, interval, config, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING id`,
			deviceID, chk.Name, chk.Type, chk.Enabled, chk.Interval,
			"{}", now, now,
		).Scan(&sensorID)
		if err != nil {
			return nil, fmt.Errorf("create sensor %q: %w", chk.Name, err)
		}
		result.SensorIDs = append(result.SensorIDs, sensorID)
	}

	for _, alert := range tmpl.Alerts {
		var ruleID int64
		err = tx.QueryRow(ctx, `
			INSERT INTO alert_rules (name, description, enabled, severity, scope_type, device_id, condition_logic, cooldown_seconds, auto_resolve, created_at, updated_at)
			VALUES ($1, $2, true, $3, 'device', $4, 'all', 300, true, $5, $6)
			RETURNING id`,
			alert.Name, fmt.Sprintf("Auto-generated from %s template", req.Template),
			alert.Severity, deviceID, now, now,
		).Scan(&ruleID)
		if err != nil {
			return nil, fmt.Errorf("create alert rule %q: %w", alert.Name, err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO alert_rule_conditions (rule_id, type, metric_field, operator, value, duration_seconds)
			VALUES ($1, 'threshold', $2, $3, $4, $5)`,
			ruleID, alert.MetricField, alert.Operator, alert.Value, alert.Duration,
		)
		if err != nil {
			return nil, fmt.Errorf("create alert condition for %q: %w", alert.Name, err)
		}

		result.RuleIDs = append(result.RuleIDs, ruleID)
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return result, nil
}
