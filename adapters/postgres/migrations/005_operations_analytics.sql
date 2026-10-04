CREATE TABLE financial_operations (
    user_id TEXT NOT NULL REFERENCES users(id),
    operation TEXT NOT NULL,
    fingerprint TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, operation)
);

CREATE TABLE IF NOT EXISTS analytics (
    id TEXT PRIMARY KEY, timestamp TEXT NOT NULL, channel TEXT NOT NULL,
    module TEXT NOT NULL, action TEXT NOT NULL, record_id TEXT,
    user_id TEXT, api_key_id TEXT, remote_ip TEXT,
    duration_ns BIGINT NOT NULL, memory_bytes BIGINT DEFAULT 0,
    request_bytes BIGINT DEFAULT 0, response_bytes BIGINT DEFAULT 0,
    success INTEGER NOT NULL, status_code INTEGER, error TEXT,
    cost_units DOUBLE PRECISION DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_analytics_timestamp ON analytics(timestamp);
CREATE INDEX IF NOT EXISTS idx_analytics_module_action ON analytics(module, action);
CREATE INDEX IF NOT EXISTS idx_analytics_user ON analytics(user_id);
CREATE INDEX IF NOT EXISTS idx_analytics_api_key ON analytics(api_key_id);
CREATE TABLE analytics_outbox (
    id TEXT PRIMARY KEY, payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    processed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_analytics_outbox_pending ON analytics_outbox(created_at) WHERE processed_at IS NULL;
