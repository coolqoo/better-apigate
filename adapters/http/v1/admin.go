package v1

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/coolqoo/better-apigate/adapters/payment"
	"github.com/coolqoo/better-apigate/adapters/postgres"
	"github.com/coolqoo/better-apigate/domain/portal"
	"github.com/coolqoo/better-apigate/domain/settings"
	"github.com/coolqoo/better-apigate/domain/wallet"
	"github.com/coolqoo/better-apigate/pkg/jsonapi"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func audit(r *http.Request, tx *postgres.Tx, action, target, reason string) error {
	_, e := tx.ExecContext(r.Context(), "INSERT INTO admin_audit(id,actor_id,action,target_id,reason) VALUES(?,?,?,?,?)", uuid.NewString(), current(r).UserID, action, target, reason)
	return e
}
func (s *Server) adminOverview(w http.ResponseWriter, r *http.Request) {
	var customers, requests, errorsCount, pending int64
	var balance, credits wallet.Money
	e := s.DB.QueryRowContext(r.Context(), `SELECT (SELECT COUNT(*) FROM users WHERE role='user'),(SELECT COUNT(*) FROM usage_events WHERE timestamp>CURRENT_TIMESTAMP-INTERVAL '24 hours'),(SELECT COUNT(*) FROM usage_events WHERE timestamp>CURRENT_TIMESTAMP-INTERVAL '24 hours' AND status_code>=400),(SELECT COUNT(*) FROM usage_reservations WHERE state='pending'),(SELECT COALESCE(SUM(balance_micros),0) FROM wallets),(SELECT COALESCE(SUM(amount_micros),0) FROM wallet_ledger WHERE kind='top_up' AND created_at>CURRENT_TIMESTAMP-INTERVAL '30 days')`).Scan(&customers, &requests, &errorsCount, &pending, &balance, &credits)
	if e != nil {
		failure(w, e)
		return
	}
	resource(w, 200, "admin-overview", "overview", map[string]any{"customers": customers, "requests": requests, "errors": errorsCount, "pending_reservations": pending, "wallet_liability": balance, "top_up_volume": credits})
}
func (s *Server) customers(w http.ResponseWriter, r *http.Request) {
	page, ok := requestPagination(w, r)
	if !ok {
		return
	}
	search := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(search) > 200 {
		jsonapi.WriteBadRequest(w, "Use a customer search of at most 200 characters.")
		return
	}
	const filter = "u.role='user' AND POSITION(LOWER(?) IN LOWER(u.email || ' ' || u.name))>0"
	if err := s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM users u WHERE "+filter, search).Scan(&page.Total); err != nil {
		failure(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT u.id,u.name,u.email,u.role,u.status,u.plan_id,u.email_verified,u.created_at,COALESCE(w.balance_micros,0),COALESCE(w.shortfall_micros,0),COALESCE(w.frozen,FALSE) FROM users u LEFT JOIN wallets w ON w.user_id=u.id WHERE `+filter+` ORDER BY u.created_at DESC,u.id DESC LIMIT ? OFFSET ?`, search, page.Limit(), page.Offset())
	if err != nil {
		failure(w, err)
		return
	}
	defer rows.Close()
	out := []portal.Customer{}
	for rows.Next() {
		var customer portal.Customer
		if err = rows.Scan(&customer.ID, &customer.Name, &customer.Email, &customer.Role, &customer.Status, &customer.PlanID, &customer.EmailVerified, &customer.CreatedAt, &customer.Balance, &customer.Shortfall, &customer.Frozen); err != nil {
			failure(w, err)
			return
		}
		out = append(out, customer)
	}
	if err = rows.Err(); err != nil {
		failure(w, err)
		return
	}
	collectionPage(w, "customer", out, page)
}
func (s *Server) customerState(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if !decode(w, r, &in) {
		return
	}
	id := chi.URLParam(r, "id")
	if (in.Status != "active" && in.Status != "suspended") || strings.TrimSpace(in.Reason) == "" || id == current(r).UserID {
		jsonapi.WriteBadRequest(w, "Choose active or suspended and provide a reason. You cannot suspend yourself.")
		return
	}
	tx, e := s.DB.BeginTx(r.Context(), nil)
	if e != nil {
		failure(w, e)
		return
	}
	defer tx.Rollback()
	result, e := tx.ExecContext(r.Context(), "UPDATE users SET status=?,updated_at=CURRENT_TIMESTAMP WHERE id=? AND role='user'", in.Status, id)
	if e == nil {
		n, _ := result.RowsAffected()
		if n == 0 {
			e = sql.ErrNoRows
		}
	}
	if e == nil {
		e = audit(r, tx, "customer."+in.Status, id, in.Reason)
	}
	if e == nil {
		e = tx.Commit()
	}
	if e != nil {
		failure(w, e)
		return
	}
	jsonapi.WriteNoContent(w)
}
func (s *Server) customerWallet(w http.ResponseWriter, r *http.Request) {
	a, e := s.Wallet.Account(r.Context(), chi.URLParam(r, "id"))
	if e != nil {
		failure(w, e)
		return
	}
	resource(w, 200, "wallet", a.UserID, a)
}
func (s *Server) adjust(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Amount    wallet.Money `json:"amount"`
		Direction string       `json:"direction"`
		Reason    string       `json:"reason"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Amount <= 0 || (in.Direction != "credit" && in.Direction != "debit") || strings.TrimSpace(in.Reason) == "" {
		jsonapi.WriteBadRequest(w, "Enter a positive amount, credit or debit, and an audit reason.")
		return
	}
	amount := in.Amount
	if in.Direction == "debit" {
		amount = -amount
	}
	e := s.Wallet.Adjust(r.Context(), chi.URLParam(r, "id"), amount, r.Header.Get("Idempotency-Key"), current(r).UserID, in.Reason)
	if e != nil {
		failure(w, e)
		return
	}
	s.customerWallet(w, r)
}
func (s *Server) unfreeze(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Reason string `json:"reason"`
	}
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Reason) == "" {
		jsonapi.WriteBadRequest(w, "An audit reason is required.")
		return
	}
	id := chi.URLParam(r, "id")
	tx, e := s.DB.BeginTx(r.Context(), nil)
	if e != nil {
		failure(w, e)
		return
	}
	defer tx.Rollback()
	var shortfall wallet.Money
	e = tx.QueryRowContext(r.Context(), "SELECT shortfall_micros FROM wallets WHERE user_id=? FOR UPDATE", id).Scan(&shortfall)
	if e != nil {
		failure(w, e)
		return
	}
	if shortfall > 0 {
		jsonapi.WriteConflict(w, "Cover the recorded shortfall before restoring access.")
		return
	}
	_, e = tx.ExecContext(r.Context(), "UPDATE wallets SET frozen=FALSE WHERE user_id=?", id)
	if e == nil {
		e = audit(r, tx, "wallet.unfreeze", id, in.Reason)
	}
	if e == nil {
		e = tx.Commit()
	}
	if e != nil {
		failure(w, e)
		return
	}
	jsonapi.WriteNoContent(w)
}
func (s *Server) adminOrders(w http.ResponseWriter, r *http.Request) {
	page, ok := requestPagination(w, r)
	if !ok {
		return
	}
	if err := s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM payment_orders").Scan(&page.Total); err != nil {
		failure(w, err)
		return
	}
	out, err := s.Wallet.OrdersPage(r.Context(), "", page.Limit(), page.Offset())
	if err != nil {
		failure(w, err)
		return
	}
	collectionPage(w, "payment-order", out, page)
}
func (s *Server) reconcile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var userID string
	if e := s.DB.QueryRowContext(r.Context(), "SELECT user_id FROM payment_orders WHERE id=?", id).Scan(&userID); e != nil {
		failure(w, e)
		return
	}
	o, e := s.Wallet.Order(r.Context(), userID, id)
	if e != nil {
		failure(w, e)
		return
	}
	registry, e := s.registry()
	if e != nil {
		failure(w, e)
		return
	}
	provider, ok := registry.CallbackProvider(o.Provider)
	if !ok {
		jsonapi.WriteBadRequest(w, "Enable this order's provider to reconcile it.")
		return
	}
	event, e := provider.LookupPayment(r.Context(), o)
	if e != nil {
		jsonapi.WriteConflict(w, "The provider cannot verify this payment yet. Await a signed callback or check the merchant dashboard.")
		return
	}
	if e = s.Wallet.ApplyPayment(r.Context(), o.Provider, event); e != nil {
		failure(w, e)
		return
	}
	o, e = s.Wallet.Order(r.Context(), userID, id)
	if e != nil {
		failure(w, e)
		return
	}
	resource(w, 200, "payment-order", id, o)
}
func (s *Server) pending(w http.ResponseWriter, r *http.Request) {
	page, ok := requestPagination(w, r)
	if !ok {
		return
	}
	if err := s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM usage_reservations WHERE state='pending' AND created_at<CURRENT_TIMESTAMP-INTERVAL '5 minutes'").Scan(&page.Total); err != nil {
		failure(w, err)
		return
	}
	out, err := s.Wallet.PendingPage(r.Context(), page.Limit(), page.Offset())
	if err != nil {
		failure(w, err)
		return
	}
	collectionPage(w, "usage-reservation", out, page)
}
func (s *Server) resolve(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Charge *bool  `json:"charge"`
		Reason string `json:"reason"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Charge == nil || strings.TrimSpace(in.Reason) == "" {
		jsonapi.WriteBadRequest(w, "Choose charge or release and provide verified evidence in the audit reason.")
		return
	}
	e := s.Wallet.Resolve(r.Context(), chi.URLParam(r, "id"), *in.Charge, current(r).UserID, in.Reason)
	if e != nil {
		failure(w, e)
		return
	}
	jsonapi.WriteNoContent(w)
}
func (s *Server) adminPlans(w http.ResponseWriter, r *http.Request) {
	rows, e := s.DB.QueryContext(r.Context(), "SELECT id,name,COALESCE(description,''),price_micros,unit_price_micros,included_units,rate_limit_per_minute,term_days,is_default,enabled FROM plans ORDER BY price_micros,id")
	if e != nil {
		failure(w, e)
		return
	}
	defer rows.Close()
	out := []wallet.Plan{}
	for rows.Next() {
		var p wallet.Plan
		if e = rows.Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.UnitPrice, &p.IncludedUnits, &p.RateLimit, &p.TermDays, &p.IsBase, &p.Enabled); e != nil {
			failure(w, e)
			return
		}
		out = append(out, p)
	}
	if e = rows.Err(); e != nil {
		failure(w, e)
		return
	}
	collection(w, "plan", out)
}
func flag(v bool) int {
	if v {
		return 1
	}
	return 0
}
func (s *Server) savePlan(w http.ResponseWriter, r *http.Request) {
	var p wallet.Plan
	if !decode(w, r, &p) {
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		id = uuid.NewString()
	} else if p.ID != "" && p.ID != id {
		jsonapi.WriteBadRequest(w, "Plan ID mismatch.")
		return
	}
	if len(p.Name) < 1 || len(p.Name) > 120 || p.IncludedUnits < 0 || p.Price < 0 || p.UnitPrice < 0 || p.RateLimit < 1 || p.TermDays != 30 || p.IsBase && (!p.Enabled || p.Price != 0 || p.IncludedUnits != 0) || !p.IsBase && p.Price == 0 {
		jsonapi.WriteBadRequest(w, "Use a 30-day plan, positive rate limit, and nonnegative pricing. Paid plans require a positive price.")
		return
	}
	tx, e := s.DB.BeginTx(r.Context(), nil)
	if e != nil {
		failure(w, e)
		return
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(r.Context(), "SELECT pg_advisory_xact_lock(734018923)"); e != nil {
		failure(w, e)
		return
	}
	var existingBase bool
	e = tx.QueryRowContext(r.Context(), "SELECT is_default=1 FROM plans WHERE id=?", id).Scan(&existingBase)
	if e != nil && e != sql.ErrNoRows {
		failure(w, e)
		return
	}
	if existingBase && !p.IsBase {
		jsonapi.WriteBadRequest(w, "The base plan must remain enabled. Select a different base plan first.")
		return
	}
	if p.IsBase {
		_, e = tx.ExecContext(r.Context(), "UPDATE plans SET is_default=0 WHERE id<>?", id)
		if e != nil {
			failure(w, e)
			return
		}
	}
	_, e = tx.ExecContext(r.Context(), `INSERT INTO plans(id,name,description,price_micros,unit_price_micros,included_units,rate_limit_per_minute,term_days,is_default,enabled) VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET name=EXCLUDED.name,description=EXCLUDED.description,price_micros=EXCLUDED.price_micros,unit_price_micros=EXCLUDED.unit_price_micros,included_units=EXCLUDED.included_units,rate_limit_per_minute=EXCLUDED.rate_limit_per_minute,term_days=EXCLUDED.term_days,is_default=EXCLUDED.is_default,enabled=EXCLUDED.enabled`, id, p.Name, p.Description, p.Price, p.UnitPrice, p.IncludedUnits, p.RateLimit, p.TermDays, flag(p.IsBase), flag(p.Enabled))
	if e == nil {
		e = audit(r, tx, "plan.configure", id, "Configured plan pricing")
	}
	if e == nil {
		e = tx.Commit()
	}
	if e == nil && s.OnConfigChange != nil {
		e = s.OnConfigChange(r.Context())
	}
	if e != nil {
		failure(w, e)
		return
	}
	p.ID = id
	resource(w, 200, "plan", id, p)
}
func allowedSetting(k string) bool {
	for _, prefix := range []string{"payment.", "email.", "portal.app_name", "auth.require_verification", "billing.top_up_amounts", "upstream.", "tls.", "cors."} {
		if strings.HasPrefix(k, prefix) {
			return true
		}
	}
	return false
}
func secretSetting(k string) bool {
	return settings.IsSensitive(k) || strings.HasSuffix(k, "secret_key") || strings.HasSuffix(k, "api_key") || strings.HasSuffix(k, "webhook_secret") || strings.HasSuffix(k, "password")
}
func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{}
	for k, v := range s.Settings.Get() {
		if allowedSetting(k) {
			if secretSetting(k) && v != "" {
				v = "••••••"
			}
			out[k] = v
		}
	}
	resource(w, 200, "settings", "configuration", map[string]any{"values": out})
}
func (s *Server) saveSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Values map[string]string `json:"values"`
	}
	if !decode(w, r, &in) {
		return
	}
	combined := s.Settings.Get()
	batch := settings.Settings{}
	for k, v := range in.Values {
		if !allowedSetting(k) || len(v) > 4096 {
			jsonapi.WriteBadRequest(w, "This setting cannot be changed through this interface.")
			return
		}
		if secretSetting(k) && v == "••••••" {
			continue
		}
		if k == "billing.top_up_amounts" {
			amounts := strings.Split(v, ",")
			seen := make(map[wallet.Money]bool)
			if len(amounts) > 12 {
				jsonapi.WriteValidationError(w, k, "Choose up to 12 positive USD amounts.")
				return
			}
			for _, amount := range amounts {
				money, err := wallet.ParseMoney(strings.TrimSpace(amount))
				if err != nil || money <= 0 || int64(money)%10_000 != 0 || seen[money] {
					jsonapi.WriteValidationError(w, k, "Use distinct positive USD amounts with at most two decimal places, separated by commas.")
					return
				}
				seen[money] = true
			}
		}
		combined[k] = v
		batch[k] = v
	}
	if _, e := payment.NewRegistry(combined); e != nil {
		jsonapi.WriteValidationError(w, "providers", e.Error())
		return
	}
	if e := s.Settings.SetBatch(r.Context(), batch); e != nil {
		failure(w, e)
		return
	}
	s.getSettings(w, r)
}

func (s *Server) reverseOrder(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Amount wallet.Money `json:"amount"`
		Reason string       `json:"reason"`
	}
	if !decode(w, r, &in) {
		return
	}
	operation := r.Header.Get("Idempotency-Key")
	id := chi.URLParam(r, "id")
	if err := s.Wallet.ReverseOrder(r.Context(), id, in.Amount, operation, current(r).UserID, in.Reason); err != nil {
		failure(w, err)
		return
	}
	var userID string
	if err := s.DB.QueryRowContext(r.Context(), "SELECT user_id FROM payment_orders WHERE id=?", id).Scan(&userID); err != nil {
		failure(w, err)
		return
	}
	order, err := s.Wallet.Order(r.Context(), userID, id)
	if err != nil {
		failure(w, err)
		return
	}
	resource(w, 200, "payment-order", id, order)
}
