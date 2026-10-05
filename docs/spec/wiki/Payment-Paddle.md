# Paddle Integration

Paddle handles payments, tax compliance, and merchant of record responsibilities.

---

## Setup

### 1. Create Paddle Account

1. Sign up at [paddle.com](https://paddle.com)
2. Complete verification
3. Get credentials from Paddle Dashboard

### 2. Configure better-apigate

```bash
# Via CLI settings
better-apigate settings set payment.provider paddle
better-apigate settings set payment.paddle.vendor_id "123456"
better-apigate settings set payment.paddle.api_key "xxx" --encrypted
better-apigate settings set payment.paddle.public_key "xxx"
better-apigate settings set payment.paddle.webhook_secret "xxx" --encrypted
```

### 3. Set Up Webhooks

In Paddle Dashboard > Developer Tools > Notifications:

1. Add webhook URL: `https://your-domain.com/webhooks/paddle`
2. Select events:
   - `subscription_created`
   - `subscription_updated`
   - `subscription_cancelled`
   - `subscription_payment_succeeded`
   - `subscription_payment_failed`

---

## Plan Configuration

### 1. Create Plans in better-apigate

```bash
apigate plans create \
  --name "Pro" \
  --rate-limit-per-minute 1000 \
  --requests-per-month 100000 \
  --price-monthly 2900
```

### 2. Link to Paddle Price

After creating the plan:

1. Create a product with price in Paddle Dashboard
2. In better-apigate Admin UI, go to **Plans** and edit the plan
3. Enter the Paddle price ID in the Paddle Price ID field
4. Save the plan

> **Note**: Paddle price IDs must be linked via the Admin UI, not CLI.

---

## Benefits

- **Tax handling**: Paddle calculates and remits VAT/sales tax
- **Merchant of record**: Paddle handles refunds, chargebacks
- **Global payments**: Supports many currencies and payment methods

---

## Checkout Flow

Paddle uses overlay checkout:

1. User clicks "Subscribe"
2. Paddle checkout opens in overlay
3. User completes payment
4. Webhook notifies better-apigate
5. User plan activated

---

## Webhook Events

| Event | better-apigate Action |
|-------|----------------|
| `subscription_created` | Activate user plan |
| `subscription_updated` | Update user plan |
| `subscription_cancelled` | Revert to free plan |
| `subscription_payment_succeeded` | Record payment |
| `subscription_payment_failed` | Send notification |

---

## Testing

Use Paddle sandbox environment and configure with sandbox credentials in the Paddle Dashboard.

---

## See Also

- [[Plans]] - Plan configuration
- [[Providers]] - Provider overview
- [[Webhooks]] - Webhook handling
- [[Metering-API]] - External usage events
