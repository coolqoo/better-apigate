package ports

import (
	"context"
	"github.com/artpar/apigate/domain/ratelimit"
	"github.com/artpar/apigate/domain/usage"
	"github.com/artpar/apigate/domain/wallet"
	"net/http"
	"time"
)

// PrepaidStore is the sole authority for funding authenticated requests.
type PrepaidStore interface {
	Account(context.Context, string) (wallet.Account, error)
	Plans(context.Context) ([]wallet.Plan, error)
	Reserve(context.Context, string, string, string, int64, time.Time) (wallet.Reservation, error)
	Settle(context.Context, string, bool, usage.Event) error
	Purchase(context.Context, string, string, string, time.Time) error
	RenewalPreference(context.Context, string, bool, string) error
	Ledger(context.Context, string, int, int) ([]wallet.LedgerEntry, error)
	Orders(context.Context, string, int) ([]wallet.Order, error)
	OrdersPage(context.Context, string, int, int) ([]wallet.Order, error)
	Order(context.Context, string, string) (wallet.Order, error)
	CreateOrder(context.Context, wallet.Order, string) (wallet.Order, error)
	SetCheckout(context.Context, string, wallet.Checkout) error
	ApplyPayment(context.Context, string, wallet.PaymentEvent) error
	ReverseOrder(context.Context, string, wallet.Money, string, string, string) error
	Adjust(context.Context, string, wallet.Money, string, string, string) error
	Pending(context.Context) ([]wallet.Reservation, error)
	PendingPage(context.Context, int, int) ([]wallet.Reservation, error)
	Resolve(context.Context, string, bool, string, string) error
}
type AtomicRateLimiter interface {
	Allow(context.Context, string, ratelimit.Config, time.Time) (ratelimit.CheckResult, error)
}
type TopUpProvider interface {
	Info() wallet.ProviderInfo
	CreateTopUp(context.Context, wallet.CheckoutRequest) (wallet.Checkout, error)
	LookupPayment(context.Context, wallet.Order) (wallet.PaymentEvent, error)
	VerifyPayment(context.Context, []byte, http.Header) (wallet.PaymentEvent, error)
}
