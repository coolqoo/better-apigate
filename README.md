# APIGate v2

A Go API gateway with prepaid billing, PostgreSQL, Redis and one React/shadcn UI for customers, administrators, onboarding and API documentation.

Customers use included plan units first, then a prepaid USD wallet. Fixed route costs are reserved before forwarding. Successful upstream 2xx–4xx responses charge; verified 5xx and transport failures release. PostgreSQL transactions prevent concurrent requests, renewals and callbacks from spending the same funds twice. Crash holds remain reserved for audited reconciliation.

Stripe, Paddle, Lemon Squeezy and current GMPay EPUSDT fund the same wallet through one-time checkouts. Internal 30-day plans renew from available wallet funds, with explicit reactivation after a failed renewal. Money uses integer USD millionths and decimal strings in the API.

```sh
scripts/init-env.sh
# Configure the public HTTPS origin in .env for a real deployment.
docker compose up --build -d --wait
```

Open `/setup` with the deployment setup token, then configure pricing, upstreams, routes and providers at `/admin`. Customers use `/portal`; public documentation is at `/docs`. Browser sessions use HttpOnly cookies and CSRF protection. API keys use indexed HMAC-SHA256 digests with a deployment-managed secret.

[Deployment, recovery, provider configuration and backup/restore](docs/deployment-v2.md) · [Generated OpenAPI](docs/openapi-v2.json)

Go 1.27.1 and Node 22.23.2 are pinned across local manifests, Docker and CI. Build with `npm ci --prefix webui && make all`. The only embedded frontend bundle is `core/channel/http/webui/dist`; YAML remains the source for configuration fields, validation and action metadata. Billing mutations use typed transactional handlers and cannot pass through generic CRUD.

This upgrade targets a fresh deployment. Production migration and legacy credential compatibility are outside its scope. A release requires passing race/coverage checks, UI flows, provider sandbox smoke checks, reproducible installation, and both ten-minute 1,000 accepted RPS load scenarios with ledger reconciliation. Test tooling and reports are kept locally and excluded from this implementation delivery. Release publication is gated by the `APIGATE_RELEASE_VERIFIED` repository variable after separate verification; a successful build alone does not establish release readiness.
