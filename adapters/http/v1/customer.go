package v1

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/artpar/apigate/adapters/payment"
	"github.com/artpar/apigate/core/runtime"
	"github.com/artpar/apigate/domain/key"
	"github.com/artpar/apigate/domain/portal"
	"github.com/artpar/apigate/domain/wallet"
	"github.com/artpar/apigate/pkg/jsonapi"
	"github.com/go-chi/chi/v5"
)

func (s *Server) plans(w http.ResponseWriter, r *http.Request) {
	p, e := s.Wallet.Plans(r.Context())
	if e != nil {
		failure(w, e)
		return
	}
	collection(w, "plan", p)
}
func (s *Server) providers(w http.ResponseWriter, r *http.Request) {
	registry, e := s.registry()
	if e != nil {
		failure(w, e)
		return
	}
	collection(w, "provider", registry.List())
}
func (s *Server) account(w http.ResponseWriter, r *http.Request) {
	a, e := s.Wallet.Account(r.Context(), current(r).UserID)
	if e != nil {
		failure(w, e)
		return
	}
	resource(w, 200, "wallet", a.UserID, a)
}
func (s *Server) ledger(w http.ResponseWriter, r *http.Request) {
	page, ok := requestPagination(w, r)
	if !ok {
		return
	}
	userID := current(r).UserID
	if err := s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM wallet_ledger WHERE user_id=?", userID).Scan(&page.Total); err != nil {
		failure(w, err)
		return
	}
	entries, err := s.Wallet.Ledger(r.Context(), userID, page.Limit(), page.Offset())
	if err != nil {
		failure(w, err)
		return
	}
	collectionPage(w, "ledger-entry", entries, page)
}
func (s *Server) orders(w http.ResponseWriter, r *http.Request) {
	page, ok := requestPagination(w, r)
	if !ok {
		return
	}
	userID := current(r).UserID
	if err := s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM payment_orders WHERE user_id=?", userID).Scan(&page.Total); err != nil {
		failure(w, err)
		return
	}
	orders, err := s.Wallet.OrdersPage(r.Context(), userID, page.Limit(), page.Offset())
	if err != nil {
		failure(w, err)
		return
	}
	collectionPage(w, "payment-order", orders, page)
}
func (s *Server) order(w http.ResponseWriter, r *http.Request) {
	o, e := s.Wallet.Order(r.Context(), current(r).UserID, chi.URLParam(r, "id"))
	if e != nil {
		failure(w, e)
		return
	}
	if o.State == "pending" && o.ExpiresAt != nil && o.ExpiresAt.Before(time.Now()) {
		_, e = s.DB.ExecContext(r.Context(), "UPDATE payment_orders SET state='expired' WHERE id=? AND state='pending' AND expires_at<=CURRENT_TIMESTAMP", o.ID)
		if e != nil {
			failure(w, e)
			return
		}
		o, e = s.Wallet.Order(r.Context(), current(r).UserID, o.ID)
		if e != nil {
			failure(w, e)
			return
		}
	}
	resource(w, 200, "payment-order", o.ID, o)
}
func (s *Server) topUpAmounts() []string {
	v := strings.Split(s.Settings.Get().GetOrDefault("billing.top_up_amounts", "10,25,50,100"), ",")
	out := []string{}
	for _, a := range v {
		m, e := wallet.ParseMoney(strings.TrimSpace(a))
		if e == nil && m > 0 && int64(m)%10_000 == 0 {
			out = append(out, m.String())
		}
	}
	return out
}
func (s *Server) topup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Amount   wallet.Money `json:"amount"`
		Provider string       `json:"provider"`
	}
	if !decode(w, r, &in) {
		return
	}
	u := current(r)
	if s.Settings.Get().GetBool("auth.require_verification") && !u.Verified {
		jsonapi.WriteForbidden(w, "Verify your email before adding funds.")
		return
	}
	allowed := false
	for _, a := range s.topUpAmounts() {
		if a == in.Amount.String() {
			allowed = true
		}
	}
	if !allowed {
		jsonapi.WriteValidationError(w, "amount", "Choose one of the configured top-up amounts.")
		return
	}
	registry, e := s.registry()
	if e != nil {
		failure(w, e)
		return
	}
	provider, ok := registry.Get(in.Provider)
	if !ok {
		jsonapi.WriteBadRequest(w, "Choose an enabled payment provider.")
		return
	}
	o, e := s.Wallet.CreateOrder(r.Context(), wallet.Order{UserID: u.UserID, Provider: in.Provider, Amount: in.Amount, Currency: "USD"}, r.Header.Get("Idempotency-Key"))
	if e != nil {
		failure(w, e)
		return
	}
	if o.CheckoutURL != "" || o.State != "pending" {
		resource(w, 200, "payment-order", o.ID, o)
		return
	}
	claim, e := s.DB.ExecContext(r.Context(), "UPDATE payment_orders SET checkout_attempted=TRUE WHERE id=? AND checkout_attempted=FALSE", o.ID)
	if e != nil {
		failure(w, e)
		return
	}
	n, e := claim.RowsAffected()
	if e != nil {
		failure(w, e)
		return
	}
	if n == 0 {
		resource(w, 202, "payment-order", o.ID, o)
		return
	}
	checkout, e := provider.CreateTopUp(r.Context(), wallet.CheckoutRequest{OrderID: o.ID, UserID: u.UserID, Email: u.Email, Name: u.Name, Amount: o.Amount, Currency: "USD", SuccessURL: s.base.String() + "/portal/wallet?order=" + o.ID, CancelURL: s.base.String() + "/portal/wallet?order=" + o.ID, NotifyURL: s.base.String() + "/api/v1/payment-webhooks/" + o.Provider})
	if e != nil {
		s.Logger.Error().Err(e).Str("order", o.ID).Msg("checkout creation outcome requires reconciliation")
		failure(w, e)
		return
	}
	if e = s.Wallet.SetCheckout(r.Context(), o.ID, checkout); e != nil {
		failure(w, e)
		return
	}
	o, e = s.Wallet.Order(r.Context(), u.UserID, o.ID)
	if e != nil {
		failure(w, e)
		return
	}
	resource(w, 201, "payment-order", o.ID, o)
}
func (s *Server) purchase(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PlanID string `json:"plan_id"`
	}
	if !decode(w, r, &in) {
		return
	}
	u := current(r)
	var e error
	if s.Actions != nil {
		_, e = s.Actions.Execute(r.Context(), "billing", "purchase", runtime.ActionInput{Channel: "http", Auth: runtime.AuthContext{UserID: u.UserID, Role: u.Role}, Data: map[string]any{"plan_id": in.PlanID, "operation": r.Header.Get("Idempotency-Key")}})
	} else {
		e = s.Wallet.Purchase(r.Context(), u.UserID, in.PlanID, r.Header.Get("Idempotency-Key"), time.Now().UTC())
	}
	if e != nil {
		failure(w, e)
		return
	}
	s.account(w, r)
}
func (s *Server) renewal(w http.ResponseWriter, r *http.Request) {
	var in struct {
		AutoRenew  *bool  `json:"auto_renew"`
		NextPlanID string `json:"next_plan_id,omitempty"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.AutoRenew == nil {
		jsonapi.WriteValidationError(w, "auto_renew", "Choose a renewal preference.")
		return
	}
	e := s.Wallet.RenewalPreference(r.Context(), current(r).UserID, *in.AutoRenew, in.NextPlanID)
	if e != nil {
		failure(w, e)
		return
	}
	s.account(w, r)
}

type keyView = portal.APIKey

func viewKey(k key.Key) keyView {
	return keyView{ID: k.ID, Name: k.Name, Prefix: k.Prefix, Scopes: append([]string{}, k.Scopes...), CreatedAt: k.CreatedAt, LastUsed: k.LastUsed, ExpiresAt: k.ExpiresAt, RevokedAt: k.RevokedAt}
}
func (s *Server) keys(w http.ResponseWriter, r *http.Request) {
	keys, e := s.Keys.ListByUser(r.Context(), current(r).UserID)
	if e != nil {
		failure(w, e)
		return
	}
	out := []keyView{}
	for _, k := range keys {
		out = append(out, viewKey(k))
	}
	collection(w, "api-key", out)
}
func (s *Server) createKey(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name      string     `json:"name"`
		Scopes    []string   `json:"scopes"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.Name) < 1 || len(in.Name) > 120 || len(in.Scopes) > 50 || in.ExpiresAt != nil && !in.ExpiresAt.After(time.Now()) {
		jsonapi.WriteBadRequest(w, "Enter a name and a future expiry, if specified.")
		return
	}
	for _, scope := range in.Scopes {
		if scope != "*" && !strings.HasPrefix(scope, "/") {
			jsonapi.WriteBadRequest(w, "Scopes must be endpoint paths or *.")
			return
		}
	}
	raw, k := key.Generate("ak_", s.KeySecret)
	k.UserID = current(r).UserID
	k.Name = in.Name
	k.Scopes = in.Scopes
	k.ExpiresAt = in.ExpiresAt
	k.QuotaBypass = false
	if e := s.Keys.Create(r.Context(), k); e != nil {
		failure(w, e)
		return
	}
	v := viewKey(k)
	v.RawKey = raw
	resource(w, 201, "api-key", k.ID, v)
}
func (s *Server) revokeKey(w http.ResponseWriter, r *http.Request) {
	result, e := s.DB.ExecContext(r.Context(), "UPDATE api_keys SET revoked_at=CURRENT_TIMESTAMP WHERE id=? AND user_id=?", chi.URLParam(r, "id"), current(r).UserID)
	if e != nil {
		failure(w, e)
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		jsonapi.WriteNotFound(w, "api-key")
		return
	}
	jsonapi.WriteNoContent(w)
}
func (s *Server) usage(w http.ResponseWriter, r *http.Request) {
	id := current(r).UserID
	rows, e := s.DB.QueryContext(r.Context(), `SELECT to_char(timestamp AT TIME ZONE 'UTC','YYYY-MM-DD'),COUNT(*),COUNT(*) FILTER(WHERE status_code>=400),COALESCE(SUM(units),0),COALESCE(AVG(latency_ms),0) FROM usage_events WHERE user_id=? AND timestamp>CURRENT_TIMESTAMP-INTERVAL '30 days' GROUP BY 1 ORDER BY 1`, id)
	if e != nil {
		failure(w, e)
		return
	}
	defer rows.Close()
	out := []portal.UsageDay{}
	for rows.Next() {
		var d portal.UsageDay
		if e = rows.Scan(&d.ID, &d.Requests, &d.Errors, &d.Units, &d.LatencyMS); e != nil {
			failure(w, e)
			return
		}
		out = append(out, d)
	}
	if e = rows.Err(); e != nil {
		failure(w, e)
		return
	}
	collection(w, "usage-day", out)
}
func (s *Server) webhook(w http.ResponseWriter, r *http.Request) {
	registry, e := s.registry()
	if e != nil {
		failure(w, e)
		return
	}
	id := chi.URLParam(r, "provider")
	provider, ok := registry.CallbackProvider(id)
	if !ok {
		jsonapi.WriteNotFound(w, "provider")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()
	b, e := io.ReadAll(r.Body)
	if e != nil {
		jsonapi.WriteBadRequest(w, "Invalid callback body.")
		return
	}
	event, e := provider.VerifyPayment(r.Context(), b, r.Header)
	if errors.Is(e, payment.ErrIgnoredPayment) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
		return
	}
	if e != nil {
		jsonapi.WriteUnauthorized(w, "Invalid payment signature or payload.")
		return
	}
	if e = s.Wallet.ApplyPayment(r.Context(), id, event); e != nil {
		s.Logger.Error().Err(e).Str("provider", id).Str("event", event.EventID).Msg("payment callback rejected")
		failure(w, e)
		return
	}
	w.WriteHeader(200)
	w.Write([]byte("ok"))
}
