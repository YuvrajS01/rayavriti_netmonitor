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

	// V10: alert_rules
	`CREATE TABLE IF NOT EXISTS alert_rules (
		id               BIGSERIAL PRIMARY KEY,
		name             TEXT NOT NULL,
		enabled          BOOLEAN NOT NULL DEFAULT TRUE,
		device_id        BIGINT REFERENCES devices(id) ON DELETE CASCADE,
		condition        TEXT NOT NULL,
		threshold        DOUBLE PRECISION,
		severity         TEXT NOT NULL DEFAULT 'warning',
		message_template TEXT,
		created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,

	// V11: default admin user (password set via seed, placeholder hash here)
	`INSERT INTO users (username, password_hash, role, enabled)
	VALUES ('admin', 'PLACEHOLDER', 'admin', TRUE)
	ON CONFLICT (username) DO NOTHING`,

	// V12: port_scan_results
	`CREATE TABLE IF NOT EXISTS port_scan_results (
		id        BIGSERIAL PRIMARY KEY,
		device_id BIGINT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
		port      INT NOT NULL,
		protocol  TEXT NOT NULL DEFAULT 'tcp',
		state     TEXT NOT NULL,
		service   TEXT,
		scanned_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
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
}
