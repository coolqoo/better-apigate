package payment

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/coolqoo/better-apigate/domain/wallet"
)

// Signed independently with epusdt-test-secret. A paid child leaves the parent
// order's original quote intact and its blockchain transaction ID empty.
const epusdtParentCallback = `{"pid":"1001","trade_id":"parent-trade","order_id":"order-10","amount":10,"actual_amount":10,"receive_address":"parent-tron-address","token":"USDT","block_transaction_id":"","status":2,"signature":"8c962b34003bca6f9a181a0fd8d9e5a4845d1c4b22f93235fe5e55960fec54ad"}`

func TestEPUSDTPaidCallback(t *testing.T) {
	p := &TopUp{cfg: topUpConfig{id: "epusdt", merchant: "1001", key: "epusdt-test-secret"}}
	direct := strings.Replace(epusdtParentCallback, `"block_transaction_id":""`, `"block_transaction_id":"chain-transaction"`, 1)
	direct = strings.Replace(direct, "8c962b34003bca6f9a181a0fd8d9e5a4845d1c4b22f93235fe5e55960fec54ad", "fe3b788c5b64f5e1eeeeb64dd049eeed79713a49bfc0ba3668de2e053198330b", 1)
	for _, tc := range []struct {
		name, body, transactionID string
	}{
		{"child payment with empty parent hash", epusdtParentCallback, ""},
		{"child payment with omitted parent hash", strings.Replace(epusdtParentCallback, `"block_transaction_id":"",`, "", 1), ""},
		{"child payment with null parent hash", strings.Replace(epusdtParentCallback, `"block_transaction_id":""`, `"block_transaction_id":null`, 1), ""},
		{"direct payment with hash", direct, "chain-transaction"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			event, err := p.VerifyPayment(context.Background(), []byte(tc.body), nil)
			if err != nil {
				t.Fatalf("VerifyPayment: %v", err)
			}
			want := wallet.PaymentEvent{EventID: "paid:parent-trade", OrderID: "order-10", ProviderID: "parent-trade", TransactionID: tc.transactionID, MerchantID: "1001", Amount: 10_000_000, Kind: "paid", Paid: true}
			if event != want {
				t.Fatalf("event = %+v, want %+v", event, want)
			}
		})
	}
}

func TestEPUSDTParentCallbackVerification(t *testing.T) {
	for _, tc := range []struct {
		name, body, merchant string
	}{
		{"tampered amount", strings.Replace(epusdtParentCallback, `"amount":10`, `"amount":11`, 1), "1001"},
		{"tampered signature", strings.Replace(epusdtParentCallback, "8c962b34", "00000000", 1), "1001"},
		{"wrong merchant with valid signature", epusdtParentCallback, "1000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &TopUp{cfg: topUpConfig{id: "epusdt", merchant: tc.merchant, key: "epusdt-test-secret"}}
			if _, err := p.VerifyPayment(context.Background(), []byte(tc.body), nil); !errors.Is(err, wallet.ErrInvalid) {
				t.Fatalf("VerifyPayment error = %v, want ErrInvalid", err)
			}
		})
	}
}
