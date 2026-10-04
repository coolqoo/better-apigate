ALTER TABLE plans ADD COLUMN price_micros BIGINT NOT NULL DEFAULT 0 CHECK(price_micros>=0);
ALTER TABLE plans ADD COLUMN unit_price_micros BIGINT NOT NULL DEFAULT 0 CHECK(unit_price_micros>=0);
ALTER TABLE plans ADD COLUMN included_units BIGINT NOT NULL DEFAULT 0 CHECK(included_units>=0);
ALTER TABLE plans ADD COLUMN term_days BIGINT NOT NULL DEFAULT 30 CHECK(term_days BETWEEN 1 AND 365);
ALTER TABLE routes ADD COLUMN unit_cost BIGINT NOT NULL DEFAULT 1 CHECK(unit_cost>0);
CREATE UNIQUE INDEX api_keys_digest ON api_keys(hash);
CREATE TABLE wallets (
 user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE RESTRICT,
 balance_micros BIGINT NOT NULL DEFAULT 0 CHECK(balance_micros>=0),
 reserved_micros BIGINT NOT NULL DEFAULT 0 CHECK(reserved_micros>=0 AND reserved_micros<=balance_micros),
 shortfall_micros BIGINT NOT NULL DEFAULT 0 CHECK(shortfall_micros>=0),
 frozen BOOLEAN NOT NULL DEFAULT FALSE,
 auto_renew BOOLEAN NOT NULL DEFAULT TRUE,
 next_plan_id TEXT NOT NULL DEFAULT '',
 updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE plan_terms (
 id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES wallets(user_id),
 plan_id TEXT NOT NULL REFERENCES plans(id), plan_name TEXT NOT NULL,
 price_micros BIGINT NOT NULL, unit_price_micros BIGINT NOT NULL,
 included_units BIGINT NOT NULL CHECK(included_units>=0),
 used_units BIGINT NOT NULL DEFAULT 0 CHECK(used_units>=0),
 reserved_units BIGINT NOT NULL DEFAULT 0 CHECK(reserved_units>=0),
 rate_limit_per_minute BIGINT NOT NULL, renewal_attempted BOOLEAN NOT NULL DEFAULT FALSE,
 starts_at TIMESTAMPTZ NOT NULL, ends_at TIMESTAMPTZ NOT NULL,
 CHECK(used_units+reserved_units<=included_units), CHECK(ends_at>starts_at),
 UNIQUE(user_id,starts_at)
);
CREATE INDEX plan_terms_user_expiry ON plan_terms(user_id,ends_at DESC);
CREATE TABLE usage_reservations (
 id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES wallets(user_id),
 key_id TEXT NOT NULL, route_id TEXT NOT NULL DEFAULT '', term_id TEXT REFERENCES plan_terms(id),
 units BIGINT NOT NULL CHECK(units>0), quota_units BIGINT NOT NULL CHECK(quota_units>=0),
 amount_micros BIGINT NOT NULL CHECK(amount_micros>=0), unit_price_micros BIGINT NOT NULL CHECK(unit_price_micros>=0),
 state TEXT NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','charged','released')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, settled_at TIMESTAMPTZ,
 CHECK(quota_units<=units)
);
CREATE INDEX reservations_pending ON usage_reservations(created_at) WHERE state='pending';
CREATE TABLE wallet_ledger (
 id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES wallets(user_id), kind TEXT NOT NULL,
 amount_micros BIGINT NOT NULL, balance_micros BIGINT NOT NULL,
 reference TEXT NOT NULL UNIQUE, description TEXT NOT NULL DEFAULT '', actor_id TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX wallet_ledger_user ON wallet_ledger(user_id,created_at DESC);
CREATE FUNCTION protect_wallet_ledger() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'wallet ledger is append-only'; END; $$;
CREATE TRIGGER wallet_ledger_immutable BEFORE UPDATE OR DELETE ON wallet_ledger FOR EACH ROW EXECUTE FUNCTION protect_wallet_ledger();
CREATE TABLE payment_orders (
 id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES wallets(user_id), provider TEXT NOT NULL,
 amount_micros BIGINT NOT NULL CHECK(amount_micros>0), currency TEXT NOT NULL DEFAULT 'USD',
 state TEXT NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','paid','expired','reversed','failed')),
 provider_id TEXT, checkout_url TEXT NOT NULL DEFAULT '', crypto_amount TEXT NOT NULL DEFAULT '', crypto_token TEXT NOT NULL DEFAULT '',
 idempotency_key TEXT NOT NULL, expires_at TIMESTAMPTZ, reversed_micros BIGINT NOT NULL DEFAULT 0,
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 UNIQUE(user_id,idempotency_key), UNIQUE(provider,provider_id)
);
CREATE TABLE payment_events (
 provider TEXT NOT NULL, event_id TEXT NOT NULL, order_id TEXT NOT NULL REFERENCES payment_orders(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY(provider,event_id)
);
CREATE TABLE billing_outbox (
 id TEXT PRIMARY KEY, event_type TEXT NOT NULL, payload JSONB NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, processed_at TIMESTAMPTZ
);
CREATE INDEX billing_outbox_pending ON billing_outbox(created_at) WHERE processed_at IS NULL;
INSERT INTO plans(id,name,description,price_micros,unit_price_micros,included_units,rate_limit_per_minute,is_default,enabled,requests_per_month,quota_grace_pct)
VALUES('paygo','Pay as you go','Use your prepaid wallet. No subscription required.',0,1000,0,600,1,1,0,0);
