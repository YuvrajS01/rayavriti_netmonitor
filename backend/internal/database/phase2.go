package database

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

type phase2Resource struct {
	Table   string
	Select  string
	Cols    map[string]bool
	OrderBy string
}

var phase2Resources = map[string]phase2Resource{
	"locations":                    {Table: "locations", OrderBy: "sort_order ASC, id ASC", Cols: colset("name", "type", "parent_id", "code", "description", "address", "latitude", "longitude", "floor_number", "contact_person_id", "metadata", "sort_order", "enabled")},
	"subnets":                      {Table: "subnets", OrderBy: "id ASC", Cols: colset("name", "vlan_id", "cidr", "gateway", "description", "location_id", "dns_servers", "dhcp_enabled")},
	"suppressed_alerts":            {Table: "suppressed_alerts", OrderBy: "created_at DESC", Cols: colset("device_id", "rule_id", "suppression_reason", "root_cause_device_id", "root_cause_alert_id", "would_have_fired_at", "released_at")},
	"discovery_jobs":               {Table: "discovery_jobs", OrderBy: "started_at DESC", Cols: colset("subnet", "scan_type", "status", "location_id", "initiated_by", "total_ips_scanned", "devices_found", "devices_new", "devices_known", "completed_at", "error_message")},
	"discovery_results":            {Table: "discovery_results", OrderBy: "discovered_at DESC", Cols: colset("job_id", "ip_address", "mac_address", "manufacturer", "hostname", "device_description", "guessed_category", "guessed_os", "open_ports", "snmp_reachable", "response_time_ms", "status", "approved_device_id")},
	"status_page_services":         {Table: "status_page_services", OrderBy: "display_order ASC, id ASC", Cols: colset("name", "description", "group_name", "aggregation", "display_order", "show_response_time", "show_uptime", "enabled")},
	"status_page_incidents":        {Table: "status_page_incidents", OrderBy: "started_at DESC", Cols: colset("title", "message", "severity", "status", "started_at", "resolved_at", "created_by")},
	"status_page_incident_updates": {Table: "status_page_incident_updates", OrderBy: "created_at ASC", Cols: colset("incident_id", "status", "message", "created_by")},
	"maintenance_windows":          {Table: "maintenance_windows", OrderBy: "created_at DESC", Cols: colset("name", "description", "scope_type", "scope_value", "schedule_type", "start_time", "end_time", "recurrence_rule", "recurrence_start_time", "recurrence_end_time", "recurrence_timezone", "suppress_alerts", "suppress_notifications", "show_maintenance_status", "created_by", "enabled")},
	"contacts":                     {Table: "contacts", OrderBy: "name ASC", Cols: colset("name", "designation", "department", "email", "phone", "telegram_chat_id", "whatsapp_number", "preferred_channel", "notification_enabled", "quiet_hours_start", "quiet_hours_end", "user_id", "enabled")},
	"device_contacts":              {Table: "device_contacts", OrderBy: "id ASC", Cols: colset("device_id", "location_id", "contact_id", "role", "notify_on")},
	"escalation_policies":          {Table: "escalation_policies", OrderBy: "id ASC", Cols: colset("name", "description", "scope_type", "scope_value", "enabled")},
	"escalation_steps":             {Table: "escalation_steps", OrderBy: "policy_id ASC, step_order ASC", Cols: colset("policy_id", "step_order", "contact_id", "delay_minutes", "notify_via", "repeat_count", "repeat_interval_minutes")},
	"oncall_schedules":             {Table: "oncall_schedules", OrderBy: "id ASC", Cols: colset("name", "policy_id", "rotation_type", "participants", "current_index", "rotation_time", "timezone", "enabled")},
	"incidents":                    {Table: "incidents", OrderBy: "started_at DESC", Cols: colset("title", "description", "severity", "status", "root_cause", "root_cause_category", "resolution", "source", "source_alert_id", "assigned_to", "location_id", "impact_description", "affected_device_count", "started_at", "acknowledged_at", "resolved_at", "closed_at", "duration_seconds", "sla_breached", "created_by")},
	"incident_timeline":            {Table: "incident_timeline", OrderBy: "created_at ASC", Cols: colset("incident_id", "entry_type", "old_value", "new_value", "message", "author")},
	"sla_definitions":              {Table: "sla_definitions", OrderBy: "id ASC", Cols: colset("name", "severity", "response_time_minutes", "resolution_time_minutes", "enabled")},
	"roles":                        {Table: "roles", OrderBy: "id ASC", Cols: colset("name", "display_name", "description", "permissions", "is_system")},
	"users":                        {Table: "users", Select: "id,username,role,display_name,email,phone,enabled,last_login_at,created_at,role_id,contact_id", OrderBy: "id ASC", Cols: colset("role", "display_name", "email", "phone", "enabled", "role_id", "contact_id")},
	"user_scopes":                  {Table: "user_scopes", OrderBy: "id ASC", Cols: colset("user_id", "scope_type", "scope_value")},
	"notification_log":             {Table: "notification_log", OrderBy: "created_at DESC", Cols: colset("alert_id", "incident_id", "contact_id", "channel_type", "recipient", "message_preview", "status", "external_id", "error_message", "attempt_count", "escalation_step", "sent_at", "delivered_at", "read_at")},
	"scheduled_reports":            {Table: "scheduled_reports", OrderBy: "created_at DESC", Cols: colset("name", "report_type", "format", "schedule_cron", "timezone", "scope_type", "scope_value", "recipients", "include_charts", "lookback_period", "custom_from", "custom_to", "last_run_at", "last_run_status", "enabled", "created_by")},
	"generated_reports":            {Table: "generated_reports", OrderBy: "generated_at DESC", Cols: colset("scheduled_report_id", "report_type", "title", "format", "file_path", "file_size_bytes", "scope_description", "period_from", "period_to", "recipients", "generated_by")},
	"isp_links":                    {Table: "isp_links", OrderBy: "id ASC", Cols: colset("name", "provider", "circuit_id", "bandwidth_mbps", "gateway_ip", "sla_uptime_percent", "cost_monthly", "contract_start", "contract_end", "monitoring_interval_seconds", "enabled")},
	"isp_metrics":                  {Table: "isp_metrics", OrderBy: "created_at DESC", Cols: colset("link_id", "latency_ms", "jitter_ms", "packet_loss_percent", "download_speed_mbps", "upload_speed_mbps", "status", "target_ip")},
}

func colset(cols ...string) map[string]bool {
	out := make(map[string]bool, len(cols))
	for _, col := range cols {
		out[col] = true
	}
	return out
}

func getPhase2Resource(resource string) (phase2Resource, error) {
	res, ok := phase2Resources[resource]
	if !ok {
		return phase2Resource{}, fmt.Errorf("unknown phase2 resource %q", resource)
	}
	return res, nil
}

func (p *Postgres) ListPhase2(ctx context.Context, resource string, filters map[string]string) ([]map[string]any, error) {
	res, err := getPhase2Resource(resource)
	if err != nil {
		return nil, err
	}
	args := []any{}
	where := []string{}
	i := 1
	for key, val := range filters {
		if !res.Cols[key] || val == "" {
			continue
		}
		where = append(where, fmt.Sprintf("%s=$%d", key, i))
		args = append(args, val)
		i++
	}
	selectCols := res.Select
	if selectCols == "" {
		selectCols = "*"
	}
	query := "SELECT " + selectCols + " FROM " + res.Table
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY " + res.OrderBy + " LIMIT 500"
	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return rowsToMaps(rows)
}

func (p *Postgres) GetPhase2(ctx context.Context, resource string, id int64) (map[string]any, error) {
	res, err := getPhase2Resource(resource)
	if err != nil {
		return nil, err
	}
	selectCols := res.Select
	if selectCols == "" {
		selectCols = "*"
	}
	rows, err := p.pool.Query(ctx, "SELECT "+selectCols+" FROM "+res.Table+" WHERE id=$1", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items, err := rowsToMaps(rows)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, pgx.ErrNoRows
	}
	return items[0], nil
}

func (p *Postgres) CreatePhase2(ctx context.Context, resource string, values map[string]any) (map[string]any, error) {
	res, err := getPhase2Resource(resource)
	if err != nil {
		return nil, err
	}
	cols, args := filteredPhase2Values(res, values)
	if len(cols) == 0 {
		return nil, fmt.Errorf("no valid fields for %s", resource)
	}
	placeholders := make([]string, len(cols))
	for i := range cols {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	query := fmt.Sprintf("INSERT INTO %s(%s) VALUES(%s) RETURNING id", res.Table, strings.Join(cols, ","), strings.Join(placeholders, ","))
	var id int64
	if err := p.pool.QueryRow(ctx, query, args...).Scan(&id); err != nil {
		return nil, err
	}
	return p.GetPhase2(ctx, resource, id)
}

func (p *Postgres) UpdatePhase2(ctx context.Context, resource string, id int64, values map[string]any) (map[string]any, error) {
	res, err := getPhase2Resource(resource)
	if err != nil {
		return nil, err
	}
	cols, args := filteredPhase2Values(res, values)
	if len(cols) == 0 {
		return p.GetPhase2(ctx, resource, id)
	}
	sets := make([]string, len(cols))
	for i, col := range cols {
		sets[i] = fmt.Sprintf("%s=$%d", col, i+1)
	}
	args = append(args, id)
	query := fmt.Sprintf("UPDATE %s SET %s WHERE id=$%d", res.Table, strings.Join(sets, ","), len(args))
	if res.Cols["updated_at"] {
		query = fmt.Sprintf("UPDATE %s SET %s,updated_at=NOW() WHERE id=$%d", res.Table, strings.Join(sets, ","), len(args))
	}
	if _, err := p.pool.Exec(ctx, query, args...); err != nil {
		return nil, err
	}
	return p.GetPhase2(ctx, resource, id)
}

func (p *Postgres) DeletePhase2(ctx context.Context, resource string, id int64) error {
	res, err := getPhase2Resource(resource)
	if err != nil {
		return err
	}
	_, err = p.pool.Exec(ctx, "DELETE FROM "+res.Table+" WHERE id=$1", id)
	return err
}

func (p *Postgres) Phase2Summary(ctx context.Context) (Phase2Summary, error) {
	count := func(table string) (int, error) {
		var n int
		err := p.pool.QueryRow(ctx, "SELECT COUNT(*) FROM "+table).Scan(&n)
		return n, err
	}
	var out Phase2Summary
	var err error
	if out.Locations, err = count("locations"); err != nil {
		return out, err
	}
	if out.Subnets, err = count("subnets"); err != nil {
		return out, err
	}
	if out.Contacts, err = count("contacts"); err != nil {
		return out, err
	}
	if out.Incidents, err = count("incidents"); err != nil {
		return out, err
	}
	if out.MaintenanceWindows, err = count("maintenance_windows"); err != nil {
		return out, err
	}
	if out.StatusServices, err = count("status_page_services"); err != nil {
		return out, err
	}
	if out.DiscoveryJobs, err = count("discovery_jobs"); err != nil {
		return out, err
	}
	if out.ISPLinks, err = count("isp_links"); err != nil {
		return out, err
	}
	out.ScheduledReports, err = count("scheduled_reports")
	return out, err
}

func filteredPhase2Values(res phase2Resource, values map[string]any) ([]string, []any) {
	// Build mapping from snake_case column name to the original request key
	// so we can look up values correctly even when keys are camelCase.
	colToOriginal := make(map[string]string, len(values))
	for key := range values {
		col := toSnake(key)
		if res.Cols[col] {
			colToOriginal[col] = key
		}
	}
	cols := make([]string, 0, len(colToOriginal))
	for col := range colToOriginal {
		cols = append(cols, col)
	}
	sort.Strings(cols)
	args := make([]any, 0, len(cols))
	for _, col := range cols {
		args = append(args, normalizePhase2Value(values[colToOriginal[col]]))
	}
	return cols, args
}

func normalizePhase2Value(v any) any {
	switch t := v.(type) {
	case map[string]any, []any:
		b, _ := json.Marshal(t)
		return string(b)
	default:
		return v
	}
}

func rowsToMaps(rows pgx.Rows) ([]map[string]any, error) {
	fields := rows.FieldDescriptions()
	out := []map[string]any{}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		item := make(map[string]any, len(fields))
		for i, fd := range fields {
			item[string(fd.Name)] = values[i]
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func toSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r + ('a' - 'A'))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
