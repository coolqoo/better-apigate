CREATE TABLE payment_inbox (
 provider TEXT NOT NULL, event_id TEXT NOT NULL,
 order_id TEXT NOT NULL REFERENCES payment_orders(id), payload JSONB NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, processed_at TIMESTAMPTZ,
 PRIMARY KEY(provider,event_id)
);
CREATE INDEX payment_inbox_pending ON payment_inbox(created_at) WHERE processed_at IS NULL;
