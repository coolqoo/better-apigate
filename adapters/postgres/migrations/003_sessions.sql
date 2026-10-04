ALTER TABLE users ALTER COLUMN plan_id SET DEFAULT 'paygo';
ALTER TABLE users ADD COLUMN email_verified BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE user_sessions ADD COLUMN token_digest BYTEA UNIQUE;
ALTER TABLE user_sessions ADD COLUMN csrf_token TEXT NOT NULL DEFAULT '';
ALTER TABLE payment_orders ADD COLUMN provider_checkout_id TEXT NOT NULL DEFAULT '';
CREATE TABLE auth_challenges (
 digest BYTEA PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id),
 purpose TEXT NOT NULL CHECK(purpose IN ('verify','reset')),
 expires_at TIMESTAMPTZ NOT NULL, used_at TIMESTAMPTZ
);
CREATE TABLE admin_audit (
 id TEXT PRIMARY KEY, actor_id TEXT NOT NULL REFERENCES users(id),
 action TEXT NOT NULL, target_id TEXT NOT NULL, reason TEXT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
