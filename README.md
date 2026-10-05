# better-apigate

A self-hosted API gateway with prepaid billing. Connect your upstream API, set route prices and plans, and let customers fund their wallets and manage API keys through a React/shadcn portal.

[Docker Hub image](https://hub.docker.com/r/coolqoo/better-apigate/tags?name=1.0.0) · [v1.0.0 release](https://github.com/coolqoo/better-apigate/releases/tag/v1.0.0) · [User guide](docs/USER_GUIDE.md) · [Deployment guide](docs/deployment-v2.md)

## Start with Docker

Use Docker Compose to start **better-apigate, PostgreSQL and Redis** together. PostgreSQL stores accounts, configuration and billing records; Redis handles shared rate limits and caches. The old single-container SQLite installation does not apply to this version.

Requires Docker Engine or Docker Desktop with the Compose plugin, Git and OpenSSL. Published images support Linux amd64 and arm64, including Apple Silicon through Docker Desktop. The default PostgreSQL configuration uses 1 GiB of shared buffers; allow at least 4 GiB for PostgreSQL plus memory for the gateway and Redis.

```sh
git clone https://github.com/coolqoo/better-apigate.git
cd better-apigate

# Create .env with fresh database passwords and deployment secrets.
sh scripts/init-env.sh

# Use the published image. No Go, Node.js or image build is required.
printf '\nAPIGATE_IMAGE=coolqoo/better-apigate:1.0.0\n' >> .env

docker compose pull
docker compose up -d --no-build --wait
```

These commands run the gateway at **http://localhost:8080**. If installing on a remote server, first edit `APIGATE_PUBLIC_URL` in `.env` to the origin you will open in your browser, such as `http://YOUR_SERVER_IP:8080`. For a public deployment, use your HTTPS domain and a TLS reverse proxy. The public URL must match the browser origin for session and CSRF checks.

Configuration variables use the `APIGATE_` prefix. Keep `.env`: it contains your database passwords, initial setup token and API-key signing secret.

## First-time setup

1. Open **http://localhost:8080/setup** (or `/setup` on your configured public origin).
2. Copy the value of `APIGATE_SETUP_TOKEN` from `.env` into the setup form, then create the administrator account. To display it locally:

   ```sh
   grep '^APIGATE_SETUP_TOKEN=' .env
   ```

3. Open **Administration → Plans & Pricing**. Configure the base pay-as-you-go price and rate limit, and add any paid plans.
4. In **API Configuration**, add your upstream API and routes. Give paid routes a fixed unit cost; mark a route public only when it is free.
5. In **Settings**, configure top-up amounts, enable payment providers and add email delivery for verification/password reset.
6. Customers sign up at **`/portal`**, add funds, optionally buy a plan, create an API key and follow the request examples at **`/docs`**.

For an initial trial without configuring a payment provider, create a customer account and use **Customers → Manage** to record a wallet credit with a reason. The credit appears in the ledger and lets that account use paid routes.

| Address | Purpose |
| --- | --- |
| `/setup` | Create the initial administrator |
| `/admin` | Configure APIs, pricing, customers, payments and settings |
| `/portal` | Customer wallet, plans, API keys, usage and account |
| `/docs` | Documentation for configured API routes |
| `/ready` | Gateway readiness, including PostgreSQL and Redis |

## What it does

| Feature | Behavior |
| --- | --- |
| Prepaid wallet | Reserve funds before forwarding; reject unfunded requests with HTTP 402 |
| Included usage | Use a plan's included units first, then draw from the wallet |
| Fixed route pricing | Set an integer unit cost for each paid route |
| Plans | 30-day terms, no rollover or proration, optional wallet-funded renewal |
| Payment providers | Stripe, Paddle, Lemon Squeezy and GMPay EPUSDT one-time top-ups |
| Account rate limits | Multiple keys share the same account limit; HTTP 429 when exhausted |
| Customer portal | Keys, usage, wallet history, payments, plans and account management |
| Administration | Curated configuration, customer controls and audited reconciliation |
| Frontend | Responsive shadcn UI with light/dark themes and browser session cookies |

Upstream 2xx–4xx responses charge the reserved usage. Verified upstream 5xx and transport failures release it. Crash holds remain reserved until reconciliation; a customer disconnect alone does not release funds. Money is stored exactly as integer USD millionths and exposed as decimal strings.

Plan purchases enable auto-renew with disclosure in the portal. Renewal uses available wallet funds; if the full plan price is unavailable, the account falls back to pay-as-you-go. A later top-up does not silently restart the subscription.

## Everyday Docker commands

```sh
# Check service health.
docker compose ps

# Follow gateway logs.
docker compose logs -f gateway

# Stop the stack; named database volumes are retained.
docker compose down

# Start it again using the published image.
docker compose up -d --no-build --wait
```

Do not use `docker compose down -v` unless you intend to delete the database volumes. Back up PostgreSQL and keep a secure copy of `.env`; see [backup and restore](docs/deployment-v2.md#backup-and-restore).

To use a future release, change the version in `APIGATE_IMAGE` in `.env`, then run `docker compose pull && docker compose up -d --no-build --wait`. Docker Hub tags contain only full versions such as `1.0.0`; use an explicit version when deploying.

## Build from source

Source builds use Go 1.27.1 and Node 22.23.2. To build the frontend and native executable:

```sh
npm ci --prefix webui
make all
./bin/better-apigate version
```

To build a Docker image from your checkout instead of pulling the release, run `docker compose up --build -d --wait` after creating `.env`.

The project targets a fresh PostgreSQL/Redis deployment; migration from the original SQLite application is not included. Developer contracts, provider setup, recovery procedures and release validation details live in the [deployment guide](docs/deployment-v2.md), [user guide](docs/USER_GUIDE.md) and [generated OpenAPI](docs/openapi-v2.json).
