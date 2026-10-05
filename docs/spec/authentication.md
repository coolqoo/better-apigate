# Authentication specification

**Status:** Authoritative for better-apigate fresh deployments

Browser customers and administrators use the `/api/v1/auth/*` endpoints and opaque PostgreSQL-backed sessions. The `apigate_session` cookie is HttpOnly, SameSite=Lax, and Secure when the configured public origin is HTTPS. Only a SHA-256 digest is stored in PostgreSQL. Session role and account status are checked against the current user row on each request. Logout deletes the session; password changes/reset invalidate prior sessions.

Every unsafe authenticated browser request must supply `X-CSRF-Token` from `/api/v1/session`. Unsafe requests with a supplied foreign Origin are rejected. JSON bodies use JSON:API `data.attributes`; unsupported content types and unknown fields are rejected. Ownership is enforced on keys, wallets and payment orders; administrative actions require the current database role to be admin. Browser auth tokens never live in localStorage.

The gateway accepts API keys in `X-API-Key` or Bearer transport. Indexed HMAC-SHA256 digests use the shared deployment `APIGATE_API_KEY_SECRET`, at least 32 characters. Credential lookups may be cached in Redis, but PostgreSQL admission verifies revocation, expiry and account suspension before authorizing funds. Legacy bcrypt keys and browser JWT credentials do not authorize prepaid requests.

An internal signed transport bridges verified browser sessions to curated YAML configuration handlers. That token is generated server-side, never returned to the browser, and cannot grant customer access to generic financial CRUD. Wallet, ledger, reservation, term, payment and outbox mutations require typed transactional actions.

Missing/invalid credentials return `401`, ownership/admin/CSRF failures return `403` or an ownership-safe `404`, suspended/frozen accounts return `403`, unfunded requests return `402`, account rate limits return `429`, and unavailable enforcement dependencies return `503`. Explicit public routes remain free and separate from paid admission.
