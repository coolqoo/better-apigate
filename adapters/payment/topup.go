package payment

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/coolqoo/better-apigate/domain/settings"
	"github.com/coolqoo/better-apigate/domain/wallet"
	"github.com/coolqoo/better-apigate/ports"
)

// EPUSDTRevision pins the upstream GMPay API contract used by this adapter.
const EPUSDTRevision = "aed4a970a28d734c8a35499496604b868a24ef7f"

var ErrIgnoredPayment = errors.New("payment event does not affect wallet funds")

// Registry holds concurrently enabled providers; no provider owns a wallet.
type Registry struct {
	providers map[string]ports.TopUpProvider
	callbacks map[string]ports.TopUpProvider
}

func NewRegistry(s settings.Settings) (*Registry, error) {
	r := &Registry{providers: map[string]ports.TopUpProvider{}, callbacks: map[string]ports.TopUpProvider{}}
	configs := []topUpConfig{
		{id: "stripe", name: "Credit card · Stripe", base: "https://api.stripe.com", key: s.Get(settings.KeyPaymentStripeSecretKey), webhook: s.Get(settings.KeyPaymentStripeWebhookSecret)},
		{id: "paddle", name: "Credit card · Paddle", base: "https://api.paddle.com", key: s.Get(settings.KeyPaymentPaddleAPIKey), webhook: s.Get(settings.KeyPaymentPaddleWebhookSecret), product: s.Get("payment.paddle.topup_product_id")},
		{id: "lemonsqueezy", name: "Credit card · Lemon Squeezy", base: "https://api.lemonsqueezy.com", key: s.Get(settings.KeyPaymentLemonAPIKey), webhook: s.Get(settings.KeyPaymentLemonWebhookSecret), store: s.Get(settings.KeyPaymentLemonStoreID), product: s.Get("payment.lemonsqueezy.topup_variant_id")},
		{id: "epusdt", name: "Crypto · EPUSDT", base: s.Get("payment.epusdt.base_url"), key: s.Get("payment.epusdt.secret_key"), merchant: s.Get("payment.epusdt.pid"), network: strings.TrimSpace(s.Get("payment.epusdt.network"))},
	}
	for _, c := range configs {
		enabled := s.GetBool("payment." + c.id + ".enabled")
		if !enabled && (c.key == "" || (c.id != "epusdt" && c.webhook == "")) {
			continue
		}
		if c.key == "" {
			return nil, fmt.Errorf("%s API secret is required", c.id)
		}
		if c.id == "epusdt" {
			u, err := url.Parse(c.base)
			if err != nil || u.Host == "" || u.Scheme != "https" && u.Scheme != "http" || c.merchant == "" {
				return nil, fmt.Errorf("EPUSDT requires a base URL and merchant PID")
			}
		} else if c.webhook == "" {
			return nil, fmt.Errorf("%s webhook secret is required", c.id)
		}
		if c.id == "paddle" && s.GetBool("payment.paddle.sandbox") {
			c.base = "https://sandbox-api.paddle.com"
		}
		if enabled && (c.id == "paddle" || c.id == "lemonsqueezy") && c.product == "" {
			return nil, fmt.Errorf("%s requires a one-time top-up product", c.id)
		}
		if c.id == "lemonsqueezy" && c.store == "" {
			if !enabled {
				continue
			}
			return nil, fmt.Errorf("Lemon Squeezy requires a merchant store ID")
		}
		provider := &TopUp{cfg: c, client: &http.Client{Timeout: 20 * time.Second}}
		r.callbacks[c.id] = provider
		if enabled {
			r.providers[c.id] = provider
		}
	}
	return r, nil
}
func (r *Registry) CallbackProvider(id string) (ports.TopUpProvider, bool) {
	p, ok := r.callbacks[id]
	return p, ok
}
func (r *Registry) Get(id string) (ports.TopUpProvider, bool) { p, ok := r.providers[id]; return p, ok }
func (r *Registry) List() []wallet.ProviderInfo {
	out := []wallet.ProviderInfo{}
	for _, id := range []string{"stripe", "paddle", "lemonsqueezy", "epusdt"} {
		if p, ok := r.providers[id]; ok {
			out = append(out, p.Info())
		}
	}
	return out
}

type topUpConfig struct{ id, name, base, key, webhook, merchant, network, product, store string }
type TopUp struct {
	cfg    topUpConfig
	client *http.Client
}

func (p *TopUp) Info() wallet.ProviderInfo {
	return wallet.ProviderInfo{ID: p.cfg.id, Name: p.cfg.name, Crypto: p.cfg.id == "epusdt"}
}
func (p *TopUp) request(ctx context.Context, method, path, contentType string, body []byte, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(p.cfg.base, "/")+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if p.cfg.id != "epusdt" {
		req.Header.Set("Authorization", "Bearer "+p.cfg.key)
	}
	if p.cfg.id == "lemonsqueezy" {
		req.Header.Set("Accept", "application/vnd.api+json")
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s returned HTTP %d", p.cfg.id, resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out)
}
func (p *TopUp) CreateTopUp(ctx context.Context, r wallet.CheckoutRequest) (wallet.Checkout, error) {
	if r.Currency != "USD" || r.Amount <= 0 || int64(r.Amount)%10_000 != 0 {
		return wallet.Checkout{}, wallet.ErrInvalid
	}
	switch p.cfg.id {
	case "epusdt":
		return p.createEPUSDT(ctx, r)
	case "stripe":
		return p.createStripe(ctx, r)
	case "paddle":
		return p.createPaddle(ctx, r)
	case "lemonsqueezy":
		return p.createLemon(ctx, r)
	}
	return wallet.Checkout{}, wallet.ErrInvalid
}

// gmpaySign implements the pinned upstream's ASCII-sorted HMAC contract.
func gmpaySign(params map[string]string, secret string) string {
	keys := []string{}
	for k, v := range params {
		if k != "signature" && v != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, k+"="+params[k])
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strings.Join(pairs, "&")))
	return hex.EncodeToString(mac.Sum(nil))
}

type epusdtResponse struct {
	StatusCode int `json:"status_code"`
	Data       struct {
		TradeID      string      `json:"trade_id"`
		OrderID      string      `json:"order_id"`
		Amount       json.Number `json:"amount"`
		Currency     string      `json:"currency"`
		ActualAmount json.Number `json:"actual_amount"`
		Token        string      `json:"token"`
		Status       int         `json:"status"`
		Expiration   int64       `json:"expiration_time"`
		PaymentURL   string      `json:"payment_url"`
	} `json:"data"`
}

func (p *TopUp) createEPUSDT(ctx context.Context, r wallet.CheckoutRequest) (wallet.Checkout, error) {
	params := map[string]string{"pid": p.cfg.merchant, "order_id": r.OrderID, "currency": "usd", "notify_url": r.NotifyURL, "redirect_url": r.SuccessURL, "name": "API wallet top-up"}
	if p.cfg.network != "" {
		params["token"] = "usdt"
		params["network"] = p.cfg.network
	}
	// Amount is sent as a normalized dollar decimal, never as binary float.
	params["amount"] = strings.TrimRight(strings.TrimRight(r.Amount.String(), "0"), ".")
	params["signature"] = gmpaySign(params, p.cfg.key)
	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	var response epusdtResponse
	if err := p.request(ctx, "POST", "/payments/gmpay/v1/order/create-transaction", "application/x-www-form-urlencoded", []byte(form.Encode()), &response); err != nil {
		return wallet.Checkout{}, err
	}
	d := response.Data
	amount, err := wallet.ParseMoney(d.Amount.String())
	if response.StatusCode != 200 || err != nil || amount != r.Amount || d.OrderID != r.OrderID || d.TradeID == "" || !strings.EqualFold(d.Currency, "USD") {
		return wallet.Checkout{}, wallet.ErrInvalid
	}
	checkout := wallet.Checkout{ProviderID: d.TradeID, URL: d.PaymentURL}
	switch d.Status {
	case 4:
		// A placeholder has no crypto quote until the customer chooses on the cashier.
		quote, err := d.ActualAmount.Float64()
		if p.cfg.network != "" || d.Token != "" || err != nil || quote != 0 {
			return wallet.Checkout{}, wallet.ErrInvalid
		}
	case 1:
		quote, err := strconv.ParseFloat(d.ActualAmount.String(), 64)
		if p.cfg.network == "" || !strings.EqualFold(d.Token, "USDT") || err != nil || quote <= 0 || math.IsInf(quote, 0) || math.IsNaN(quote) {
			return wallet.Checkout{}, wallet.ErrInvalid
		}
		checkout.CryptoAmount, checkout.CryptoToken = d.ActualAmount.String(), "USDT"
	default:
		return wallet.Checkout{}, wallet.ErrInvalid
	}
	u, err := url.Parse(d.PaymentURL)
	base, _ := url.Parse(p.cfg.base)
	if err != nil || u.Host != base.Host || u.Scheme != base.Scheme {
		return wallet.Checkout{}, wallet.ErrInvalid
	}
	expiry := time.Unix(d.Expiration, 0).UTC()
	if !expiry.After(time.Now()) {
		return wallet.Checkout{}, wallet.ErrInvalid
	}
	checkout.ExpiresAt = &expiry
	return checkout, nil
}
func (p *TopUp) verifyEPUSDT(body []byte) (wallet.PaymentEvent, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return wallet.PaymentEvent{}, err
	}
	params := map[string]string{}
	for k, b := range raw {
		if string(b) == "null" {
			continue
		}
		var s string
		if json.Unmarshal(b, &s) == nil {
			params[k] = s
		} else {
			var n json.Number
			if json.Unmarshal(b, &n) != nil {
				return wallet.PaymentEvent{}, wallet.ErrInvalid
			}
			params[k] = n.String()
		}
	}
	signature, err := hex.DecodeString(params["signature"])
	if err != nil {
		return wallet.PaymentEvent{}, wallet.ErrInvalid
	}
	expected, _ := hex.DecodeString(gmpaySign(params, p.cfg.key))
	if !hmac.Equal(signature, expected) || params["pid"] != p.cfg.merchant {
		return wallet.PaymentEvent{}, wallet.ErrInvalid
	}
	amount, err := wallet.ParseMoney(params["amount"])
	if err != nil {
		return wallet.PaymentEvent{}, err
	}
	if params["trade_id"] == "" || params["order_id"] == "" || params["block_transaction_id"] == "" {
		return wallet.PaymentEvent{}, wallet.ErrInvalid
	}
	if params["status"] != "2" {
		return wallet.PaymentEvent{}, ErrIgnoredPayment
	}
	if strings.TrimSpace(params["token"]) == "" {
		return wallet.PaymentEvent{}, wallet.ErrInvalid
	}
	return wallet.PaymentEvent{EventID: "paid:" + params["trade_id"], OrderID: params["order_id"], ProviderID: params["trade_id"], TransactionID: params["block_transaction_id"], MerchantID: params["pid"], Amount: amount, Currency: strings.ToUpper(params["currency"]), Kind: "paid", Paid: true}, nil
}

type stripeCheckout struct {
	ID             string `json:"id"`
	ExpiresAt      int64  `json:"expires_at"`
	URL            string `json:"url"`
	PaymentStatus  string `json:"payment_status"`
	Currency       string `json:"currency"`
	AmountSubtotal int64  `json:"amount_subtotal"`
	PaymentIntent  string `json:"payment_intent"`
	Metadata       struct {
		OrderID string `json:"order_id"`
	} `json:"metadata"`
}

func (p *TopUp) createStripe(ctx context.Context, r wallet.CheckoutRequest) (wallet.Checkout, error) {
	form := url.Values{"mode": {"payment"}, "success_url": {r.SuccessURL}, "cancel_url": {r.CancelURL}, "customer_email": {r.Email}, "client_reference_id": {r.OrderID}, "metadata[order_id]": {r.OrderID}, "payment_intent_data[metadata][order_id]": {r.OrderID}, "line_items[0][quantity]": {"1"}, "line_items[0][price_data][currency]": {"usd"}, "line_items[0][price_data][unit_amount]": {strconv.FormatInt(int64(r.Amount)/10_000, 10)}, "line_items[0][price_data][product_data][name]": {"API wallet credit"}}
	expiry := time.Now().UTC().Add(time.Hour)
	form.Set("expires_at", strconv.FormatInt(expiry.Unix(), 10))
	var c stripeCheckout
	if err := p.request(ctx, "POST", "/v1/checkout/sessions", "application/x-www-form-urlencoded", []byte(form.Encode()), &c); err != nil {
		return wallet.Checkout{}, err
	}
	if c.ID == "" || !validCheckoutURL(c.URL) {
		return wallet.Checkout{}, wallet.ErrInvalid
	}
	if c.ExpiresAt > 0 {
		expiry = time.Unix(c.ExpiresAt, 0).UTC()
	}
	if !expiry.After(time.Now()) {
		return wallet.Checkout{}, wallet.ErrInvalid
	}
	return wallet.Checkout{ProviderID: c.ID, URL: c.URL, ExpiresAt: &expiry}, nil
}

type paddleTransaction struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Currency string `json:"currency_code"`
	Custom   struct {
		OrderID string `json:"order_id"`
	} `json:"custom_data"`
	Checkout struct {
		URL string `json:"url"`
	} `json:"checkout"`
	Details struct {
		Totals struct {
			Subtotal string `json:"subtotal"`
		} `json:"totals"`
	} `json:"details"`
}

func (p *TopUp) createPaddle(ctx context.Context, r wallet.CheckoutRequest) (wallet.Checkout, error) {
	type price struct {
		Description  string  `json:"description"`
		ProductID    string  `json:"product_id"`
		BillingCycle *string `json:"billing_cycle"`
		TaxMode      string  `json:"tax_mode"`
		UnitPrice    struct {
			Amount   string `json:"amount"`
			Currency string `json:"currency_code"`
		} `json:"unit_price"`
	}
	type item struct {
		Price    price `json:"price"`
		Quantity int   `json:"quantity"`
	}
	request := struct {
		Items          []item `json:"items"`
		Currency       string `json:"currency_code"`
		CollectionMode string `json:"collection_mode"`
		Custom         struct {
			OrderID string `json:"order_id"`
		} `json:"custom_data"`
		Checkout struct {
			URL string `json:"url"`
		} `json:"checkout"`
	}{Currency: "USD", CollectionMode: "automatic"}
	it := item{Quantity: 1, Price: price{Description: "API wallet credit", ProductID: p.cfg.product, TaxMode: "external"}}
	it.Price.UnitPrice.Amount = strconv.FormatInt(int64(r.Amount)/10_000, 10)
	it.Price.UnitPrice.Currency = "USD"
	request.Items = []item{it}
	request.Custom.OrderID = r.OrderID
	request.Checkout.URL = r.SuccessURL
	b, err := json.Marshal(request)
	if err != nil {
		return wallet.Checkout{}, err
	}
	var response struct {
		Data paddleTransaction `json:"data"`
	}
	if err = p.request(ctx, "POST", "/transactions", "application/json", b, &response); err != nil {
		return wallet.Checkout{}, err
	}
	if response.Data.ID == "" || !validCheckoutURL(response.Data.Checkout.URL) {
		return wallet.Checkout{}, wallet.ErrInvalid
	}
	return wallet.Checkout{ProviderID: response.Data.ID, URL: response.Data.Checkout.URL}, nil
}

type lemonOrder struct {
	ID         string `json:"id"`
	Attributes struct {
		Identifier     string `json:"identifier"`
		Status         string `json:"status"`
		Currency       string `json:"currency"`
		Subtotal       int64  `json:"subtotal"`
		RefundedAmount int64  `json:"refunded_amount"`
		StoreID        int64  `json:"store_id"`
	} `json:"attributes"`
}

func (p *TopUp) createLemon(ctx context.Context, r wallet.CheckoutRequest) (wallet.Checkout, error) {
	var prices struct {
		Data []struct {
			Attributes struct {
				VariantID int64  `json:"variant_id"`
				Category  string `json:"category"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := p.request(ctx, "GET", "/v1/prices?filter[variant_id]="+url.QueryEscape(p.cfg.product)+"&page[size]=1", "application/vnd.api+json", nil, &prices); err != nil {
		return wallet.Checkout{}, err
	}
	if len(prices.Data) != 1 || strconv.FormatInt(prices.Data[0].Attributes.VariantID, 10) != p.cfg.product || prices.Data[0].Attributes.Category != "one_time" {
		return wallet.Checkout{}, fmt.Errorf("top-up variant must have a one-time price")
	}

	type relationship struct {
		Data struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		} `json:"data"`
	}
	request := struct {
		Data struct {
			Type       string `json:"type"`
			Attributes struct {
				Price          int64     `json:"custom_price"`
				ExpiresAt      time.Time `json:"expires_at"`
				ProductOptions struct {
					Redirect        string  `json:"redirect_url"`
					EnabledVariants []int64 `json:"enabled_variants"`
				} `json:"product_options"`
				CheckoutOptions struct {
					Discount bool `json:"discount"`
				} `json:"checkout_options"`
				CheckoutData struct {
					Email  string `json:"email"`
					Custom struct {
						OrderID string `json:"order_id"`
					} `json:"custom"`
				} `json:"checkout_data"`
			} `json:"attributes"`
			Relationships struct {
				Store   relationship `json:"store"`
				Variant relationship `json:"variant"`
			} `json:"relationships"`
		} `json:"data"`
	}{}
	request.Data.Type = "checkouts"
	a := &request.Data.Attributes
	a.Price = int64(r.Amount) / 10_000
	a.ExpiresAt = time.Now().UTC().Add(time.Hour)
	a.ProductOptions.Redirect = r.SuccessURL
	variant, err := strconv.ParseInt(p.cfg.product, 10, 64)
	if err != nil {
		return wallet.Checkout{}, err
	}
	a.ProductOptions.EnabledVariants = []int64{variant}
	a.CheckoutData.Email = r.Email
	a.CheckoutData.Custom.OrderID = r.OrderID
	request.Data.Relationships.Store.Data.Type = "stores"
	request.Data.Relationships.Store.Data.ID = p.cfg.store
	request.Data.Relationships.Variant.Data.Type = "variants"
	request.Data.Relationships.Variant.Data.ID = p.cfg.product
	b, err := json.Marshal(request)
	if err != nil {
		return wallet.Checkout{}, err
	}
	var response struct {
		Data struct {
			ID         string `json:"id"`
			Attributes struct {
				URL       string     `json:"url"`
				ExpiresAt *time.Time `json:"expires_at"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err = p.request(ctx, "POST", "/v1/checkouts", "application/vnd.api+json", b, &response); err != nil {
		return wallet.Checkout{}, err
	}
	if response.Data.ID == "" || !validCheckoutURL(response.Data.Attributes.URL) {
		return wallet.Checkout{}, wallet.ErrInvalid
	}
	expiry := a.ExpiresAt
	if response.Data.Attributes.ExpiresAt != nil {
		expiry = *response.Data.Attributes.ExpiresAt
	}
	if !expiry.After(time.Now()) {
		return wallet.Checkout{}, wallet.ErrInvalid
	}
	return wallet.Checkout{ProviderID: response.Data.ID, URL: response.Data.Attributes.URL, ExpiresAt: &expiry}, nil
}
func (p *TopUp) LookupPayment(ctx context.Context, o wallet.Order) (wallet.PaymentEvent, error) {
	event, err := p.lookupPayment(ctx, o)
	if err != nil {
		return event, err
	}
	return validatePaymentEvent(event)
}
func (p *TopUp) lookupPayment(ctx context.Context, o wallet.Order) (wallet.PaymentEvent, error) {
	switch p.cfg.id {
	case "stripe":
		var c stripeCheckout
		if err := p.request(ctx, "GET", "/v1/checkout/sessions/"+url.PathEscape(o.ProviderID), "", nil, &c); err != nil {
			return wallet.PaymentEvent{}, err
		}
		return wallet.PaymentEvent{EventID: "lookup:" + c.ID, OrderID: c.Metadata.OrderID, ProviderID: c.ID, Amount: centsToMoney(c.AmountSubtotal), Currency: strings.ToUpper(c.Currency), Paid: c.PaymentStatus == "paid", Kind: "paid"}, nil
	case "paddle":
		var r struct {
			Data paddleTransaction `json:"data"`
		}
		if err := p.request(ctx, "GET", "/transactions/"+url.PathEscape(o.ProviderID), "", nil, &r); err != nil {
			return wallet.PaymentEvent{}, err
		}
		return paddleEvent("lookup:"+r.Data.ID, r.Data)
	case "lemonsqueezy":
		if o.State != "paid" && o.State != "reversed" {
			return wallet.PaymentEvent{}, ErrIgnoredPayment
		}
		var r struct {
			Data lemonOrder `json:"data"`
		}
		if err := p.request(ctx, "GET", "/v1/orders/"+url.PathEscape(o.ProviderID), "", nil, &r); err != nil {
			return wallet.PaymentEvent{}, err
		}
		if strconv.FormatInt(r.Data.Attributes.StoreID, 10) != p.cfg.store {
			return wallet.PaymentEvent{}, wallet.ErrInvalid
		}
		return wallet.PaymentEvent{EventID: "lookup:" + r.Data.ID, OrderID: o.ID, ProviderID: r.Data.ID, Amount: centsToMoney(r.Data.Attributes.Subtotal), Currency: strings.ToUpper(r.Data.Attributes.Currency), Paid: r.Data.Attributes.Status == "paid", Kind: "paid"}, nil
	case "epusdt":
		// Upstream's public status endpoint is useful for display, but its unsigned
		// status response is never sufficient evidence to credit a wallet.
		return wallet.PaymentEvent{}, ErrIgnoredPayment
	}
	return wallet.PaymentEvent{}, wallet.ErrInvalid
}
func (p *TopUp) verifyPayment(ctx context.Context, b []byte, h http.Header) (wallet.PaymentEvent, error) {
	switch p.cfg.id {
	case "epusdt":
		return p.verifyEPUSDT(b)
	case "lemonsqueezy":
		if !validMAC(b, h.Get("X-Signature"), p.cfg.webhook) {
			return wallet.PaymentEvent{}, wallet.ErrInvalid
		}
		var event struct {
			Meta struct {
				EventName string `json:"event_name"`
				Custom    struct {
					OrderID string `json:"order_id"`
				} `json:"custom_data"`
			} `json:"meta"`
			Data lemonOrder `json:"data"`
		}
		if err := json.Unmarshal(b, &event); err != nil {
			return wallet.PaymentEvent{}, err
		}
		if strconv.FormatInt(event.Data.Attributes.StoreID, 10) != p.cfg.store {
			return wallet.PaymentEvent{}, wallet.ErrInvalid
		}
		e := wallet.PaymentEvent{EventID: event.Meta.EventName + ":" + event.Data.ID + ":" + strconv.FormatInt(event.Data.Attributes.RefundedAmount, 10), OrderID: event.Meta.Custom.OrderID, ProviderID: event.Data.ID, MerchantID: p.cfg.store, Currency: strings.ToUpper(event.Data.Attributes.Currency), Amount: centsToMoney(event.Data.Attributes.Subtotal)}
		switch event.Meta.EventName {
		case "order_created":
			e.Paid = event.Data.Attributes.Status == "paid"
			e.Kind = "paid"
		case "order_refunded":
			e.Kind = "refund"
			e.ReversedAmount = centsToMoney(event.Data.Attributes.RefundedAmount)
			if e.ReversedAmount > e.Amount {
				e.ReversedAmount = e.Amount
			}
		default:
			return e, ErrIgnoredPayment
		}
		return e, nil
	case "paddle":
		fields := signatureFields(h.Get("Paddle-Signature"), ";")
		timestamp, err := strconv.ParseInt(fields["ts"], 10, 64)
		if err != nil || time.Since(time.Unix(timestamp, 0)).Abs() > 5*time.Minute || !validMACSet(append([]byte(fields["ts"]+":"), b...), h.Get("Paddle-Signature"), ";", "h1", p.cfg.webhook) {
			return wallet.PaymentEvent{}, wallet.ErrInvalid
		}
		var event struct {
			ID   string          `json:"event_id"`
			Type string          `json:"event_type"`
			Data json.RawMessage `json:"data"`
		}
		if err = json.Unmarshal(b, &event); err != nil {
			return wallet.PaymentEvent{}, err
		}
		if event.Type == "transaction.completed" {
			var t paddleTransaction
			if err = json.Unmarshal(event.Data, &t); err != nil {
				return wallet.PaymentEvent{}, err
			}
			return paddleEvent(event.ID, t)
		}
		if event.Type == "adjustment.updated" {
			var adj struct {
				ID            string `json:"id"`
				TransactionID string `json:"transaction_id"`
				Status        string `json:"status"`
				Action        string `json:"action"`
				Type          string `json:"type"`
				Totals        struct {
					Subtotal string `json:"subtotal"`
				} `json:"totals"`
			}
			if err = json.Unmarshal(event.Data, &adj); err != nil {
				return wallet.PaymentEvent{}, err
			}
			if adj.ID == "" {
				return wallet.PaymentEvent{}, wallet.ErrInvalid
			}
			if adj.Status != "approved" || adj.Action != "refund" && adj.Action != "chargeback" {
				return wallet.PaymentEvent{}, ErrIgnoredPayment
			}
			var response struct {
				Data paddleTransaction `json:"data"`
			}
			if err = p.request(ctx, "GET", "/transactions/"+url.PathEscape(adj.TransactionID), "", nil, &response); err != nil {
				return wallet.PaymentEvent{}, err
			}
			e, err := paddleEvent("adjustment:"+adj.ID, response.Data)
			if err != nil {
				return e, err
			}
			e.Paid = false
			e.Kind = "refund"
			if adj.Action == "chargeback" {
				e.Kind = "reversal"
			}
			if adj.Type == "full" {
				e.ReversedAmount = e.Amount
			} else {
				n, err := strconv.ParseInt(adj.Totals.Subtotal, 10, 64)
				if err != nil {
					return e, err
				}
				e.ReversalIsDelta = true
				e.ReversedAmount = centsToMoney(n)
			}
			return e, nil
		}
		return wallet.PaymentEvent{}, ErrIgnoredPayment
	case "stripe":
		return p.verifyStripe(ctx, b, h)
	}
	return wallet.PaymentEvent{}, wallet.ErrInvalid
}
func signatureFields(s, separator string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(s, separator) {
		pair := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(pair) == 2 {
			out[pair[0]] = pair[1]
		}
	}
	return out
}
func validMAC(body []byte, signature, secret string) bool {
	got, err := hex.DecodeString(signature)
	if err != nil || secret == "" {
		return false
	}
	m := hmac.New(sha256.New, []byte(secret))
	m.Write(body)
	return hmac.Equal(got, m.Sum(nil))
}
func paddleEvent(id string, t paddleTransaction) (wallet.PaymentEvent, error) {
	n, err := strconv.ParseInt(t.Details.Totals.Subtotal, 10, 64)
	if err != nil {
		return wallet.PaymentEvent{}, err
	}
	return wallet.PaymentEvent{EventID: id, OrderID: t.Custom.OrderID, ProviderID: t.ID, Currency: strings.ToUpper(t.Currency), Amount: centsToMoney(n), Paid: t.Status == "completed", Kind: "paid"}, nil
}
func (p *TopUp) verifyStripe(ctx context.Context, b []byte, h http.Header) (wallet.PaymentEvent, error) {
	fields := signatureFields(h.Get("Stripe-Signature"), ",")
	timestamp, err := strconv.ParseInt(fields["t"], 10, 64)
	if err != nil || time.Since(time.Unix(timestamp, 0)).Abs() > 5*time.Minute || !validMACSet(append([]byte(fields["t"]+"."), b...), h.Get("Stripe-Signature"), ",", "v1", p.cfg.webhook) {
		return wallet.PaymentEvent{}, wallet.ErrInvalid
	}
	var event struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Data struct {
			Object json.RawMessage `json:"object"`
		} `json:"data"`
	}
	if err = json.Unmarshal(b, &event); err != nil {
		return wallet.PaymentEvent{}, err
	}
	if event.Type == "checkout.session.completed" || event.Type == "checkout.session.async_payment_succeeded" {
		var c stripeCheckout
		if err = json.Unmarshal(event.Data.Object, &c); err != nil {
			return wallet.PaymentEvent{}, err
		}
		return wallet.PaymentEvent{EventID: event.ID, OrderID: c.Metadata.OrderID, ProviderID: c.ID, Currency: strings.ToUpper(c.Currency), Amount: centsToMoney(c.AmountSubtotal), Paid: c.PaymentStatus == "paid", Kind: "paid"}, nil
	}
	if event.Type != "charge.refunded" && event.Type != "charge.dispute.created" {
		return wallet.PaymentEvent{}, ErrIgnoredPayment
	}
	var charge struct {
		ID             string `json:"id"`
		Charge         string `json:"charge"`
		PaymentIntent  string `json:"payment_intent"`
		AmountRefunded int64  `json:"amount_refunded"`
	}
	if err = json.Unmarshal(event.Data.Object, &charge); err != nil {
		return wallet.PaymentEvent{}, err
	}
	reversal := event.Type == "charge.dispute.created"
	if reversal {
		if err = p.request(ctx, "GET", "/v1/charges/"+url.PathEscape(charge.Charge), "", nil, &charge); err != nil {
			return wallet.PaymentEvent{}, err
		}
	}
	var sessions struct {
		Data []stripeCheckout `json:"data"`
	}
	if err = p.request(ctx, "GET", "/v1/checkout/sessions?payment_intent="+url.QueryEscape(charge.PaymentIntent), "", nil, &sessions); err != nil {
		return wallet.PaymentEvent{}, err
	}
	if len(sessions.Data) != 1 {
		return wallet.PaymentEvent{}, wallet.ErrInvalid
	}
	c := sessions.Data[0]
	e := wallet.PaymentEvent{EventID: event.ID, OrderID: c.Metadata.OrderID, ProviderID: c.ID, Currency: strings.ToUpper(c.Currency), Amount: centsToMoney(c.AmountSubtotal), Kind: "refund", ReversedAmount: centsToMoney(charge.AmountRefunded)}
	if e.ReversedAmount > e.Amount {
		e.ReversedAmount = e.Amount
	}
	if reversal {
		e.Kind = "reversal"
		e.ReversedAmount = e.Amount
	}
	return e, nil
}

func validCheckoutURL(value string) bool {
	u, err := url.Parse(value)
	return err == nil && u.Host != "" && u.User == nil && (u.Scheme == "https" || u.Scheme == "http")
}

func centsToMoney(cents int64) wallet.Money {
	if cents < 0 || cents > math.MaxInt64/10_000 {
		return -1
	}
	return wallet.Money(cents * 10_000)
}
func (p *TopUp) VerifyPayment(ctx context.Context, body []byte, headers http.Header) (wallet.PaymentEvent, error) {
	event, err := p.verifyPayment(ctx, body, headers)
	if err != nil {
		return event, err
	}
	return validatePaymentEvent(event)
}
func validatePaymentEvent(event wallet.PaymentEvent) (wallet.PaymentEvent, error) {
	if event.EventID == "" || event.OrderID == "" || event.ProviderID == "" || event.Amount <= 0 || event.ReversedAmount < 0 {
		return wallet.PaymentEvent{}, wallet.ErrInvalid
	}
	return event, nil
}
func validMACSet(body []byte, header, separator, name, secret string) bool {
	for _, part := range strings.Split(header, separator) {
		pair := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(pair) == 2 && pair[0] == name && validMAC(body, pair[1], secret) {
			return true
		}
	}
	return false
}
