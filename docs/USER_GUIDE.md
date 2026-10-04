# APIGate v2 user guide

APIGate provides a prepaid API gateway with a customer workspace at `/portal`, administration at `/admin`, and public documentation at `/docs`. All browser operations use the signed-in session; API requests use a separately generated API key.

## Set up the gateway

1. Follow [deployment instructions](deployment-v2.md) to start PostgreSQL, Redis and the gateway with the deployment secrets.
2. Open `/setup`, enter the setup token, and create the initial administrator.
3. In **Plans & Pricing**, configure the base pay-as-you-go unit price and rate limit, then add paid 30-day plans with their prices and included units.
4. In **API Configuration**, add upstreams and routes. Give each paid route a fixed positive integer unit cost. Store upstream authentication as a deployment environment reference, for example `${UPSTREAM_API_TOKEN}`, and supply its value to the gateway environment. Mark a route public only when it is explicitly free.
5. In **Settings**, configure available top-up amounts and enable the payment providers your merchant accounts support. Configure signed callback secrets and the gateway's public HTTPS origin before accepting payments. Add email delivery if verification or password reset is needed. SMTP requires TLS by default.

Configuration fields are generated from the module YAML schemas. Wallets, ledger entries, purchased terms and usage reservations are changed through transactional billing actions rather than generic configuration forms.

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

**Settings** groups payment, billing, email and account options. Deployment-level changes, such as connection pools or upstream secret values, belong in the environment. See [deployment and recovery](deployment-v2.md) for health checks, multiple instances, database backup/restore and release requirements.

The [generated OpenAPI contract](openapi-v2.json) documents the versioned browser APIs. The same Go models generate the frontend TypeScript types.
