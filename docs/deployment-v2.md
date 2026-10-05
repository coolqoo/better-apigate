# better-apigate deployment

This is a fresh-deployment release. Docker Compose runs the gateway against your existing PostgreSQL and Redis services using connection URLs. PostgreSQL owns users, configuration, sessions, terms, reservations, payment orders, the immutable wallet ledger and durable outboxes. Redis owns shared account limits and short-lived credential/configuration caches. All gateway instances must use the same PostgreSQL database, Redis deployment and API-key secret.

## Clean installation

Use the published Docker Hub image from release `v1.0.0`. Requires Docker Engine/Desktop with the Compose plugin, Git, OpenSSL for generating secrets, and reachable PostgreSQL and Redis services. Go and Node.js are needed only for source builds.

```sh
git clone https://github.com/coolqoo/better-apigate.git
cd better-apigate
sh scripts/init-env.sh
# Edit .env and supply your APIGATE_DATABASE_DSN and APIGATE_REDIS_URL.
# For a server, set APIGATE_PUBLIC_URL in .env to your browser's public origin.
docker compose pull
docker compose up -d --no-build --wait
curl --fail http://localhost:8080/ready
```

`scripts/init-env.sh` creates gateway secrets and selects the published image, leaving the two service URLs for you to fill in. It preserves an existing `.env`. Set these variables before running Compose:

```dotenv
APIGATE_DATABASE_DSN='postgresql://USER:PASSWORD@POSTGRES_HOST:5432/DATABASE?sslmode=require'
APIGATE_REDIS_URL='rediss://default:PASSWORD@REDIS_HOST:6379/0'
```

Use your providers' complete URLs, with their required ports and TLS parameters. PostgreSQL accepts `postgres://` or `postgresql://`; Redis accepts `redis://` or TLS-enabled `rediss://`. Credentials are part of the URLs; separate `POSTGRES_PASSWORD` and `REDIS_PASSWORD` fields are not used. Percent-encode reserved characters in credentials and single-quote the values in `.env` so Compose preserves literal dollar signs. Use a dedicated PostgreSQL database whose user can run schema migrations, and a Redis deployment configured to retain rate-limit keys without eviction.

URLs must be reachable from inside the gateway container. For PostgreSQL or Redis running on the Docker host, use `host.docker.internal` instead of `localhost` and allow connections from Docker's network. For remote services, use their reachable hostnames. This Compose file does not start PostgreSQL or Redis servers or manage their persistence and credentials. When switching an existing checkout to this setup, set the two URLs and remove the old separate password fields from `.env`.

Open `/setup`, enter the deployment setup token from `.env`, and create the initial administrator. Configure the base pay-as-you-go price, route unit costs, plans, enabled payment providers and email delivery under `/admin`. New customers sign up, fund their wallet, optionally purchase a plan and create an API key in `/portal`. Public API documentation lives at `/docs`.

The setup token and API-key secret are deployment secrets. Keep `.env` private and backed up securely. API-key secret rotation invalidates existing keys; issue replacements deliberately. Browser sessions use HttpOnly cookies with CSRF protection. A TLS reverse proxy must retain the public origin; `APIGATE_PUBLIC_URL` controls secure cookies, callback URLs and origin validation. Do not put auth tokens in browser storage.

PostgreSQL migrations run under an advisory lock at startup. Each process has one shared pool, defaulting to 32 open and 8 idle connections. Set `APIGATE_DB_MAX_CONNS` between 2 and 500; budget the sum across instances plus administrative/backup connections below your database service's connection limit. Readiness checks both PostgreSQL and Redis; a dependency outage prevents paid forwarding. Shutdown drains HTTP requests, stops workers and closes the pool. Configure database capacity, durable storage and Redis persistence through your service providers or your own database deployments.

## Published images and native binaries

The published image is [`coolqoo/better-apigate:1.0.0`](https://hub.docker.com/r/coolqoo/better-apigate/tags?name=1.0.0), with Linux amd64 and arm64 variants. Set `APIGATE_IMAGE=coolqoo/better-apigate:1.0.0` in `.env`, then run `docker compose pull && docker compose up -d --no-build --wait`. Published images embed the frontend and PostgreSQL migrations and run as an unprivileged user; no SQLite files or external migration directory are needed. For a source image build, use `docker compose up --build -d --wait`.

`scripts/install.sh` downloads from `coolqoo/better-apigate`, verifies `checksums.txt` before extracting, and installs `better-apigate` (`better-apigate.exe` on Windows). `VERSION=v1.0.0` selects the first release; `INSTALL_DIR` selects the destination. Native deployments still require PostgreSQL and Redis and the same `APIGATE_DATABASE_DSN`, `APIGATE_REDIS_URL`, `APIGATE_API_KEY_SECRET`, `APIGATE_SETUP_TOKEN` and `APIGATE_PUBLIC_URL` environment values before `better-apigate serve`. Use URLs reachable from the native process; `localhost` is appropriate when the native process and database are on the same host.

For a second gateway, run `docker compose --profile replica pull && docker compose --profile replica up -d --no-build --wait` and put ports 8080 and 8082 behind the same public HTTPS origin. Both instances share database state, secrets and account limits. Include both pools in the database connection budget. A provider callback may reach either instance.

## Accounting and recovery

USD amounts are integer millionths, returned as six-place decimal strings. One request consumes its route's integer unit cost, using included term units first and available wallet funds for the rest. The reservation commits before upstream dispatch. Upstream 2xx–4xx responses charge; verified upstream 5xx/transport failures release. A customer disconnect alone does not release funds. Public routes explicitly bypass paid admission.

Plans have 30-day terms with no rollover or proration. Purchases enable auto-renew, which is disclosed in the portal. A plan change applies at the next term. Renewals spend available funds atomically with term creation. Insufficient funds, cancellation or an unavailable plan returns the account to the configured base plan. Later top-ups do not automatically reactivate an expired subscription.

A crash can leave a pending reservation. Holds never expire automatically. In **Payments → Unresolved requests**, inspect upstream evidence and make an audited charge/release decision. **Payments → Audit trail** shows the administrator, record, decision and reason. Both views have page controls, so older records remain reachable. Never directly edit a wallet or ledger row. Analytics and notification outboxes survive restarts; consumers acknowledge jobs transactionally and deduplicate by stable event IDs. External webhook delivery is at-least-once, so recipients must deduplicate `X-Event-ID`.

Refunds and reversals remove available funds. If the credit was spent, a shortfall is recorded and paid access freezes. Top-ups and credits cover that shortfall before increasing spendable funds. An administrator explicitly unfreezes an account after reconciliation. For a provider without a signed reversal callback, record the verified reversal with its order, amount and evidence using the audited order-reversal action; initiating the external refund is a separate provider operation.

## Payment providers

Enable multiple providers in Settings. Every checkout is a one-time top-up; recurring plan purchases happen internally from the wallet. Configure webhook secrets and public callback URLs `/api/v1/payment-webhooks/{provider}`. A redirect never credits a wallet. An uncertain checkout-creation response remains pending for callback/reconciliation; retrying the same idempotency key does not create a second checkout.

- Stripe: one-time Checkout, webhook signing secret; completed/async-success checkout and charge refund/dispute events.
- Paddle: a top-up product, non-recurring inline price, webhook secret, client token and public merchant checkout origin. Use the sandbox setting for sandbox credentials. Approved adjustment IDs provide stable reversal identity.
- Lemon Squeezy: store ID, webhook secret and a top-up variant whose current Price category is `one_time`. Subscription variants are rejected before checkout. Configure order-created and order-refunded events.
- EPUSDT: GMPay URL, merchant PID, API key and network. The adapter is pinned to [revision 58141cd](https://github.com/GMWalletApp/epusdt/blob/58141cd148408bbe05b0cd45716d6110f3952007/wiki/API.md). USD wallet credit and the provider's USDT quote are displayed separately. Signed successful callbacks must include the trade and blockchain transaction identity. The pinned API has no signed refund contract; use verified evidence and the audited order-reversal action.

[Lemon Squeezy Price documentation](https://docs.lemonsqueezy.com/api/prices/the-price-object) defines one-time products. Provider sandbox smoke checks are a release gate; local signature fixtures do not establish that merchant accounts are configured correctly.

## Backup and restore

```sh
scripts/backup.sh /secure/backups/better-apigate.dump
# Restore into an isolated deployment first and verify the ledger.
scripts/restore.sh /secure/backups/better-apigate.dump
```

These scripts start an ephemeral PostgreSQL client container under the optional `maintenance` profile and connect using `APIGATE_DATABASE_DSN` from `.env`. They do not start a database server or require PostgreSQL tools on the Docker host. The default client image is `postgres:17.10-alpine`; set `APIGATE_POSTGRES_CLIENT_IMAGE` in `.env` to match your database server's major version. A `pg_dump` client cannot back up a newer server major version; see [PostgreSQL backup documentation](https://www.postgresql.org/docs/17/app-pgdump.html).

Backups use PostgreSQL custom format and restrictive permissions. Restore stops both gateways in this Compose project, restores transactionally and leaves gateway restart to the operator. Stop any other gateway instances using the same database before restoring. Retain `.env` and the API-key secret separately. Configure Redis persistence through your Redis service for limiter/cache continuity; PostgreSQL backups are authoritative for funds. Do not restore an old database over a live payment processor without reconciling callbacks and orders that occurred after the backup.

## Build and release verification

```sh
npm ci --prefix webui
make contracts webui build
npm run typecheck --prefix webui
```

Added test code, load tooling and validation reports are local files excluded from Git. CI builds the frontend, checks generated contracts and builds the Go targets without running that local test tooling.

Before declaring a production release accepted, separately complete accounting, payment signatures/reversals, renewal, recovery/security, browser flows, provider sandbox smoke checks, a clean Docker installation, Go race/coverage checks and sustained-load reconciliation. The performance target requires at least 1,000 accepted requests/second for ten minutes with many accounts and one concurrent account across two instances, with exact wallet/ledger agreement and no unresolved settlement backlog. Record the gateway revision, hardware, p95/p99 overhead and database contention for both scenarios. Image publication does not establish completion of these checks.

## CI/CD and Docker Hub

Configure these GitHub repository secrets under **Settings → Secrets and variables → Actions**:

- `DOCKERHUB_USERNAME`: the Docker Hub account that owns the `better-apigate` image repository.
- `DOCKERHUB_TOKEN`: a Docker Hub access token with write access to that repository.

No other release variable or registry credential is required. Pull requests, branch pushes and branch CI dispatches only validate and build. Semantic version tag pushes publish a single Linux amd64/arm64 manifest to `<DOCKERHUB_USERNAME>/better-apigate:<version>` after the build succeeds. The pipeline checks the published manifest by digest and reports it in the job summary. GitHub releases reuse the same binaries, with the same embedded frontend, and attach SHA256 checksums for installation.

| Trigger | Docker Hub tags |
| --- | --- |
| Branch push or pull request | None |
| Push `v1.0.0` | `1.0.0` |
| Push `v1.1.0` | `1.1.0` |
| Push `v1.1.0-rc.1` | `1.1.0-rc.1` |

Push a semantic version tag when ready to publish a GitHub release. Docker tags contain only the complete version without the leading `v`; `latest`, branch names, SHAs and shortened versions are never published. Prerelease tags create GitHub prereleases. The Actions workflow owns Docker publishing; `.goreleaser.yaml` remains available for native archive builds. For a local image publish, run `docker login` and `DOCKERHUB_USERNAME=your-namespace make docker-publish DOCKER_TAG=1.0.0`.

## Upstream credentials and monitoring

Configure upstream authentication with an environment reference such as `${UPSTREAM_API_TOKEN}` in the API Configuration screen. Add `UPSTREAM_API_TOKEN` to the private `.env` or supply it through your deployment secret manager; Compose passes `.env` to both gateway instances. Recreate the gateway containers after changing deployment secrets. Only the reference is stored in PostgreSQL; the credential value is not returned to the browser. For Basic authentication, the environment value is the base64-encoded `username:password` string.

When metrics are enabled in gateway settings, `/metrics` requires a current admin session or `Authorization: Bearer <APIGATE_METRICS_TOKEN>`. Configure a separate deployment-managed token with at least 32 characters for Prometheus scrapes. Avoid putting it in a URL.
