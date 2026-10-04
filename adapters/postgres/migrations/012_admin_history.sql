CREATE INDEX payment_orders_history ON payment_orders(created_at DESC,id DESC);
CREATE INDEX payment_orders_account_history ON payment_orders(user_id,created_at DESC,id DESC);
CREATE INDEX users_customer_history ON users(created_at DESC,id DESC) WHERE role='user';
CREATE INDEX admin_audit_history ON admin_audit(created_at DESC,id DESC);
