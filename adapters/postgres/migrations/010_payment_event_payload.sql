-- Bind retries to the verified normalized event, including transaction identity.
ALTER TABLE payment_events ADD COLUMN payload JSONB NOT NULL DEFAULT '{}'::jsonb;
