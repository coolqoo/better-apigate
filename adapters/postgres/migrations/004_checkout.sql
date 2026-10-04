ALTER TABLE payment_orders ADD COLUMN checkout_attempted BOOLEAN NOT NULL DEFAULT FALSE;
CREATE UNIQUE INDEX plans_single_base ON plans(is_default) WHERE is_default=1;
