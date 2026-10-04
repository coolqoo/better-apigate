-- Fresh PostgreSQL schema. Historical SQLite migrations are intentionally not replayed.
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    stripe_id TEXT,
    plan_id TEXT NOT NULL DEFAULT 'free',
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
, password_hash BYTEA, role TEXT NOT NULL DEFAULT 'user');

CREATE TABLE usage_summaries (
    user_id TEXT NOT NULL,
    period_start TIMESTAMPTZ NOT NULL,
    period_end TIMESTAMPTZ NOT NULL,
    request_count BIGINT NOT NULL DEFAULT 0,
    compute_units DOUBLE PRECISION NOT NULL DEFAULT 0,
    bytes_in BIGINT NOT NULL DEFAULT 0,
    bytes_out BIGINT NOT NULL DEFAULT 0,
    error_count BIGINT NOT NULL DEFAULT 0,
    avg_latency_ms BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, period_start)
);

CREATE TABLE upstreams (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',

    -- Target
    base_url TEXT NOT NULL,
    timeout_ms BIGINT NOT NULL DEFAULT 30000,

    -- Connection pooling
    max_idle_conns BIGINT NOT NULL DEFAULT 100,
    idle_conn_timeout_ms BIGINT NOT NULL DEFAULT 90000,

    -- Authentication injection
    auth_type TEXT NOT NULL DEFAULT 'none', -- none, header, bearer, basic
    auth_header TEXT,                        -- Header name for auth_type=header
    auth_value_encrypted BYTEA,               -- Encrypted auth value

    -- Metadata
    enabled BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE plans (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    rate_limit_per_minute BIGINT DEFAULT 60,
    requests_per_month BIGINT DEFAULT 1000,
    price_monthly BIGINT DEFAULT 0,
    overage_price BIGINT DEFAULT 0,
    features TEXT, -- JSON array of feature strings
    stripe_price_id TEXT,
    paddle_price_id TEXT,
    lemon_variant_id TEXT,
    is_default BIGINT DEFAULT 0,
    enabled BIGINT DEFAULT 1,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
, quota_enforce_mode TEXT DEFAULT 'hard', quota_grace_pct DOUBLE PRECISION DEFAULT 0.05, trial_days BIGINT NOT NULL DEFAULT 0, meter_type TEXT NOT NULL DEFAULT 'requests', estimated_cost_per_req DOUBLE PRECISION NOT NULL DEFAULT 1.0);

CREATE TABLE "settings" (
    id TEXT PRIMARY KEY,
    key TEXT NOT NULL UNIQUE,
    value TEXT NOT NULL,
    encrypted BIGINT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE "usage_events" (
    id TEXT PRIMARY KEY,
    key_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    method TEXT NOT NULL,
    path TEXT NOT NULL,
    status_code BIGINT NOT NULL,
    latency_ms BIGINT NOT NULL,
    request_bytes BIGINT NOT NULL DEFAULT 0,
    response_bytes BIGINT NOT NULL DEFAULT 0,
    cost_multiplier DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    ip_address TEXT,
    user_agent TEXT,
    timestamp TIMESTAMPTZ NOT NULL
, event_type TEXT NOT NULL DEFAULT '', resource_id TEXT NOT NULL DEFAULT '', resource_type TEXT NOT NULL DEFAULT '', quantity DOUBLE PRECISION NOT NULL DEFAULT 1.0, source TEXT NOT NULL DEFAULT 'proxy', source_name TEXT NOT NULL DEFAULT '', metadata TEXT);

CREATE TABLE subscriptions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    plan_id TEXT NOT NULL,
    provider TEXT NOT NULL DEFAULT 'local',
    provider_id TEXT,
    provider_item_id TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    current_period_start TIMESTAMPTZ NOT NULL,
    current_period_end TIMESTAMPTZ NOT NULL,
    cancel_at_period_end BIGINT NOT NULL DEFAULT 0,
    cancelled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE invoices (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    provider TEXT,
    provider_id TEXT,
    period_start TIMESTAMPTZ NOT NULL,
    period_end TIMESTAMPTZ NOT NULL,
    items TEXT NOT NULL DEFAULT '[]',
    subtotal BIGINT NOT NULL DEFAULT 0,
    tax BIGINT NOT NULL DEFAULT 0,
    total BIGINT NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'USD',
    status TEXT NOT NULL DEFAULT 'draft',
    due_date TIMESTAMPTZ,
    paid_at TIMESTAMPTZ,
    invoice_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE entitlements (
    id TEXT PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    display_name TEXT,
    description TEXT,
    category TEXT DEFAULT 'feature',
    value_type TEXT DEFAULT 'boolean',
    default_value TEXT DEFAULT 'true',
    header_name TEXT,
    enabled BIGINT DEFAULT 1,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE plan_entitlements (
    id TEXT PRIMARY KEY,
    plan_id TEXT NOT NULL,
    entitlement_id TEXT NOT NULL,
    value TEXT,
    notes TEXT,
    enabled BIGINT DEFAULT 1,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(plan_id, entitlement_id)
);

CREATE TABLE oauth_states (
    state TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    redirect_uri TEXT,
    code_verifier TEXT,
    nonce TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE certificates (
    id TEXT PRIMARY KEY,
    domain TEXT NOT NULL,
    cert_pem BYTEA NOT NULL,
    chain_pem BYTEA,
    key_pem BYTEA NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    issuer TEXT,
    serial_number TEXT,
    acme_account_url TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    revoked_at TIMESTAMPTZ,
    revoke_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE acme_cache (
    key TEXT PRIMARY KEY,
    data BYTEA NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE quota_state (
    user_id TEXT NOT NULL,
    period_start TIMESTAMPTZ NOT NULL,
    request_count BIGINT NOT NULL DEFAULT 0,
    compute_units DOUBLE PRECISION NOT NULL DEFAULT 0,
    bytes_used BIGINT NOT NULL DEFAULT 0,
    last_updated TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (user_id, period_start)
);

CREATE TABLE routes (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',

    -- Matching criteria
    path_pattern TEXT NOT NULL,              -- /api/v1/*, /users/{id}, regex pattern
    match_type TEXT NOT NULL DEFAULT 'prefix', -- exact, prefix, regex
    methods TEXT,                            -- JSON array: ["GET", "POST"], null = all
    headers TEXT,                            -- JSON array of HeaderMatch objects

    -- Target configuration
    upstream_id TEXT NOT NULL REFERENCES upstreams(id) ON DELETE RESTRICT,
    path_rewrite TEXT,                       -- Expr expression for path rewriting
    method_override TEXT,                    -- Override request method

    -- Transformations (JSON)
    request_transform TEXT,                  -- JSON Transform object
    response_transform TEXT,                 -- JSON Transform object

    -- Metering
    metering_expr TEXT NOT NULL DEFAULT '1', -- Expr to extract usage value
    metering_mode TEXT NOT NULL DEFAULT 'request', -- request, response_field, bytes, custom

    -- Protocol behavior
    protocol TEXT NOT NULL DEFAULT 'http',   -- http, http_stream, sse, websocket

    -- Metadata
    priority BIGINT NOT NULL DEFAULT 0,     -- Higher = evaluated first
    enabled BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
, metering_unit TEXT NOT NULL DEFAULT 'requests', example_request TEXT NOT NULL DEFAULT '', example_response TEXT NOT NULL DEFAULT '', host_pattern TEXT NOT NULL DEFAULT '', host_match_type TEXT NOT NULL DEFAULT '', auth_required BIGINT NOT NULL DEFAULT 1);

CREATE TABLE "auth_tokens" (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    token_type TEXT NOT NULL,
    token_hash BYTEA NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE "user_sessions" (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    ip_address TEXT,
    user_agent TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE webhooks (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    url TEXT NOT NULL,
    secret TEXT NOT NULL,
    events TEXT NOT NULL DEFAULT '[]',  -- JSON array of event types
    retry_count BIGINT NOT NULL DEFAULT 3,
    timeout_ms BIGINT NOT NULL DEFAULT 30000,
    enabled BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE admin_invites (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL,
    token_hash BYTEA NOT NULL UNIQUE,
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    FOREIGN KEY (created_by) REFERENCES users(id)
);

CREATE TABLE groups (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    description TEXT,
    owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    plan_id TEXT REFERENCES plans(id),
    billing_email TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE oauth_identities (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    provider_user_id TEXT NOT NULL,
    email TEXT,
    name TEXT,
    avatar_url TEXT,
    access_token TEXT,
    refresh_token TEXT,
    token_expires_at TIMESTAMPTZ,
    raw_data TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(provider, provider_user_id)
);

CREATE TABLE "api_keys" (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    hash BYTEA NOT NULL,
    prefix TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    scopes TEXT,
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
, group_id TEXT REFERENCES groups(id), created_by TEXT REFERENCES users(id), quota_bypass BOOLEAN NOT NULL DEFAULT FALSE);

CREATE TABLE webhook_deliveries (
    id TEXT PRIMARY KEY,
    webhook_id TEXT NOT NULL,
    event_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    payload TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    attempt BIGINT NOT NULL DEFAULT 1,
    max_attempts BIGINT NOT NULL DEFAULT 3,
    status_code BIGINT,
    response_body TEXT,
    error TEXT,
    duration_ms BIGINT,
    next_retry TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (webhook_id) REFERENCES webhooks(id) ON DELETE CASCADE
);

CREATE TABLE group_members (
    id TEXT PRIMARY KEY,
    group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'member',
    invited_by TEXT REFERENCES users(id),
    invited_at TIMESTAMPTZ,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(group_id, user_id)
);

CREATE TABLE group_invites (
    id TEXT PRIMARY KEY,
    group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'member',
    invited_by TEXT NOT NULL REFERENCES users(id),
    token TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE "rate_limit_state" (
    key_id TEXT PRIMARY KEY REFERENCES api_keys(id) ON DELETE CASCADE,
    count BIGINT NOT NULL DEFAULT 0,
    window_end TIMESTAMPTZ NOT NULL,
    burst_used BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_stripe_id ON users(stripe_id);
CREATE INDEX idx_users_status ON users(status);
CREATE INDEX idx_usage_summaries_user_period ON usage_summaries(user_id, period_start, period_end);
CREATE INDEX idx_upstreams_enabled ON upstreams(enabled);
CREATE INDEX idx_upstreams_name ON upstreams(name);
CREATE INDEX idx_routes_enabled_priority ON routes(enabled, priority DESC);
CREATE INDEX idx_routes_path_pattern ON routes(path_pattern);
CREATE INDEX idx_routes_upstream_id ON routes(upstream_id);
CREATE INDEX idx_routes_name ON routes(name);
CREATE INDEX idx_settings_key ON settings(key);
CREATE INDEX idx_auth_tokens_user_type ON auth_tokens(user_id, token_type);
CREATE INDEX idx_auth_tokens_expires ON auth_tokens(expires_at);
CREATE INDEX idx_auth_tokens_hash ON auth_tokens(token_hash);
CREATE INDEX idx_user_sessions_user ON user_sessions(user_id);
CREATE INDEX idx_user_sessions_expires ON user_sessions(expires_at);
CREATE INDEX idx_api_keys_prefix ON api_keys(prefix);
CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX idx_usage_events_user_id ON usage_events(user_id);
CREATE INDEX idx_usage_events_key_id ON usage_events(key_id);
CREATE INDEX idx_usage_events_timestamp ON usage_events(timestamp);
CREATE INDEX idx_usage_events_user_timestamp ON usage_events(user_id, timestamp);
CREATE INDEX idx_subscriptions_user_id ON subscriptions(user_id);
CREATE INDEX idx_subscriptions_status ON subscriptions(status);
CREATE INDEX idx_subscriptions_provider_id ON subscriptions(provider_id);
CREATE INDEX idx_invoices_user_id ON invoices(user_id);
CREATE INDEX idx_invoices_status ON invoices(status);
CREATE INDEX idx_invoices_created_at ON invoices(created_at);
CREATE INDEX idx_entitlements_name ON entitlements(name);
CREATE INDEX idx_entitlements_category ON entitlements(category);
CREATE INDEX idx_entitlements_enabled ON entitlements(enabled);
CREATE INDEX idx_plan_entitlements_plan ON plan_entitlements(plan_id);
CREATE INDEX idx_plan_entitlements_entitlement ON plan_entitlements(entitlement_id);
CREATE INDEX idx_plan_entitlements_enabled ON plan_entitlements(enabled);
CREATE INDEX idx_webhooks_user_id ON webhooks(user_id);
CREATE INDEX idx_webhooks_enabled ON webhooks(enabled);
CREATE INDEX idx_webhook_deliveries_webhook_id ON webhook_deliveries(webhook_id);
CREATE INDEX idx_webhook_deliveries_status ON webhook_deliveries(status);
CREATE INDEX idx_webhook_deliveries_next_retry ON webhook_deliveries(next_retry) WHERE status = 'retrying';
CREATE INDEX idx_webhook_deliveries_event_id ON webhook_deliveries(event_id);
CREATE INDEX idx_admin_invites_email ON admin_invites(email);
CREATE INDEX idx_admin_invites_expires_at ON admin_invites(expires_at);
CREATE INDEX idx_groups_owner ON groups(owner_id);
CREATE INDEX idx_groups_slug ON groups(slug);
CREATE INDEX idx_groups_status ON groups(status);
CREATE INDEX idx_group_members_group ON group_members(group_id);
CREATE INDEX idx_group_members_user ON group_members(user_id);
CREATE INDEX idx_group_members_role ON group_members(role);
CREATE INDEX idx_group_invites_token ON group_invites(token);
CREATE INDEX idx_group_invites_email ON group_invites(email);
CREATE INDEX idx_group_invites_group ON group_invites(group_id);
CREATE INDEX idx_group_invites_expires ON group_invites(expires_at);
CREATE INDEX idx_api_keys_group ON api_keys(group_id);
CREATE INDEX idx_oauth_identities_user ON oauth_identities(user_id);
CREATE INDEX idx_oauth_identities_provider ON oauth_identities(provider, provider_user_id);
CREATE INDEX idx_oauth_identities_email ON oauth_identities(email);
CREATE INDEX idx_oauth_states_expires ON oauth_states(expires_at);
CREATE INDEX idx_certificates_domain ON certificates(domain);
CREATE INDEX idx_certificates_expires ON certificates(expires_at);
CREATE INDEX idx_certificates_status ON certificates(status);
CREATE UNIQUE INDEX idx_certificates_domain_active
ON certificates(domain) WHERE status = 'active';
CREATE INDEX idx_routes_host_pattern ON routes(host_pattern) WHERE host_pattern != '';
CREATE INDEX idx_routes_auth_required ON routes(auth_required);
CREATE INDEX idx_api_keys_quota_bypass ON api_keys(quota_bypass) WHERE quota_bypass = TRUE;
CREATE INDEX idx_acme_cache_key ON acme_cache(key);
CREATE INDEX idx_quota_state_period ON quota_state(period_start);
CREATE INDEX idx_usage_events_event_type ON usage_events(event_type);
CREATE INDEX idx_usage_events_source ON usage_events(source);
