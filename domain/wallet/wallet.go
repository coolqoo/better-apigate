// Package wallet defines prepaid money and immutable accounting values.
package wallet

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

type Money int64

const MicrosPerDollar int64 = 1_000_000

var (
	ErrInsufficient = errors.New("insufficient prepaid funds")
	ErrFrozen       = errors.New("account requires payment reconciliation")
	ErrInvalid      = errors.New("invalid billing operation")
	ErrConflict     = errors.New("billing operation conflicts with existing state")
)

func ParseMoney(s string) (Money, error) {
	if s == "" || strings.TrimSpace(s) != s || strings.HasPrefix(s, "-") || strings.HasPrefix(s, "+") {
		return 0, ErrInvalid
	}
	parts := strings.Split(s, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, ErrInvalid
	}
	for _, p := range parts {
		for _, c := range p {
			if c < '0' || c > '9' {
				return 0, ErrInvalid
			}
		}
	}
	whole, e := strconv.ParseInt(parts[0], 10, 64)
	if e != nil || whole > math.MaxInt64/MicrosPerDollar {
		return 0, ErrInvalid
	}
	frac := ""
	if len(parts) == 2 {
		frac = parts[1]
		if frac == "" || len(frac) > 6 {
			return 0, ErrInvalid
		}
	}
	frac += strings.Repeat("0", 6-len(frac))
	f, e := strconv.ParseInt(frac, 10, 64)
	if e != nil || whole*MicrosPerDollar > math.MaxInt64-f {
		return 0, ErrInvalid
	}
	return Money(whole*MicrosPerDollar + f), nil
}
func (m Money) String() string {
	n := uint64(m)
	sign := ""
	if m < 0 {
		sign = "-"
		n = uint64(-(m + 1)) + 1
	}
	return fmt.Sprintf("%s%d.%06d", sign, n/uint64(MicrosPerDollar), n%uint64(MicrosPerDollar))
}
func (m Money) MarshalJSON() ([]byte, error) { return []byte(strconv.Quote(m.String())), nil }
func (m *Money) UnmarshalJSON(b []byte) error {
	s, e := strconv.Unquote(string(b))
	if e != nil {
		return ErrInvalid
	}
	v, e := ParseMoney(s)
	if e == nil {
		*m = v
	}
	return e
}
func Cost(units int64, price Money) (Money, error) {
	if units < 0 || price < 0 || int64(price) > 0 && units > math.MaxInt64/int64(price) {
		return 0, ErrInvalid
	}
	return Money(units * int64(price)), nil
}

type Account struct {
	UserID       string `json:"user_id"`
	Balance      Money  `json:"balance"`
	Reserved     Money  `json:"reserved"`
	Available    Money  `json:"available"`
	Shortfall    Money  `json:"shortfall"`
	Frozen       bool   `json:"frozen"`
	Currency     string `json:"currency"`
	AutoRenew    bool   `json:"auto_renew"`
	NextPlanID   string `json:"next_plan_id,omitempty"`
	RenewalPrice *Money `json:"renewal_price,omitempty"`
	Term         *Term  `json:"term,omitempty"`
}
type Plan struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Price         Money  `json:"price"`
	UnitPrice     Money  `json:"unit_price"`
	IncludedUnits int64  `json:"included_units"`
	RateLimit     int    `json:"rate_limit_per_minute"`
	TermDays      int    `json:"term_days"`
	IsBase        bool   `json:"is_base"`
	Enabled       bool   `json:"enabled"`
}
type Term struct {
	ID            string    `json:"id"`
	PlanID        string    `json:"plan_id"`
	PlanName      string    `json:"plan_name"`
	Price         Money     `json:"price"`
	UnitPrice     Money     `json:"unit_price"`
	IncludedUnits int64     `json:"included_units"`
	UsedUnits     int64     `json:"used_units"`
	ReservedUnits int64     `json:"reserved_units"`
	RateLimit     int       `json:"rate_limit_per_minute"`
	StartsAt      time.Time `json:"starts_at"`
	EndsAt        time.Time `json:"ends_at"`
}
type Reservation struct {
	PlanID     string    `json:"plan_id,omitempty"`
	RateLimit  int       `json:"rate_limit_per_minute,omitempty"`
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	KeyID      string    `json:"key_id"`
	RouteID    string    `json:"route_id"`
	TermID     string    `json:"term_id,omitempty"`
	Units      int64     `json:"units"`
	QuotaUnits int64     `json:"quota_units"`
	Amount     Money     `json:"amount"`
	UnitPrice  Money     `json:"unit_price"`
	State      string    `json:"state"`
	CreatedAt  time.Time `json:"created_at"`
}
type LedgerEntry struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Kind        string    `json:"kind"`
	Amount      Money     `json:"amount"`
	Balance     Money     `json:"balance"`
	Reference   string    `json:"reference"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}
type Order struct {
	ID           string     `json:"id"`
	UserID       string     `json:"user_id"`
	Provider     string     `json:"provider"`
	Amount       Money      `json:"amount"`
	Currency     string     `json:"currency"`
	State        string     `json:"state"`
	ProviderID   string     `json:"provider_id,omitempty"`
	CheckoutURL  string     `json:"checkout_url,omitempty"`
	CryptoAmount string     `json:"crypto_amount,omitempty"`
	CryptoToken  string     `json:"crypto_token,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}
type CheckoutRequest struct {
	OrderID, UserID, Email, Name, Currency, SuccessURL, CancelURL, NotifyURL string
	Amount                                                                   Money
}
type Checkout struct {
	ProviderID, URL, CryptoAmount, CryptoToken string
	ExpiresAt                                  *time.Time
}
type PaymentEvent struct {
	ActorID, Description                                     string // Set only by audited server-side administrative actions.
	EventID, OrderID, ProviderID, MerchantID, Currency, Kind string
	TransactionID                                            string
	Amount                                                   Money
	Paid                                                     bool
	ReversedAmount                                           Money
	ReversalIsDelta                                          bool
}
type ProviderInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Crypto bool   `json:"crypto"`
}
