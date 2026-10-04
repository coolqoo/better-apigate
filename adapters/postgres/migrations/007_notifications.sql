CREATE TABLE notification_deliveries (
 id TEXT PRIMARY KEY, event_id TEXT NOT NULL REFERENCES billing_outbox(id),
 channel TEXT NOT NULL CHECK(channel IN ('webhook','email')),
 target TEXT NOT NULL, secret TEXT NOT NULL DEFAULT '', payload JSONB NOT NULL,
 attempts INTEGER NOT NULL DEFAULT 0, next_attempt TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 delivered_at TIMESTAMPTZ, last_error TEXT NOT NULL DEFAULT '',
 UNIQUE(event_id,channel,target)
);
CREATE INDEX notification_pending ON notification_deliveries(next_attempt) WHERE delivered_at IS NULL;
