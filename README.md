# better-apigate

A self-hosted API gateway with prepaid wallets, usage plans and a React/shadcn customer portal. Supports Stripe, Paddle, Lemon Squeezy and EPUSDT payments.

[Docker Hub](https://hub.docker.com/r/coolqoo/better-apigate) · [Releases](https://github.com/coolqoo/better-apigate/releases) · [User guide](docs/USER_GUIDE.md)

## Docker installation

Requires Docker Compose and existing PostgreSQL and Redis services reachable from the container. The image supports amd64 and arm64.

```sh
git clone https://github.com/coolqoo/better-apigate.git
cd better-apigate
cp .env.example .env
```

Edit `.env`:

- Set `APIGATE_DATABASE_DSN` and `APIGATE_REDIS_URL` to your complete service URLs. Credentials are included in the URLs.
- Generate a value for each of `APIGATE_API_KEY_SECRET` and `APIGATE_SETUP_TOKEN` using `openssl rand -hex 32`.
- Set `APIGATE_PUBLIC_URL` to the origin you will open in your browser. Use your HTTPS origin for a public deployment.

Keep `.env` private. Use `redis://` for Redis without TLS or `rediss://` with TLS. Percent-encode reserved characters in URL credentials. The database user must be able to run migrations. `localhost` inside the container refers to the container itself; use a reachable database hostname or host address.

```sh
docker compose pull
docker compose up -d --wait
```

Open **http://localhost:8080/setup**, enter `APIGATE_SETUP_TOKEN` from `.env`, and create the administrator account.

| Address | Purpose |
| --- | --- |
| `/admin` | APIs, pricing, customers, payments and settings |
| `/portal` | Customer wallets, plans, keys and usage |
| `/docs` | Documentation for your configured API routes |
| `/ready` | Gateway, PostgreSQL and Redis readiness |

In administration, configure **Plans & Pricing**, add upstreams and routes under **API Configuration**, and enable payment providers in **Settings**. Customers then sign up, fund their wallets, optionally purchase a plan, and create an API key. See the [user guide](docs/USER_GUIDE.md) for payment configuration and accounting behavior.

## Docker commands

```sh
docker compose logs -f gateway
docker compose ps
docker compose down
docker compose up -d --wait
```

To upgrade, change the image version in `docker-compose.yaml`, then pull and start again. Docker Hub tags use complete versions such as `1.0.0`. Back up PostgreSQL through your database service and retain `.env` separately.

## Build from source

Requires Go 1.27.1 and Node 22.23.2.

```sh
npm ci --prefix webui
make all
./bin/better-apigate version
```

## CI and releases

Set the GitHub repository secrets `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN`. Branch pushes build the application; version tags such as `v1.0.1` publish native archives and `<DOCKERHUB_USERNAME>/better-apigate:1.0.1` for amd64 and arm64. Images are published only with full version tags.

This version uses PostgreSQL and Redis; migration from the original SQLite application is not included.
