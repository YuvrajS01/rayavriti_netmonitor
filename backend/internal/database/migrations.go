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

	// V32: enable TimescaleDB extension and recreate hypertables
	`CREATE EXTENSION IF NOT EXISTS timescaledb;
	SELECT create_hypertable('metrics', 'timestamp', if_not_exists => TRUE);
	SELECT create_hypertable('flows', 'created_at', if_not_exists => TRUE);
	SELECT create_hypertable('alert_history', 'created_at', if_not_exists => TRUE);
	SELECT create_hypertable('capture_packets', 'timestamp', if_not_exists => TRUE);
	SELECT create_hypertable('monitoring_http_requests', 'timestamp', if_not_exists => TRUE);
	SELECT create_hypertable('monitoring_db_queries', 'timestamp', if_not_exists => TRUE);
	SELECT create_hypertable('monitoring_collector_runs', 'timestamp', if_not_exists => TRUE);
	SELECT create_hypertable('monitoring_system_metrics', 'timestamp', if_not_exists => TRUE);
	SELECT create_hypertable('monitoring_alerts', 'timestamp', if_not_exists => TRUE);
	SELECT create_hypertable('monitoring_alert_activity', 'created_at', if_not_exists => TRUE);`,

	// V33: health_scores (latest snapshot per device), health_score_history (time series), alerts.group_id
	`CREATE TABLE IF NOT EXISTS health_scores (
		device_id      BIGINT PRIMARY KEY REFERENCES devices(id) ON DELETE CASCADE,
		score          REAL NOT NULL,
		label          TEXT NOT NULL,
		trend          TEXT NOT NULL DEFAULT 'stable',
		trend_delta    REAL NOT NULL DEFAULT 0,
		factors        JSONB NOT NULL DEFAULT '{}',
		issues         JSONB NOT NULL DEFAULT '[]',
		computed_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS health_score_history (
		id          BIGSERIAL PRIMARY KEY,
		device_id   BIGINT REFERENCES devices(id) ON DELETE CASCADE,
		score       REAL NOT NULL,
		label       TEXT NOT NULL,
		factors     JSONB,
		computed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_hsh_device_time ON health_score_history(device_id, computed_at DESC);

	ALTER TABLE alerts ADD COLUMN IF NOT EXISTS group_id TEXT;`,

	// V34: Phase 2 campus-grade schema additions
	`CREATE TABLE IF NOT EXISTS roles (
		id           BIGSERIAL PRIMARY KEY,
		name         TEXT NOT NULL UNIQUE,
		display_name TEXT NOT NULL,
		description  TEXT,
		permissions  JSONB NOT NULL DEFAULT '[]',
		is_system    BOOLEAN NOT NULL DEFAULT FALSE,
		created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS contacts (
		id                   BIGSERIAL PRIMARY KEY,
		name                 TEXT NOT NULL,
		designation          TEXT,
		department           TEXT,
		email                TEXT,
		phone                TEXT,
		telegram_chat_id     TEXT,
		whatsapp_number      TEXT,
		preferred_channel    TEXT NOT NULL DEFAULT 'email',
		notification_enabled BOOLEAN NOT NULL DEFAULT TRUE,
		quiet_hours_start    TEXT,
		quiet_hours_end      TEXT,
		user_id              BIGINT REFERENCES users(id) ON DELETE SET NULL,
		enabled              BOOLEAN NOT NULL DEFAULT TRUE,
		created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS locations (
		id                BIGSERIAL PRIMARY KEY,
		name              TEXT NOT NULL,
		type              TEXT NOT NULL,
		parent_id         BIGINT REFERENCES locations(id) ON DELETE SET NULL,
		code              TEXT UNIQUE,
		description       TEXT,
		address           TEXT,
		latitude          REAL,
		longitude         REAL,
		floor_number      INTEGER,
		contact_person_id BIGINT REFERENCES contacts(id) ON DELETE SET NULL,
		metadata          JSONB NOT NULL DEFAULT '{}',
		sort_order        INTEGER NOT NULL DEFAULT 0,
		enabled           BOOLEAN NOT NULL DEFAULT TRUE,
		created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_loc_parent ON locations(parent_id);
	CREATE INDEX IF NOT EXISTS idx_loc_type ON locations(type);
	CREATE INDEX IF NOT EXISTS idx_loc_code ON locations(code);

	CREATE TABLE IF NOT EXISTS subnets (
		id           BIGSERIAL PRIMARY KEY,
		name         TEXT NOT NULL,
		vlan_id      BIGINT,
		cidr         CIDR NOT NULL,
		gateway      INET,
		description  TEXT,
		location_id  BIGINT REFERENCES locations(id) ON DELETE SET NULL,
		dns_servers  INET[],
		dhcp_enabled BOOLEAN NOT NULL DEFAULT FALSE,
		created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_subnets_vlan ON subnets(vlan_id);
	CREATE INDEX IF NOT EXISTS idx_subnets_location ON subnets(location_id);

	ALTER TABLE devices ADD COLUMN IF NOT EXISTS location_id BIGINT REFERENCES locations(id) ON DELETE SET NULL;
	ALTER TABLE devices ADD COLUMN IF NOT EXISTS parent_device_id BIGINT REFERENCES devices(id) ON DELETE SET NULL;
	ALTER TABLE devices ADD COLUMN IF NOT EXISTS dependency_port TEXT;
	ALTER TABLE devices ADD COLUMN IF NOT EXISTS rack_position TEXT;
	ALTER TABLE devices ADD COLUMN IF NOT EXISTS asset_tag TEXT;
	ALTER TABLE devices ADD COLUMN IF NOT EXISTS mac_address TEXT;
	ALTER TABLE devices ADD COLUMN IF NOT EXISTS serial_number TEXT;
	ALTER TABLE devices ADD COLUMN IF NOT EXISTS manufacturer TEXT;
	ALTER TABLE devices ADD COLUMN IF NOT EXISTS model TEXT;
	ALTER TABLE devices ADD COLUMN IF NOT EXISTS device_category TEXT;
	ALTER TABLE devices ADD COLUMN IF NOT EXISTS notes TEXT;
	CREATE INDEX IF NOT EXISTS idx_devices_location ON devices(location_id);
	CREATE INDEX IF NOT EXISTS idx_devices_parent ON devices(parent_device_id);
	CREATE INDEX IF NOT EXISTS idx_devices_category ON devices(device_category);

	ALTER TABLE users ADD COLUMN IF NOT EXISTS role_id BIGINT REFERENCES roles(id);
	ALTER TABLE users ADD COLUMN IF NOT EXISTS display_name TEXT;
	ALTER TABLE users ADD COLUMN IF NOT EXISTS email TEXT;
	ALTER TABLE users ADD COLUMN IF NOT EXISTS phone TEXT;
	ALTER TABLE users ADD COLUMN IF NOT EXISTS contact_id BIGINT REFERENCES contacts(id) ON DELETE SET NULL;
	ALTER TABLE users ADD COLUMN IF NOT EXISTS enabled BOOLEAN NOT NULL DEFAULT TRUE;
	ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMPTZ;

	CREATE TABLE IF NOT EXISTS suppressed_alerts (
		id                   BIGSERIAL,
		device_id            BIGINT NOT NULL REFERENCES devices(id),
		rule_id              BIGINT,
		suppression_reason   TEXT NOT NULL,
		root_cause_device_id BIGINT REFERENCES devices(id),
		root_cause_alert_id  BIGINT REFERENCES alerts(id),
		would_have_fired_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		released_at          TIMESTAMPTZ,
		created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_suppressed_device ON suppressed_alerts(device_id);
	CREATE INDEX IF NOT EXISTS idx_suppressed_root ON suppressed_alerts(root_cause_device_id);

	CREATE TABLE IF NOT EXISTS discovery_jobs (
		id                BIGSERIAL PRIMARY KEY,
		subnet            TEXT NOT NULL,
		scan_type         TEXT NOT NULL,
		status            TEXT NOT NULL DEFAULT 'running',
		location_id        BIGINT REFERENCES locations(id),
		initiated_by       TEXT,
		total_ips_scanned  INTEGER NOT NULL DEFAULT 0,
		devices_found      INTEGER NOT NULL DEFAULT 0,
		devices_new        INTEGER NOT NULL DEFAULT 0,
		devices_known      INTEGER NOT NULL DEFAULT 0,
		started_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		completed_at       TIMESTAMPTZ,
		error_message      TEXT
	);

	CREATE TABLE IF NOT EXISTS discovery_results (
		id                 BIGSERIAL PRIMARY KEY,
		job_id             BIGINT NOT NULL REFERENCES discovery_jobs(id) ON DELETE CASCADE,
		ip_address         TEXT NOT NULL,
		mac_address        TEXT,
		manufacturer       TEXT,
		hostname           TEXT,
		device_description TEXT,
		guessed_category   TEXT,
		guessed_os         TEXT,
		open_ports         JSONB NOT NULL DEFAULT '[]',
		snmp_reachable     BOOLEAN NOT NULL DEFAULT FALSE,
		response_time_ms   REAL,
		status             TEXT NOT NULL DEFAULT 'pending',
		approved_device_id BIGINT REFERENCES devices(id),
		discovered_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		http_title         TEXT,
		ssh_banner         TEXT,
		tls_cert_cn        TEXT,
		snmp_name          TEXT,
		snmp_description   TEXT,
		snmp_sys_object_id TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_disc_results_job ON discovery_results(job_id);
	CREATE INDEX IF NOT EXISTS idx_disc_results_status ON discovery_results(status);

	CREATE TABLE IF NOT EXISTS status_page_services (
		id                 BIGSERIAL PRIMARY KEY,
		name               TEXT NOT NULL,
		description        TEXT,
		group_name         TEXT NOT NULL DEFAULT 'General',
		aggregation        TEXT NOT NULL DEFAULT 'any_down',
		display_order      INTEGER NOT NULL DEFAULT 0,
		show_response_time BOOLEAN NOT NULL DEFAULT FALSE,
		show_uptime        BOOLEAN NOT NULL DEFAULT TRUE,
		enabled            BOOLEAN NOT NULL DEFAULT TRUE,
		created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS status_page_service_devices (
		service_id BIGINT NOT NULL REFERENCES status_page_services(id) ON DELETE CASCADE,
		device_id  BIGINT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
		PRIMARY KEY (service_id, device_id)
	);
	CREATE INDEX IF NOT EXISTS idx_status_service_devices_device ON status_page_service_devices(device_id);

	CREATE TABLE IF NOT EXISTS status_page_incidents (
		id          BIGSERIAL PRIMARY KEY,
		title       TEXT NOT NULL,
		message     TEXT NOT NULL,
		severity    TEXT NOT NULL DEFAULT 'info',
		status      TEXT NOT NULL DEFAULT 'investigating',
		started_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		resolved_at TIMESTAMPTZ,
		created_by  BIGINT REFERENCES users(id),
		created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS status_page_incident_services (
		incident_id BIGINT NOT NULL REFERENCES status_page_incidents(id) ON DELETE CASCADE,
		service_id  BIGINT NOT NULL REFERENCES status_page_services(id) ON DELETE CASCADE,
		PRIMARY KEY (incident_id, service_id)
	);

	CREATE TABLE IF NOT EXISTS status_page_incident_updates (
		id          BIGSERIAL PRIMARY KEY,
		incident_id BIGINT NOT NULL REFERENCES status_page_incidents(id) ON DELETE CASCADE,
		status      TEXT NOT NULL,
		message     TEXT NOT NULL,
		created_by  BIGINT REFERENCES users(id),
		created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS maintenance_windows (
		id                      BIGSERIAL PRIMARY KEY,
		name                    TEXT NOT NULL,
		description             TEXT,
		scope_type              TEXT NOT NULL,
		scope_value             TEXT NOT NULL,
		schedule_type           TEXT NOT NULL,
		start_time              TIMESTAMPTZ,
		end_time                TIMESTAMPTZ,
		recurrence_rule         TEXT,
		recurrence_start_time   TEXT,
		recurrence_end_time     TEXT,
		recurrence_timezone     TEXT NOT NULL DEFAULT 'Asia/Kolkata',
		suppress_alerts         BOOLEAN NOT NULL DEFAULT TRUE,
		suppress_notifications  BOOLEAN NOT NULL DEFAULT TRUE,
		show_maintenance_status BOOLEAN NOT NULL DEFAULT TRUE,
		created_by              BIGINT REFERENCES users(id),
		enabled                 BOOLEAN NOT NULL DEFAULT TRUE,
		created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_maint_scope ON maintenance_windows(scope_type, scope_value);
	CREATE INDEX IF NOT EXISTS idx_maint_schedule ON maintenance_windows(schedule_type, start_time, end_time);

	CREATE TABLE IF NOT EXISTS device_contacts (
		id          BIGSERIAL PRIMARY KEY,
		device_id   BIGINT REFERENCES devices(id) ON DELETE CASCADE,
		location_id BIGINT REFERENCES locations(id) ON DELETE CASCADE,
		contact_id  BIGINT NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
		role        TEXT NOT NULL DEFAULT 'primary',
		notify_on   TEXT NOT NULL DEFAULT 'critical,warning',
		created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_dc_device ON device_contacts(device_id);
	CREATE INDEX IF NOT EXISTS idx_dc_location ON device_contacts(location_id);

	CREATE TABLE IF NOT EXISTS escalation_policies (
		id          BIGSERIAL PRIMARY KEY,
		name        TEXT NOT NULL,
		description TEXT,
		scope_type  TEXT NOT NULL DEFAULT 'global',
		scope_value TEXT,
		enabled     BOOLEAN NOT NULL DEFAULT TRUE,
		created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS escalation_steps (
		id                      BIGSERIAL PRIMARY KEY,
		policy_id               BIGINT NOT NULL REFERENCES escalation_policies(id) ON DELETE CASCADE,
		step_order              INTEGER NOT NULL,
		contact_id              BIGINT NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
		delay_minutes           INTEGER NOT NULL,
		notify_via              TEXT NOT NULL DEFAULT 'preferred',
		repeat_count            INTEGER NOT NULL DEFAULT 1,
		repeat_interval_minutes INTEGER NOT NULL DEFAULT 5,
		created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_esc_steps_policy ON escalation_steps(policy_id, step_order);

	CREATE TABLE IF NOT EXISTS oncall_schedules (
		id            BIGSERIAL PRIMARY KEY,
		name          TEXT NOT NULL,
		policy_id     BIGINT NOT NULL REFERENCES escalation_policies(id) ON DELETE CASCADE,
		rotation_type TEXT NOT NULL DEFAULT 'weekly',
		participants  JSONB NOT NULL DEFAULT '[]',
		current_index INTEGER NOT NULL DEFAULT 0,
		rotation_time TEXT NOT NULL DEFAULT '09:00',
		timezone      TEXT NOT NULL DEFAULT 'Asia/Kolkata',
		enabled       BOOLEAN NOT NULL DEFAULT TRUE,
		created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS incidents (
		id                    BIGSERIAL PRIMARY KEY,
		title                 TEXT NOT NULL,
		description           TEXT,
		severity              TEXT NOT NULL,
		status                TEXT NOT NULL DEFAULT 'open',
		root_cause            TEXT,
		root_cause_category   TEXT,
		resolution            TEXT,
		source                TEXT NOT NULL DEFAULT 'auto',
		source_alert_id       BIGINT REFERENCES alerts(id),
		assigned_to           BIGINT REFERENCES contacts(id),
		location_id           BIGINT REFERENCES locations(id),
		impact_description    TEXT,
		affected_device_count INTEGER NOT NULL DEFAULT 0,
		started_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		acknowledged_at       TIMESTAMPTZ,
		resolved_at           TIMESTAMPTZ,
		closed_at             TIMESTAMPTZ,
		duration_seconds      INTEGER,
		sla_breached          BOOLEAN NOT NULL DEFAULT FALSE,
		created_by            BIGINT REFERENCES users(id),
		created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_incidents_status ON incidents(status);
	CREATE INDEX IF NOT EXISTS idx_incidents_severity ON incidents(severity);
	CREATE INDEX IF NOT EXISTS idx_incidents_started ON incidents(started_at);
	CREATE INDEX IF NOT EXISTS idx_incidents_location ON incidents(location_id);

	CREATE TABLE IF NOT EXISTS incident_devices (
		incident_id BIGINT NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
		device_id   BIGINT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
		PRIMARY KEY (incident_id, device_id)
	);

	CREATE TABLE IF NOT EXISTS incident_timeline (
		id          BIGSERIAL,
		incident_id BIGINT NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
		entry_type  TEXT NOT NULL,
		old_value   TEXT,
		new_value   TEXT,
		message     TEXT NOT NULL,
		author      TEXT,
		created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_inc_timeline ON incident_timeline(incident_id, created_at);

	CREATE TABLE IF NOT EXISTS sla_definitions (
		id                      BIGSERIAL PRIMARY KEY,
		name                    TEXT NOT NULL,
		severity                TEXT NOT NULL UNIQUE,
		response_time_minutes   INTEGER NOT NULL,
		resolution_time_minutes INTEGER NOT NULL,
		enabled                 BOOLEAN NOT NULL DEFAULT TRUE,
		created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS user_scopes (
		id          BIGSERIAL PRIMARY KEY,
		user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		scope_type  TEXT NOT NULL,
		scope_value TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_user_scopes ON user_scopes(user_id);

	CREATE TABLE IF NOT EXISTS notification_log (
		id              BIGSERIAL,
		alert_id        BIGINT REFERENCES alerts(id),
		incident_id     BIGINT REFERENCES incidents(id),
		contact_id      BIGINT NOT NULL REFERENCES contacts(id),
		channel_type    TEXT NOT NULL,
		recipient       TEXT NOT NULL,
		message_preview TEXT,
		status          TEXT NOT NULL,
		external_id     TEXT,
		error_message   TEXT,
		attempt_count   INTEGER NOT NULL DEFAULT 1,
		escalation_step INTEGER,
		sent_at         TIMESTAMPTZ,
		delivered_at    TIMESTAMPTZ,
		read_at         TIMESTAMPTZ,
		created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_notif_log_alert ON notification_log(alert_id);
	CREATE INDEX IF NOT EXISTS idx_notif_log_contact ON notification_log(contact_id);
	CREATE INDEX IF NOT EXISTS idx_notif_log_status ON notification_log(status);

	CREATE TABLE IF NOT EXISTS scheduled_reports (
		id               BIGSERIAL PRIMARY KEY,
		name             TEXT NOT NULL,
		report_type      TEXT NOT NULL,
		format           TEXT NOT NULL DEFAULT 'pdf',
		schedule_cron    TEXT NOT NULL,
		timezone         TEXT NOT NULL DEFAULT 'Asia/Kolkata',
		scope_type       TEXT NOT NULL DEFAULT 'global',
		scope_value      TEXT,
		recipients       JSONB NOT NULL DEFAULT '[]',
		include_charts   BOOLEAN NOT NULL DEFAULT TRUE,
		lookback_period  TEXT NOT NULL DEFAULT '7d',
		custom_from      TIMESTAMPTZ,
		custom_to        TIMESTAMPTZ,
		last_run_at      TIMESTAMPTZ,
		last_run_status  TEXT,
		enabled          BOOLEAN NOT NULL DEFAULT TRUE,
		created_by       BIGINT REFERENCES users(id),
		created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_sched_reports_cron ON scheduled_reports(enabled, schedule_cron);

	CREATE TABLE IF NOT EXISTS generated_reports (
		id                  BIGSERIAL PRIMARY KEY,
		scheduled_report_id BIGINT REFERENCES scheduled_reports(id) ON DELETE SET NULL,
		report_type         TEXT NOT NULL,
		title               TEXT NOT NULL,
		format              TEXT NOT NULL,
		file_path           TEXT NOT NULL,
		file_size_bytes     INTEGER,
		scope_description   TEXT,
		period_from         TIMESTAMPTZ,
		period_to           TIMESTAMPTZ,
		recipients          TEXT,
		generated_by        TEXT,
		generated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS isp_links (
		id                          BIGSERIAL PRIMARY KEY,
		name                        TEXT NOT NULL,
		provider                    TEXT NOT NULL,
		circuit_id                  TEXT,
		bandwidth_mbps              INTEGER,
		gateway_ip                  TEXT NOT NULL,
		sla_uptime_percent          REAL,
		cost_monthly                REAL,
		contract_start              DATE,
		contract_end                DATE,
		monitoring_interval_seconds INTEGER NOT NULL DEFAULT 10,
		enabled                     BOOLEAN NOT NULL DEFAULT TRUE,
		created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS isp_metrics (
		id                  BIGSERIAL,
		link_id             BIGINT NOT NULL REFERENCES isp_links(id) ON DELETE CASCADE,
		latency_ms          REAL,
		jitter_ms           REAL,
		packet_loss_percent REAL,
		download_speed_mbps REAL,
		upload_speed_mbps   REAL,
		status              TEXT NOT NULL,
		target_ip           TEXT,
		created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_isp_metrics_link ON isp_metrics(link_id, created_at);

	INSERT INTO roles(name, display_name, description, permissions, is_system)
	VALUES
		('super_admin', 'Super Admin', 'Full platform access', '["*"]', TRUE),
		('network_admin', 'Network Admin', 'Network operations access', '["devices.read","devices.write","alerts.read","alerts.acknowledge","alerts.resolve","alert_rules.write","incidents.write","maintenance.write","contacts.write","reports.read","import.execute","discovery.execute","capture.execute","status_page.manage","system.monitoring"]', TRUE),
		('dept_admin', 'Department Admin', 'Scoped department operations', '["devices.read","alerts.read","alerts.acknowledge","incidents.write","reports.read"]', TRUE),
		('viewer', 'Viewer', 'Scoped read-only access', '["devices.read","alerts.read"]', TRUE),
		('public', 'Public', 'Public status access', '[]', TRUE)
	ON CONFLICT (name) DO UPDATE SET display_name=EXCLUDED.display_name, permissions=EXCLUDED.permissions, is_system=EXCLUDED.is_system;

	INSERT INTO locations(name, type, code, description)
	VALUES ('Main Campus', 'campus', 'MC', 'Default Phase 2 root location')
	ON CONFLICT (code) DO NOTHING;

	INSERT INTO sla_definitions(name, severity, response_time_minutes, resolution_time_minutes)
	VALUES
		('Critical SLA', 'critical', 15, 120),
		('Major SLA', 'major', 30, 480),
		('Minor SLA', 'minor', 60, 1440)
	ON CONFLICT (severity) DO UPDATE SET response_time_minutes=EXCLUDED.response_time_minutes, resolution_time_minutes=EXCLUDED.resolution_time_minutes;

	INSERT INTO escalation_policies(name, description, scope_type, enabled)
	VALUES ('Default IT Escalation', 'Default Phase 2 escalation policy', 'global', TRUE)
	ON CONFLICT DO NOTHING;

	UPDATE users SET role_id=(SELECT id FROM roles WHERE name='super_admin' LIMIT 1), role='super_admin'
	WHERE username='admin';

	CREATE EXTENSION IF NOT EXISTS timescaledb;
	SELECT create_hypertable('suppressed_alerts', 'created_at', if_not_exists => TRUE);
	SELECT create_hypertable('notification_log', 'created_at', if_not_exists => TRUE);
	SELECT create_hypertable('incident_timeline', 'created_at', if_not_exists => TRUE);
	SELECT create_hypertable('isp_metrics', 'created_at', if_not_exists => TRUE);`,
}
