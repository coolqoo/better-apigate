package postgres_test

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coolqoo/better-apigate/adapters/postgres"
	"github.com/coolqoo/better-apigate/domain/wallet"
	"github.com/google/uuid"
)

func paymentTestDB(t *testing.T) *postgres.DB {
	t.Helper()
	dsn := os.Getenv("APIGATE_TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("set APIGATE_TEST_DATABASE_DSN to run PostgreSQL payment tests")
	}
	admin, err := postgres.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	schema := "payment_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = admin.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec("DROP SCHEMA " + schema + " CASCADE"); err != nil {
			t.Error(err)
		}
	})
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	db, err := postgres.Open(u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestApplyEPUSDTPayment(t *testing.T) {
	db := paymentTestDB(t)
	store := postgres.NewWalletStore(db)
	ctx := context.Background()
	for _, tc := range []struct {
		name, state, transactionID string
	}{
		{"elapsed_parent", "pending", ""},
		{"expired_parent", "expired", ""},
		{"direct_payment", "pending", "chain-transaction"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			userID, orderID, tradeID := "user_"+tc.name, "order_"+tc.name, "trade_"+tc.name
			if _, err := db.Exec("INSERT INTO users(id,email) VALUES(?,?)", userID, userID+"@example.com"); err != nil {
				t.Fatal(err)
			}
			if _, err := store.CreateOrder(ctx, wallet.Order{ID: orderID, UserID: userID, Provider: "epusdt", Amount: 10_000_000, Currency: "USD"}, "top-up"); err != nil {
				t.Fatal(err)
			}
			expiry := time.Now().Add(-time.Hour)
			if err := store.SetCheckout(ctx, orderID, wallet.Checkout{ProviderID: tradeID, ExpiresAt: &expiry}); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec("UPDATE payment_orders SET state=? WHERE id=?", tc.state, orderID); err != nil {
				t.Fatal(err)
			}
			event := wallet.PaymentEvent{EventID: "paid:" + tradeID, OrderID: orderID, ProviderID: tradeID, MerchantID: "1001", TransactionID: tc.transactionID, Amount: 10_000_000, Kind: "paid", Paid: true}
			for _, field := range []string{"amount", "trade", "provider", "currency"} {
				bad, provider := event, "epusdt"
				switch field {
				case "amount":
					bad.Amount++
				case "trade":
					bad.ProviderID = "other-trade"
				case "provider":
					provider = "stripe"
				case "currency":
					bad.Currency = "EUR"
				}
				if err := store.ApplyPayment(ctx, provider, bad); !errors.Is(err, wallet.ErrInvalid) {
					t.Fatalf("mismatched %s: error = %v, want ErrInvalid", field, err)
				}
			}
			for attempt := 0; attempt < 2; attempt++ {
				if err := store.ApplyPayment(ctx, "epusdt", event); err != nil {
					t.Fatalf("ApplyPayment attempt %d: %v", attempt, err)
				}
			}
			account, err := store.Account(ctx, userID)
			if err != nil || account.Balance != 10_000_000 {
				t.Fatalf("balance = %s, error = %v; want $10 credited once", account.Balance, err)
			}
			order, err := store.Order(ctx, userID, orderID)
			if err != nil || order.State != "paid" || order.ProviderID != tradeID {
				t.Fatalf("order = %+v, error = %v", order, err)
			}
			var transactionID sql.NullString
			var eventCount, ledgerCount int
			if err := db.QueryRow(`SELECT provider_transaction_id,
				(SELECT COUNT(*) FROM payment_events WHERE order_id=?),
				(SELECT COUNT(*) FROM wallet_ledger WHERE reference=?)
				FROM payment_orders WHERE id=?`, orderID, "payment:"+orderID, orderID).Scan(&transactionID, &eventCount, &ledgerCount); err != nil {
				t.Fatal(err)
			}
			if transactionID.String != tc.transactionID || transactionID.Valid != (tc.transactionID != "") || eventCount != 1 || ledgerCount != 1 {
				t.Fatalf("transaction ID = %+v, events = %d, ledger entries = %d", transactionID, eventCount, ledgerCount)
			}
		})
	}
}
