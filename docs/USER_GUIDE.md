# better-apigate user guide

better-apigate provides a prepaid API gateway with a customer workspace at `/portal`, administration at `/admin`, and public documentation at `/docs`. All browser operations use the signed-in session; API requests use a separately generated API key.

## Set up the gateway

1. Follow [Docker installation](../README.md#docker-installation) to supply your existing PostgreSQL and Redis connection URLs and start the gateway with its deployment secrets.
2. Open `/setup`, enter the setup token, and create the initial administrator.
3. In **Plans & Pricing**, configure the base pay-as-you-go unit price and rate limit, then add paid 30-day plans with their prices and included units.
4. In **API Configuration**, add upstreams and routes. Enter each upstream's authentication credential directly in its form. Give each paid route a fixed positive integer unit cost. Mark a route public only when it is explicitly free.
5. In **Settings**, configure available top-up amounts and enable the payment providers your merchant accounts support. Configure signed callback secrets and the gateway's public HTTPS origin before accepting payments. Add email delivery if verification or password reset is needed. SMTP requires TLS by default.

Configuration is defined by the module YAML schemas. Wallets, ledger entries, purchased terms and usage reservations are changed through transactional billing actions rather than generic configuration forms.

## Configure routes and transforms

In **API Configuration → Upstreams**, each service has its own URL and credential. Choose None, Custom header, Bearer token or Basic authentication. Timeout and connection pooling settings are available in the same form. **Check** reports whether the service responds; an HTTP error response still means it was reached.

The route editor has six sections:

- **Routing:** path, HTTP methods, hostname/header conditions, priority, upstream, protocol, method override and path rewrite. Rewrite examples can be inserted directly into the field.
- **Documentation:** description and sample request/response bodies, published at `/docs` and included in OpenAPI. The docs provide editable cURL, JavaScript and Python request examples.
- **Usage & pricing:** fixed prepaid units per request plus the original request, response-field, response-size and custom metering modes. Response measurements appear in **Usage → Metered response usage**. They do not change the fixed prepaid charge.
- **Request transform:** set/remove headers and query parameters, and transform the JSON body.
- **Response transform:** set/remove headers and transform buffered response bodies.
- **Test:** use sample request/response data to evaluate matching, rewrites, transforms and metering without contacting the upstream or spending funds. **Test current draft** uses unsaved changes; **Match saved routes** checks the active routing table.

Header and query values use one `name=expression` per line. Quote literals, for example `X-Version="v2"`; dynamic values can use `userID`, `keyID` or `env("API_KEY")`. Expression fields include validation and insertable examples. For SSE metering, supply raw SSE events as the test response; **Use route examples** loads the current documentation samples.

## Fund an account and make the first request

1. Sign up and verify your email when required. The Overview page shows the next onboarding action.
2. Open **Wallet & Payments → Add funds**, choose the USD wallet credit and an enabled provider, then complete its checkout. EPUSDT displays its USDT quote separately from the USD credit.
3. Return to the order result. Pending means the gateway is waiting for verified provider confirmation; a browser redirect never credits money. The wallet updates when payment is verified. Expired orders can be replaced with a new checkout. A reversal can freeze paid access until the outstanding shortfall is resolved.
4. Optionally buy a plan in **Plans**. Its entire price is debited from available wallet funds. The purchase includes a clearly disclosed, initially enabled auto-renew preference.
5. Create a key in **API Keys**. Copy it immediately; the full secret is shown once. Select endpoint scopes and an expiry if needed.
6. Open **API Docs**, choose one of the configured routes, and use its request example with your key. For example, after configuring `/v1/echo`:

```sh
curl "$APIGATE_ORIGIN/v1/echo" \
  -H "X-API-Key: $APIGATE_API_KEY"
```

Set those variables locally to your gateway origin and the key you copied. Keep the secret outside source control. Revoking a key immediately prevents new admitted requests. All keys share the account's rate limit and funding.

## Understand balances and plan terms

Included units are used first. A request's remaining units are funded from the available wallet using its snapshotted unit price. Money is stored as integer USD millionths, and API monetary values are exact decimal strings.

Funds and units are reserved before the request goes upstream. Upstream 2xx–4xx responses charge; verified 5xx and transport failures release. A client disconnect alone does not establish an upstream failure. Available balance excludes all active holds. HTTP 402 means funding is insufficient, 429 means the account rate limit has been reached, and 503 means enforcement is unavailable.

Plans last 30 days, have no rollover and no proration, and renew internally from the wallet. A selected plan change applies at the next renewal. Disabling auto-renew ends the paid plan at expiry. If full renewal funds are unavailable, the account falls back to the base pay-as-you-go plan. A later top-up does not silently restart the plan: purchase it explicitly to reactivate. Active terms retain purchased pricing; the next term uses the applicable catalog pricing shown for renewal.

Overview shows available balance, included units, the next renewal or term end, request volume, errors and spending in the last 30 days. Spending comes from the complete ledger and separates requests from plan purchases. Wallet history includes page controls for older payments and balance changes.

## Administer accounts and reconcile outcomes

**Customers** supports name/email search and page controls. Manage an account to suspend access, record a positive credit or debit, or unfreeze access after a shortfall has been covered. Enter a concrete reason for every adjustment. Admin adjustments use idempotency keys so retries cannot apply a credit twice.

**Payments → Payment orders** lists pending, paid, expired and reversed orders. Reconcile checks the provider's authenticated payment lookup when available. For an externally verified refund without a signed callback, record its reversal with the amount and provider receipt/evidence; this does not initiate an external refund.

**Payments → Unresolved requests** preserves holds older than five minutes. Inspect upstream evidence before choosing charge or release. Holds left by a crash never expire into spendable funds automatically. **Payments → Audit trail** records who made financial/access decisions, the affected record and the reason. All three views support pagination.

**Settings** groups payment, billing, email and account options. Deployment-level changes, such as connection pools or upstream secret values, belong in the environment. Recreate the gateway container after changing `.env`.

The [generated OpenAPI contract](openapi-v2.json) documents the versioned browser APIs. The same Go models generate the frontend TypeScript types.

## Payment configuration

Enable providers in **Settings** and configure their webhook secrets. The callback address is `/api/v1/payment-webhooks/{provider}` on your public origin.

- **Stripe:** use one-time Checkout and configure successful checkout, refund and dispute events.
- **Paddle:** configure a top-up product, a non-recurring price, webhook secret, client token and merchant checkout origin. Use sandbox mode with sandbox credentials.
- **Lemon Squeezy:** configure the store, webhook secret and a top-up variant with a `one_time` Price. Enable order-created and order-refunded events.
- **EPUSDT:** configure the GMPay URL, merchant PID, API key and network. The adapter targets [GMPay revision 58141cd](https://github.com/GMWalletApp/epusdt/blob/58141cd148408bbe05b0cd45716d6110f3952007/wiki/API.md). Record externally verified reversals through the audited admin action.

## Deployment notes

PostgreSQL runs migrations at startup and stores authoritative billing records. Redis must retain rate-limit keys without eviction. Each gateway uses up to 32 database connections by default; set `APIGATE_DB_MAX_CONNS` to fit your database service's connection limit.

Use your database service's backup and restore tools. Stop the gateway before restoring, verify the ledger, and reconcile payments received since the backup before resuming access. Keep the API-key secret backed up separately; changing it invalidates existing keys.

Use a TLS reverse proxy for public access, and keep `APIGATE_PUBLIC_URL` equal to the browser origin for cookies, CSRF checks and payment callbacks.
