// Package portal defines the public versioned API contract shared with the frontend.
package portal

import (
	"github.com/coolqoo/better-apigate/domain/wallet"
	"time"
)

type Session struct {
	UserID        string `json:"user_id"`
	Name          string `json:"name"`
	Email         string `json:"email"`
	Role          string `json:"role"`
	PlanID        string `json:"plan_id"`
	EmailVerified bool   `json:"email_verified"`
	CSRFToken     string `json:"csrf_token"`
}
type APIKey struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Prefix    string     `json:"prefix"`
	Scopes    []string   `json:"scopes"`
	CreatedAt time.Time  `json:"created_at"`
	LastUsed  *time.Time `json:"last_used"`
	ExpiresAt *time.Time `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at"`
	RawKey    string     `json:"raw_key,omitempty"`
}
type UsageDay struct {
	ID        string  `json:"id"`
	Requests  int64   `json:"requests"`
	Errors    int64   `json:"errors"`
	Units     int64   `json:"units"`
	LatencyMS float64 `json:"latency_ms"`
}
type UsageSummary struct {
	Requests      int64        `json:"requests"`
	Errors        int64        `json:"errors"`
	Units         int64        `json:"units"`
	Spending      wallet.Money `json:"spending"`
	UsageSpending wallet.Money `json:"usage_spending"`
	PlanSpending  wallet.Money `json:"plan_spending"`
}
type Installation struct {
	SetupRequired     bool     `json:"setup_required"`
	AppName           string   `json:"app_name"`
	Currency          string   `json:"currency"`
	TopUpAmounts      []string `json:"top_up_amounts"`
	PaddleClientToken string   `json:"paddle_client_token"`
	PaddleSandbox     bool     `json:"paddle_sandbox"`
}
type Customer struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Email         string       `json:"email"`
	Status        string       `json:"status"`
	PlanID        string       `json:"plan_id"`
	Balance       wallet.Money `json:"balance"`
	Shortfall     wallet.Money `json:"shortfall"`
	Frozen        bool         `json:"frozen"`
	Role          string       `json:"role"`
	EmailVerified bool         `json:"email_verified"`
	CreatedAt     time.Time    `json:"created_at"`
}
type Overview struct {
	Customers           int64        `json:"customers"`
	Requests            int64        `json:"requests"`
	Errors              int64        `json:"errors"`
	PendingReservations int64        `json:"pending_reservations"`
	WalletLiability     wallet.Money `json:"wallet_liability"`
	TopUpVolume         wallet.Money `json:"top_up_volume"`
}
type Settings struct {
	Values map[string]string `json:"values"`
}
type Pagination struct {
	Total   int64 `json:"total"`
	Page    int   `json:"page"`
	PerPage int   `json:"per_page"`
	Pages   int   `json:"pages"`
}
type AuditEntry struct {
	ID         string    `json:"id"`
	ActorID    string    `json:"actor_id"`
	ActorEmail string    `json:"actor_email"`
	Action     string    `json:"action"`
	TargetID   string    `json:"target_id"`
	Reason     string    `json:"reason"`
	CreatedAt  time.Time `json:"created_at"`
}
type Credentials struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	Name       string `json:"name,omitempty"`
	SetupToken string `json:"setup_token,omitempty"`
}
type TopUpRequest struct {
	Amount   wallet.Money `json:"amount"`
	Provider string       `json:"provider"`
}
type PurchaseRequest struct {
	PlanID string `json:"plan_id"`
}
type RenewalRequest struct {
	AutoRenew  bool   `json:"auto_renew"`
	NextPlanID string `json:"next_plan_id,omitempty"`
}
type KeyRequest struct {
	Name      string     `json:"name"`
	Scopes    []string   `json:"scopes,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}
type AdjustmentRequest struct {
	Amount    wallet.Money `json:"amount"`
	Direction string       `json:"direction"`
	Reason    string       `json:"reason"`
}
type ResolveRequest struct {
	Charge bool   `json:"charge"`
	Reason string `json:"reason"`
}
type CustomerStateRequest struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}
type ReasonRequest struct {
	Reason string `json:"reason"`
}
type AccountRequest struct {
	Name string `json:"name"`
}
type PasswordRequest struct {
	Current  string `json:"current_password"`
	Password string `json:"password"`
}
type EmailRequest struct {
	Email string `json:"email"`
}
type ChallengeRequest struct {
	Token    string `json:"token"`
	Password string `json:"password,omitempty"`
}

type AccountCreated struct {
	UserID               string `json:"user_id"`
	VerificationRequired bool   `json:"verification_required,omitempty"`
}
type Message struct {
	Message string `json:"message"`
}

type ReversalRequest struct {
	Amount wallet.Money `json:"amount"`
	Reason string       `json:"reason"`
}
