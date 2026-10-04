ALTER TABLE payment_orders ADD COLUMN provider_transaction_id TEXT;
CREATE UNIQUE INDEX payment_orders_transaction_identity ON payment_orders(provider,provider_transaction_id) WHERE provider_transaction_id IS NOT NULL;
