package database

// migrations is an ordered list of SQL migrations. Each entry is applied exactly once,
// tracked by its 1-based index (version) in the schema_migrations table.
var migrations = []string{
	// V1: migration tracking table
	`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    BIGINT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,

	// V2: users
	`CREATE TABLE IF NOT EXISTS users (
		id            BIGSERIAL PRIMARY KEY,
		username      TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		role          TEXT NOT NULL DEFAULT 'viewer',
		display_name  TEXT,
		email         TEXT,
		phone         TEXT,
		enabled       BOOLEAN NOT NULL DEFAULT TRUE,
		last_login_at TIMESTAMPTZ,
		created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		role_id       BIGINT
	)`,

	// V3: api_keys
	`CREATE TABLE IF NOT EXISTS api_keys (
		id           BIGSERIAL PRIMARY KEY,
		user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		key_hash     TEXT UNIQUE NOT NULL,
		description  TEXT,
		created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		last_used_at TIMESTAMPTZ
	)`,

	// V4: devices
	`CREATE TABLE IF NOT EXISTS devices (
		id                   BIGSERIAL PRIMARY KEY,
		name                 TEXT NOT NULL,
		ip_address           TEXT NOT NULL,
		protocol             TEXT NOT NULL DEFAULT 'ping',
		enabled              BOOLEAN NOT NULL DEFAULT TRUE,
		status               TEXT NOT NULL DEFAULT 'unknown',
		tags                 JSONB NOT NULL DEFAULT '[]',
		snmp_community       TEXT,
		snmp_version         TEXT,
		snmp_port            INT DEFAULT 161,
		http_path            TEXT,
		http_expected_status INT,
		interval_sec         INT NOT NULL DEFAULT 60,
		location_id          BIGINT,
		parent_device_id     BIGINT REFERENCES devices(id) ON DELETE SET NULL,
		rack_position        TEXT,
		asset_tag            TEXT,
		mac_address          TEXT,
		manufacturer         TEXT,
		model                TEXT,
		device_category      TEXT,
		notes                TEXT,
		created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,

	// V5: metrics hypertable
	`CREATE TABLE IF NOT EXISTS metrics (
		id            BIGSERIAL,
		device_id     BIGINT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
		timestamp     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		status        TEXT NOT NULL,
		response_time DOUBLE PRECISION,
		packet_loss   DOUBLE PRECISION,
		cpu_usage     DOUBLE PRECISION,
		memory_usage  DOUBLE PRECISION,
		bandwidth     DOUBLE PRECISION,
		custom_value  DOUBLE PRECISION,
		details       JSONB,
		PRIMARY KEY (id, timestamp)
	);
	SELECT create_hypertable('metrics', 'timestamp', if_not_exists => TRUE);`,

	// V6: alerts
	`CREATE TABLE IF NOT EXISTS alerts (
		id              BIGSERIAL PRIMARY KEY,
		device_id       BIGINT REFERENCES devices(id) ON DELETE SET NULL,
		device_name     TEXT,
		severity        TEXT NOT NULL DEFAULT 'warning',
		message         TEXT NOT NULL,
		status          TEXT NOT NULL DEFAULT 'active',
		rule_id         BIGINT,
		created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		acknowledged_at TIMESTAMPTZ,
		resolved_at     TIMESTAMPTZ,
		acknowledged_by TEXT,
		resolved_by     TEXT
	)`,

	// V7: flows hypertable
	`CREATE TABLE IF NOT EXISTS flows (
		id         BIGSERIAL,
		src_ip     INET,
		dst_ip     INET,
		src_port   INT,
		dst_port   INT,
		protocol   TEXT,
		bytes      BIGINT,
		packets    BIGINT,
		duration   DOUBLE PRECISION,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		PRIMARY KEY (id, created_at)
	);
	SELECT create_hypertable('flows', 'created_at', if_not_exists => TRUE);`,

	// V8: dashboards
	`CREATE TABLE IF NOT EXISTS dashboards (
		id         BIGSERIAL PRIMARY KEY,
		user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		name       TEXT NOT NULL,
		layout     JSONB NOT NULL DEFAULT '{}',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,

	// V9: indexes
	`CREATE INDEX IF NOT EXISTS idx_devices_ip      ON devices(ip_address);
	CREATE INDEX IF NOT EXISTS idx_metrics_device   ON metrics(device_id, timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_alerts_device    ON alerts(device_id, status, created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_flows_ips        ON flows(src_ip, dst_ip, created_at DESC);`,

	// V10: alert_rules (enriched for rule-based alert engine)
	`CREATE TABLE IF NOT EXISTS alert_rules (
		id               BIGSERIAL PRIMARY KEY,
		name             TEXT NOT NULL,
		description      TEXT,
		enabled          BOOLEAN NOT NULL DEFAULT TRUE,
		severity         TEXT NOT NULL DEFAULT 'warning',
		scope_type       TEXT NOT NULL DEFAULT 'global',
		scope_value      TEXT,
		device_id        BIGINT REFERENCES devices(id) ON DELETE CASCADE,
		condition_logic  TEXT NOT NULL DEFAULT 'all',
		cooldown_seconds INT NOT NULL DEFAULT 300,
		auto_resolve     BOOLEAN NOT NULL DEFAULT TRUE,
		created_by       BIGINT REFERENCES users(id) ON DELETE SET NULL,
		created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,

	// V11: default admin user (password set via seed, placeholder hash here)
	`INSERT INTO users (username, password_hash, role, enabled)
	VALUES ('admin', 'PLACEHOLDER', 'admin', TRUE)
	ON CONFLICT (username) DO NOTHING`,

	// V12: port_scan_results
	`CREATE TABLE IF NOT EXISTS port_scan_results (
		id             BIGSERIAL PRIMARY KEY,
		device_id      BIGINT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
		port           INT NOT NULL,
		protocol       TEXT NOT NULL DEFAULT 'tcp',
		state          TEXT NOT NULL,
		service        TEXT,
		response_time  DOUBLE PRECISION,
		first_seen     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		last_seen      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		last_changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		scanned_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		UNIQUE(device_id, port, protocol)
	)`,

	// V13: monitoring_http_requests hypertable
	`CREATE TABLE IF NOT EXISTS monitoring_http_requests (
		id            BIGSERIAL,
		request_id    TEXT,
		method        TEXT NOT NULL,
		path          TEXT NOT NULL,
		status_code   INT NOT NULL,
		duration_ms   DOUBLE PRECISION NOT NULL,
		user_id       BIGINT,
		remote_addr   TEXT,
		user_agent    TEXT,
		response_size BIGINT,
		timestamp     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		PRIMARY KEY (id, timestamp)
	);
	SELECT create_hypertable('monitoring_http_requests', 'timestamp', if_not_exists => TRUE);`,

	// V14: monitoring_db_queries hypertable
	`CREATE TABLE IF NOT EXISTS monitoring_db_queries (
		id             BIGSERIAL,
		trace_id       TEXT,
		operation      TEXT NOT NULL,
		table_name     TEXT,
		duration_ms    DOUBLE PRECISION NOT NULL,
		rows_returned  BIGINT,
		slow_query     BOOLEAN NOT NULL DEFAULT FALSE,
		timestamp      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		PRIMARY KEY (id, timestamp)
	);
	SELECT create_hypertable('monitoring_db_queries', 'timestamp', if_not_exists => TRUE);`,

	// V15: monitoring_collector_runs hypertable
	`CREATE TABLE IF NOT EXISTS monitoring_collector_runs (
		id          BIGSERIAL,
		device_id   BIGINT NOT NULL,
		protocol    TEXT NOT NULL,
		status      TEXT NOT NULL,
		duration_ms DOUBLE PRECISION NOT NULL,
		error       TEXT,
		timestamp   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		PRIMARY KEY (id, timestamp)
	);
	SELECT create_hypertable('monitoring_collector_runs', 'timestamp', if_not_exists => TRUE);`,

	// V16: monitoring_system_metrics hypertable
	`CREATE TABLE IF NOT EXISTS monitoring_system_metrics (
		id              BIGSERIAL,
		memory_used_mb  DOUBLE PRECISION NOT NULL,
		goroutines      INT NOT NULL,
		gc_pause_ms_avg DOUBLE PRECISION,
		uptime_seconds  BIGINT NOT NULL,
		timestamp       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		PRIMARY KEY (id, timestamp)
	);
	SELECT create_hypertable('monitoring_system_metrics', 'timestamp', if_not_exists => TRUE);`,

	// V17: monitoring_alerts hypertable
	`CREATE TABLE IF NOT EXISTS monitoring_alerts (
		id         BIGSERIAL,
		alert_id   BIGINT NOT NULL,
		rule_id    BIGINT,
		device_id  BIGINT,
		severity   TEXT NOT NULL,
		event_type TEXT NOT NULL,
		message    TEXT,
		timestamp  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		PRIMARY KEY (id, timestamp)
	);
	SELECT create_hypertable('monitoring_alerts', 'timestamp', if_not_exists => TRUE);`,

	// V18: sensors table
	`CREATE TABLE IF NOT EXISTS sensors (
		id          BIGSERIAL PRIMARY KEY,
		device_id   BIGINT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
		name        TEXT NOT NULL,
		type        TEXT NOT NULL,
		enabled     BOOLEAN NOT NULL DEFAULT TRUE,
		interval    INT NOT NULL DEFAULT 60,
		config      JSONB NOT NULL DEFAULT '{}',
		created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_sensors_device ON sensors(device_id);`,

	// V19: capture_sessions table
	`CREATE TABLE IF NOT EXISTS capture_sessions (
		id              BIGSERIAL PRIMARY KEY,
		interface_name  TEXT NOT NULL,
		filter          TEXT NOT NULL DEFAULT '',
		status          TEXT NOT NULL DEFAULT 'running',
		started_by      TEXT,
		total_packets   BIGINT NOT NULL DEFAULT 0,
		total_bytes     BIGINT NOT NULL DEFAULT 0,
		protocols       JSONB NOT NULL DEFAULT '{}',
		started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		stopped_at      TIMESTAMPTZ,
		error_message   TEXT
	)`,

	// V20: alert_rule_conditions table
	`CREATE TABLE IF NOT EXISTS alert_rule_conditions (
		id               BIGSERIAL PRIMARY KEY,
		rule_id          BIGINT NOT NULL REFERENCES alert_rules(id) ON DELETE CASCADE,
		type             TEXT NOT NULL,
		metric_field     TEXT,
		operator         TEXT,
		value            TEXT,
		duration_seconds INT DEFAULT 0,
		config           JSONB NOT NULL DEFAULT '{}'
	);
	CREATE INDEX IF NOT EXISTS idx_conditions_rule ON alert_rule_conditions(rule_id);`,

	// V21: notification_channels table
	`CREATE TABLE IF NOT EXISTS notification_channels (
		id         BIGSERIAL PRIMARY KEY,
		name       TEXT NOT NULL,
		type       TEXT NOT NULL,
		config     JSONB NOT NULL DEFAULT '{}',
		enabled    BOOLEAN NOT NULL DEFAULT TRUE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,

	// V22: alert_rule_channels (many-to-many)
	`CREATE TABLE IF NOT EXISTS alert_rule_channels (
		rule_id    BIGINT NOT NULL REFERENCES alert_rules(id) ON DELETE CASCADE,
		channel_id BIGINT NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
		PRIMARY KEY (rule_id, channel_id)
	)`,

	// V23: alert_history hypertable
	`CREATE TABLE IF NOT EXISTS alert_history (
		id         BIGSERIAL,
		alert_id   BIGINT NOT NULL,
		rule_id    BIGINT,
		action     TEXT NOT NULL,
		actor      TEXT,
		details    JSONB,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		PRIMARY KEY (id, created_at)
	);
	SELECT create_hypertable('alert_history', 'created_at', if_not_exists => TRUE);
	CREATE INDEX IF NOT EXISTS idx_alert_history_alert ON alert_history(alert_id, created_at DESC);`,

	// V24: alert_rule_state (durable per-rule/per-device state)
	`CREATE TABLE IF NOT EXISTS alert_rule_state (
		rule_id            BIGINT NOT NULL REFERENCES alert_rules(id) ON DELETE CASCADE,
		device_id          BIGINT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
		state              TEXT NOT NULL DEFAULT 'idle',
		first_met_at       TIMESTAMPTZ,
		last_evaluated_at  TIMESTAMPTZ,
		last_fired_at      TIMESTAMPTZ,
		last_resolved_at   TIMESTAMPTZ,
		active_alert_id    BIGINT,
		condition_snapshot JSONB,
		PRIMARY KEY (rule_id, device_id)
	)`,

	// V25: monitoring_audit_log table
	`CREATE TABLE IF NOT EXISTS monitoring_audit_log (
		id            BIGSERIAL PRIMARY KEY,
		request_id    TEXT,
		event_type    TEXT NOT NULL,
		severity      TEXT NOT NULL DEFAULT 'info',
		actor         TEXT,
		actor_ip      TEXT,
		resource_type TEXT,
		resource_id   TEXT,
		description   TEXT NOT NULL,
		details       JSONB,
		created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_mon_audit_time   ON monitoring_audit_log(created_at);
	CREATE INDEX IF NOT EXISTS idx_mon_audit_event  ON monitoring_audit_log(event_type);
	CREATE INDEX IF NOT EXISTS idx_mon_audit_actor  ON monitoring_audit_log(actor);`,

	// V26: monitoring_app_health table
	`CREATE TABLE IF NOT EXISTS monitoring_app_health (
		id                      BIGSERIAL PRIMARY KEY,
		uptime_seconds          BIGINT NOT NULL,
		goroutine_count         INT NOT NULL,
		heap_alloc_bytes        BIGINT NOT NULL,
		heap_sys_bytes          BIGINT NOT NULL,
		stack_in_use_bytes      BIGINT NOT NULL,
		gc_pause_total_ns       BIGINT NOT NULL,
		gc_runs                 INT NOT NULL,
		gc_last_pause_ns        BIGINT,
		num_cpu                 INT NOT NULL,
		active_ws_connections   INT NOT NULL DEFAULT 0,
		active_capture_sessions INT NOT NULL DEFAULT 0,
		scheduler_jobs_active   INT NOT NULL DEFAULT 0,
		db_open_connections     INT,
		db_idle_connections     INT,
		db_wait_count           BIGINT,
		db_wait_duration_ms     DOUBLE PRECISION,
		requests_total          BIGINT NOT NULL DEFAULT 0,
		requests_active         INT NOT NULL DEFAULT 0,
		errors_total            BIGINT NOT NULL DEFAULT 0,
		created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_mon_health_time ON monitoring_app_health(created_at);`,

	// V27: monitoring_alert_activity table
	`CREATE TABLE IF NOT EXISTS monitoring_alert_activity (
		id            BIGSERIAL,
		trace_id      TEXT,
		rule_id       BIGINT,
		rule_name     TEXT,
		device_id     BIGINT,
		device_name   TEXT,
		action        TEXT NOT NULL,
		severity      TEXT,
		alert_id      BIGINT,
		channel_id    BIGINT,
		channel_type  TEXT,
		details       JSONB,
		duration_ms   DOUBLE PRECISION,
		error_message TEXT,
		created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		PRIMARY KEY (id, created_at)
	);
	SELECT create_hypertable('monitoring_alert_activity', 'created_at', if_not_exists => TRUE);
	CREATE INDEX IF NOT EXISTS idx_mon_alert_act_rule   ON monitoring_alert_activity(rule_id);
	CREATE INDEX IF NOT EXISTS idx_mon_alert_act_device ON monitoring_alert_activity(device_id);
	CREATE INDEX IF NOT EXISTS idx_mon_alert_act_action ON monitoring_alert_activity(action);`,

	// V28: additional indexes for Phase 2
	`CREATE INDEX IF NOT EXISTS idx_alerts_status     ON alerts(status, created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_alerts_rule        ON alerts(rule_id);
	CREATE INDEX IF NOT EXISTS idx_metrics_status     ON metrics(device_id, status, timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_devices_status     ON devices(status, enabled);
	CREATE INDEX IF NOT EXISTS idx_port_scan_device   ON port_scan_results(device_id, scanned_at DESC);
	CREATE INDEX IF NOT EXISTS idx_capture_status     ON capture_sessions(status);`,

	// V29: capture_packets table (was missing — queried by GetCapturePackets)
	`CREATE TABLE IF NOT EXISTS capture_packets (
		id          BIGSERIAL,
		session_id  BIGINT NOT NULL REFERENCES capture_sessions(id) ON DELETE CASCADE,
		timestamp   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		src_ip      TEXT,
		dst_ip      TEXT,
		src_port    INT,
		dst_port    INT,
		protocol    TEXT,
		length      INT,
		flags       TEXT,
		payload     TEXT,
		PRIMARY KEY (id, timestamp)
	);
	SELECT create_hypertable('capture_packets', 'timestamp', if_not_exists => TRUE);
	CREATE INDEX IF NOT EXISTS idx_capture_packets_session ON capture_packets(session_id, timestamp ASC);`,

	// V30: add port column to devices
	`ALTER TABLE devices ADD COLUMN IF NOT EXISTS port INT NOT NULL DEFAULT 0;`,

	// V31: refresh_tokens table for DB-backed session revocation
	`CREATE TABLE IF NOT EXISTS refresh_tokens (
		id           BIGSERIAL PRIMARY KEY,
		token_hash   TEXT NOT NULL UNIQUE,
		user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		expires_at   TIMESTAMPTZ NOT NULL,
		created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_refresh_tokens_hash ON refresh_tokens(token_hash);
	CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user ON refresh_tokens(user_id);`,
}
