package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/coolqoo/better-apigate/domain/usage"
	"github.com/coolqoo/better-apigate/domain/wallet"
	"github.com/coolqoo/better-apigate/ports"
	"github.com/google/uuid"
)

type WalletStore struct {
	db      *DB
	queueMu sync.Mutex
	queues  map[string]*accountQueue
}
type accountQueue struct {
	token   chan struct{}
	waiters int
}

// Bound contention within this process; PostgreSQL still serializes every
// financial transaction across gateway instances. Entries disappear when idle.
func (s *WalletStore) admitTransaction(ctx context.Context, userID string) (func(), error) {
	s.queueMu.Lock()
	if s.queues == nil {
		s.queues = make(map[string]*accountQueue)
	}
	q := s.queues[userID]
	if q == nil {
		q = &accountQueue{token: make(chan struct{}, 1)}
		s.queues[userID] = q
	}
	q.waiters++
	s.queueMu.Unlock()
	leave := func() {
		s.queueMu.Lock()
		q.waiters--
		if q.waiters == 0 {
			delete(s.queues, userID)
		}
		s.queueMu.Unlock()
	}
	select {
	case q.token <- struct{}{}:
		return func() { <-q.token; leave() }, nil
	case <-ctx.Done():
		leave()
		return nil, ctx.Err()
	}
}

func NewWalletStore(db *DB) *WalletStore { return &WalletStore{db: db} }

var _ ports.PrepaidStore = (*WalletStore)(nil)

const planColumns = "id,name,COALESCE(description,''),price_micros,unit_price_micros,included_units,rate_limit_per_minute,term_days,is_default,enabled"
const termColumns = "id,plan_id,plan_name,price_micros,unit_price_micros,included_units,used_units,reserved_units,rate_limit_per_minute,starts_at,ends_at"
const orderColumns = "id,user_id,provider,amount_micros,currency,CASE WHEN state='pending' AND expires_at<=CURRENT_TIMESTAMP THEN 'expired' ELSE state END,COALESCE(provider_id,''),checkout_url,crypto_amount,crypto_token,expires_at,created_at"

type scanner interface{ Scan(...any) error }

func scanPlan(row scanner) (wallet.Plan, error) {
	var p wallet.Plan
	err := row.Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.UnitPrice, &p.IncludedUnits, &p.RateLimit, &p.TermDays, &p.IsBase, &p.Enabled)
	return p, err
}
func scanTerm(row scanner) (*wallet.Term, error) {
	var t wallet.Term
	err := row.Scan(&t.ID, &t.PlanID, &t.PlanName, &t.Price, &t.UnitPrice, &t.IncludedUnits, &t.UsedUnits, &t.ReservedUnits, &t.RateLimit, &t.StartsAt, &t.EndsAt)
	return &t, err
}
func scanOrder(row scanner) (wallet.Order, error) {
	var o wallet.Order
	err := row.Scan(&o.ID, &o.UserID, &o.Provider, &o.Amount, &o.Currency, &o.State, &o.ProviderID, &o.CheckoutURL, &o.CryptoAmount, &o.CryptoToken, &o.ExpiresAt, &o.CreatedAt)
	return o, err
}

// Account creates only an empty wallet; it never grants credit or quota.
func (s *WalletStore) Account(ctx context.Context, userID string) (wallet.Account, error) {
	if _, err := s.db.ExecContext(ctx, "INSERT INTO wallets(user_id) VALUES(?) ON CONFLICT DO NOTHING", userID); err != nil {
		return wallet.Account{}, err
	}
	var a wallet.Account
	a.UserID = userID
	a.Currency = "USD"
	err := s.db.QueryRowContext(ctx, "SELECT balance_micros,reserved_micros,shortfall_micros,frozen,auto_renew,next_plan_id FROM wallets WHERE user_id=?", userID).Scan(&a.Balance, &a.Reserved, &a.Shortfall, &a.Frozen, &a.AutoRenew, &a.NextPlanID)
	if err != nil {
		return a, err
	}
	a.Available = a.Balance - a.Reserved
	t, e := scanTerm(s.db.QueryRowContext(ctx, "SELECT "+termColumns+" FROM plan_terms WHERE user_id=? ORDER BY ends_at DESC LIMIT 1", userID))
	if e == nil && t.EndsAt.After(time.Now()) {
		a.Term = t
	} else if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return a, e
	}
	if a.Term != nil && a.AutoRenew {
		planID := a.NextPlanID
		if planID == "" {
			planID = a.Term.PlanID
		}
		var price wallet.Money
		err = s.db.QueryRowContext(ctx, "SELECT price_micros FROM plans WHERE id=? AND enabled=1 AND is_default=0", planID).Scan(&price)
		if err == nil {
			a.RenewalPrice = &price
		} else if !errors.Is(err, sql.ErrNoRows) {
			return a, err
		}
	}
	return a, nil
}
func (s *WalletStore) Plans(ctx context.Context) ([]wallet.Plan, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+planColumns+" FROM plans WHERE enabled=1 ORDER BY price_micros,id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	plans := []wallet.Plan{}
	for rows.Next() {
		p, e := scanPlan(rows)
		if e != nil {
			return nil, e
		}
		plans = append(plans, p)
	}
	return plans, rows.Err()
}
func lockWallet(ctx context.Context, tx *Tx, userID string) (wallet.Account, error) {
	a := wallet.Account{UserID: userID, Currency: "USD"}
	err := tx.QueryRowContext(ctx, "SELECT balance_micros,reserved_micros,shortfall_micros,frozen,auto_renew,next_plan_id FROM wallets WHERE user_id=? FOR UPDATE", userID).Scan(&a.Balance, &a.Reserved, &a.Shortfall, &a.Frozen, &a.AutoRenew, &a.NextPlanID)
	if errors.Is(err, sql.ErrNoRows) {
		if _, err = tx.ExecContext(ctx, "INSERT INTO wallets(user_id) VALUES(?) ON CONFLICT DO NOTHING", userID); err != nil {
			return a, err
		}
		err = tx.QueryRowContext(ctx, "SELECT balance_micros,reserved_micros,shortfall_micros,frozen,auto_renew,next_plan_id FROM wallets WHERE user_id=? FOR UPDATE", userID).Scan(&a.Balance, &a.Reserved, &a.Shortfall, &a.Frozen, &a.AutoRenew, &a.NextPlanID)
	}
	a.Available = a.Balance - a.Reserved
	return a, err
}
func appendLedger(ctx context.Context, tx *Tx, userID, kind, ref, description, actor string, amount, balance wallet.Money) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO wallet_ledger(id,user_id,kind,amount_micros,balance_micros,reference,description,actor_id) VALUES(?,?,?,?,?,?,?,?)", uuid.NewString(), userID, kind, amount, balance, ref, description, actor)
	if err != nil {
		return err
	}
	if kind != "usage_charged" && kind != "usage_released" {
		eventType := "wallet.adjusted"
		switch kind {
		case "top_up":
			eventType = "payment.success"
		case "refund", "reversal":
			eventType = "payment.reversed"
		case "plan_purchase":
			eventType = "subscription.start"
			if strings.HasPrefix(ref, "renew:") {
				eventType = "subscription.renew"
			}
		case "plan_scheduled":
			eventType = "plan.changed"
		}
		return queueNotification(ctx, tx, userID, eventType, ref, map[string]any{"amount": amount, "balance": balance, "description": description})
	}
	return nil
}
func latestTerm(ctx context.Context, tx *Tx, userID string) (*wallet.Term, bool, error) {
	var t wallet.Term
	var attempted bool
	err := tx.QueryRowContext(ctx, "SELECT "+termColumns+",renewal_attempted FROM plan_terms WHERE user_id=? ORDER BY ends_at DESC LIMIT 1 FOR UPDATE", userID).Scan(&t.ID, &t.PlanID, &t.PlanName, &t.Price, &t.UnitPrice, &t.IncludedUnits, &t.UsedUnits, &t.ReservedUnits, &t.RateLimit, &t.StartsAt, &t.EndsAt, &attempted)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	return &t, attempted, err
}
func basePlan(ctx context.Context, tx *Tx) (wallet.Plan, error) {
	return scanPlan(tx.QueryRowContext(ctx, "SELECT "+planColumns+" FROM plans WHERE is_default=1 AND enabled=1 AND price_micros=0 ORDER BY id LIMIT 1"))
}
func createTerm(ctx context.Context, tx *Tx, a *wallet.Account, p wallet.Plan, start time.Time, ref string) (*wallet.Term, error) {
	if p.Price <= 0 || a.Available < p.Price {
		return nil, wallet.ErrInsufficient
	}
	t := &wallet.Term{ID: uuid.NewString(), PlanID: p.ID, PlanName: p.Name, Price: p.Price, UnitPrice: p.UnitPrice, IncludedUnits: p.IncludedUnits, RateLimit: p.RateLimit, StartsAt: start, EndsAt: start.Add(time.Duration(p.TermDays) * 24 * time.Hour)}
	if _, err := tx.ExecContext(ctx, "INSERT INTO plan_terms(id,user_id,plan_id,plan_name,price_micros,unit_price_micros,included_units,rate_limit_per_minute,starts_at,ends_at) VALUES(?,?,?,?,?,?,?,?,?,?)", t.ID, a.UserID, t.PlanID, t.PlanName, t.Price, t.UnitPrice, t.IncludedUnits, t.RateLimit, t.StartsAt, t.EndsAt); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE wallets SET balance_micros=balance_micros-?,auto_renew=TRUE,next_plan_id='',updated_at=CURRENT_TIMESTAMP WHERE user_id=?", p.Price, a.UserID); err != nil {
		return nil, err
	}
	a.Balance -= p.Price
	a.Available -= p.Price
	if err := appendLedger(ctx, tx, a.UserID, "plan_purchase", ref, p.Name+" prepaid term", "", -p.Price, a.Balance); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE users SET plan_id=? WHERE id=?", p.ID, a.UserID); err != nil {
		return nil, err
	}
	return t, nil
}

// resolveTerm performs the single renewal attempt, under the wallet lock.
func resolveTerm(ctx context.Context, tx *Tx, a *wallet.Account, now time.Time) (*wallet.Term, wallet.Plan, error) {
	t, attempted, err := latestTerm(ctx, tx, a.UserID)
	if err != nil {
		return nil, wallet.Plan{}, err
	}
	if t != nil && t.EndsAt.After(now) {
		return t, wallet.Plan{ID: t.PlanID, Name: t.PlanName, UnitPrice: t.UnitPrice, RateLimit: t.RateLimit}, nil
	}
	if t != nil && !attempted {
		if _, err = tx.ExecContext(ctx, "UPDATE plan_terms SET renewal_attempted=TRUE WHERE id=?", t.ID); err != nil {
			return nil, wallet.Plan{}, err
		}
		if a.AutoRenew && !a.Frozen {
			target := t.PlanID
			if a.NextPlanID != "" {
				target = a.NextPlanID
			}
			p, e := scanPlan(tx.QueryRowContext(ctx, "SELECT "+planColumns+" FROM plans WHERE id=? AND enabled=1", target))
			if e != nil && !errors.Is(e, sql.ErrNoRows) {
				return nil, wallet.Plan{}, e
			}
			if e == nil {
				// Every new term snapshots the current catalog. An existing term
				// keeps its purchased prices and quota until it expires.
				if p.Price >= 0 && a.Available >= p.Price {
					next, e := createTerm(ctx, tx, a, p, now, "renew:"+t.ID)
					return next, p, e
				}
			}
		}
		if err = queueNotification(ctx, tx, a.UserID, "subscription.end", "expiry:"+t.ID, map[string]any{"plan_id": t.PlanID, "auto_renew": a.AutoRenew, "description": "The term expired. Paid usage now uses the base pay-as-you-go plan; reactivation requires an explicit purchase."}); err != nil {
			return nil, wallet.Plan{}, err
		}
	}
	p, err := basePlan(ctx, tx)
	if err != nil {
		return nil, p, err
	}
	if t != nil {
		if _, err = tx.ExecContext(ctx, "UPDATE users SET plan_id=? WHERE id=? AND plan_id<>?", p.ID, a.UserID, p.ID); err != nil {
			return nil, p, err
		}
	}
	return nil, p, nil
}
func (s *WalletStore) Reserve(ctx context.Context, userID, keyID, routeID string, units int64, now time.Time) (wallet.Reservation, error) {
	if units <= 0 {
		return wallet.Reservation{}, wallet.ErrInvalid
	}
	leave, err := s.admitTransaction(ctx, userID)
	if err != nil {
		return wallet.Reservation{}, err
	}
	defer leave()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return wallet.Reservation{}, err
	}
	defer tx.Rollback()
	a, err := lockWallet(ctx, tx, userID)
	if err != nil {
		return wallet.Reservation{}, err
	}
	if a.Frozen || a.Shortfall > 0 {
		return wallet.Reservation{}, wallet.ErrFrozen
	}
	var active bool
	if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE id=? AND status='active')", userID).Scan(&active); err != nil {
		return wallet.Reservation{}, err
	}
	if !active {
		return wallet.Reservation{}, wallet.ErrFrozen
	}
	if keyID != "" {
		if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM api_keys WHERE id=? AND user_id=? AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at>?))", keyID, userID, now).Scan(&active); err != nil {
			return wallet.Reservation{}, err
		}
		if !active {
			return wallet.Reservation{}, wallet.ErrFrozen
		}
	}
	t, p, err := resolveTerm(ctx, tx, &a, now)
	if err != nil {
		return wallet.Reservation{}, err
	}
	r := wallet.Reservation{ID: uuid.NewString(), UserID: userID, KeyID: keyID, RouteID: routeID, Units: units, UnitPrice: p.UnitPrice, State: "pending", CreatedAt: now, PlanID: p.ID, RateLimit: p.RateLimit}
	if t != nil {
		r.TermID = t.ID
		r.QuotaUnits = min(units, t.IncludedUnits-t.UsedUnits-t.ReservedUnits)
	}
	remaining := units - r.QuotaUnits
	// A zero priced base explicitly permits free usage; paid terms must have
	// an overage price to allow calls beyond their included quota.
	if remaining > 0 && t != nil && p.UnitPrice == 0 {
		if err = tx.Commit(); err != nil {
			return r, err
		}
		return r, wallet.ErrInsufficient
	}
	r.Amount, err = wallet.Cost(remaining, p.UnitPrice)
	if err != nil {
		return r, err
	}
	if a.Available < r.Amount {
		if err = tx.Commit(); err != nil {
			return r, err
		}
		return r, wallet.ErrInsufficient
	}
	if _, err = tx.ExecContext(ctx, `WITH held_wallet AS (
		UPDATE wallets SET reserved_micros=reserved_micros+? WHERE user_id=?
	), held_quota AS (
		UPDATE plan_terms SET reserved_units=reserved_units+? WHERE id=?
	) INSERT INTO usage_reservations(id,user_id,key_id,route_id,term_id,units,quota_units,amount_micros,unit_price_micros,created_at)
	VALUES(?,?,?,?,?,?,?,?,?,?)`, r.Amount, userID, r.QuotaUnits, nullString(r.TermID), r.ID, userID, keyID, routeID, nullString(r.TermID), units, r.QuotaUnits, r.Amount, r.UnitPrice, now); err != nil {
		return r, err
	}
	return r, tx.Commit()
}
func scanReservation(row scanner) (wallet.Reservation, error) {
	var r wallet.Reservation
	err := row.Scan(&r.ID, &r.UserID, &r.KeyID, &r.RouteID, &r.TermID, &r.Units, &r.QuotaUnits, &r.Amount, &r.UnitPrice, &r.State, &r.CreatedAt)
	return r, err
}

const reservationColumns = "id,user_id,key_id,route_id,COALESCE(term_id,''),units,quota_units,amount_micros,unit_price_micros,state,created_at"

func (s *WalletStore) Settle(ctx context.Context, id string, charge bool, event usage.Event) error {
	return s.settle(ctx, id, charge, event, "", "")
}
func (s *WalletStore) settle(ctx context.Context, id string, charge bool, event usage.Event, actor, reason string) error {
	// Find ownership before locking, then always acquire wallet before reservation.
	var userID string
	if err := s.db.QueryRowContext(ctx, "SELECT user_id FROM usage_reservations WHERE id=?", id).Scan(&userID); err != nil {
		return err
	}
	leave, err := s.admitTransaction(ctx, userID)
	if err != nil {
		return err
	}
	defer leave()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	a, err := lockWallet(ctx, tx, userID)
	if err != nil {
		return err
	}
	r, err := scanReservation(tx.QueryRowContext(ctx, "SELECT "+reservationColumns+" FROM usage_reservations WHERE id=? FOR UPDATE", id))
	if err != nil {
		return err
	}
	target := "released"
	if charge {
		target = "charged"
	}
	if r.State != "pending" {
		if r.State == target {
			return tx.Commit()
		}
		return wallet.ErrConflict
	}
	amount := wallet.Money(0)
	used := int64(0)
	if charge {
		amount = r.Amount
		used = r.QuotaUnits
	}
	coverage := wallet.Money(0)
	if !charge && a.Shortfall > 0 {
		coverage = min(a.Available+r.Amount, a.Shortfall)
	}
	if reason == "" {
		reason = "API usage"
	}
	if charge && a.Balance-amount == 0 && amount > 0 {
		if err = queueNotification(ctx, tx, userID, "wallet.exhausted", "exhausted:"+id, map[string]any{"available": "0.000000"}); err != nil {
			return err
		}
	}
	event.ID = id
	event.UserID = userID
	event.KeyID = r.KeyID
	event.Units = r.Units
	event.RouteID = r.RouteID
	event.CostMultiplier = float64(r.Units)
	event.Source = usage.SourceProxy
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `WITH changed_wallet AS (
		UPDATE wallets SET reserved_micros=reserved_micros-?,balance_micros=balance_micros-?,shortfall_micros=shortfall_micros-?,updated_at=CURRENT_TIMESTAMP WHERE user_id=?
	), changed_quota AS (
		UPDATE plan_terms SET reserved_units=reserved_units-?,used_units=used_units+? WHERE id=?
	), changed_reservation AS (
		UPDATE usage_reservations SET state=?,settled_at=CURRENT_TIMESTAMP WHERE id=?
	), ledger_entry AS (
		INSERT INTO wallet_ledger(id,user_id,kind,amount_micros,balance_micros,reference,description,actor_id) VALUES(?,?,?,?,?,?,?,?)
	) INSERT INTO billing_outbox(id,event_type,payload) VALUES(?,'usage.settled',?::jsonb)`,
		r.Amount, amount+coverage, coverage, userID, r.QuotaUnits, used, nullString(r.TermID), target, id,
		uuid.NewString(), userID, "usage_"+target, -amount, a.Balance-amount-coverage, "usage:"+id, reason, actor, id, string(payload)); err != nil {
		return err
	}
	return tx.Commit()
}

// claimOperation binds a retry key to its original financial intent. The wallet
// lock serializes this check with the corresponding mutation in the same transaction.
func claimOperation(ctx context.Context, tx *Tx, userID, operation, fingerprint string) (bool, error) {
	var prior string
	err := tx.QueryRowContext(ctx, "SELECT fingerprint FROM financial_operations WHERE user_id=? AND operation=?", userID, operation).Scan(&prior)
	if err == nil {
		if prior != fingerprint {
			return false, wallet.ErrConflict
		}
		return true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO financial_operations(user_id,operation,fingerprint) VALUES(?,?,?)", userID, operation, fingerprint)
	return false, err
}

func (s *WalletStore) Purchase(ctx context.Context, userID, planID, operation string, now time.Time) error {
	if operation == "" || len(operation) > 128 {
		return wallet.ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	a, err := lockWallet(ctx, tx, userID)
	if err != nil {
		return err
	}
	if a.Frozen {
		return wallet.ErrFrozen
	}
	ref := "purchase:" + userID + ":" + operation
	done, err := claimOperation(ctx, tx, userID, "purchase:"+operation, planID)
	if err != nil {
		return err
	}
	if done {
		return tx.Commit()
	}
	p, err := scanPlan(tx.QueryRowContext(ctx, "SELECT "+planColumns+" FROM plans WHERE id=? AND enabled=1 AND is_default=0", planID))
	if err != nil {
		return err
	}
	current, _, err := latestTerm(ctx, tx, userID)
	if err != nil {
		return err
	}
	if current != nil && current.EndsAt.After(now) {
		if _, err = tx.ExecContext(ctx, "UPDATE wallets SET next_plan_id=? WHERE user_id=?", p.ID, userID); err != nil {
			return err
		}
		if err = appendLedger(ctx, tx, userID, "plan_scheduled", ref, "Plan change at next renewal: "+p.Name, "", 0, a.Balance); err != nil {
			return err
		}
	} else {
		if current != nil {
			if _, err = tx.ExecContext(ctx, "UPDATE plan_terms SET renewal_attempted=TRUE WHERE id=?", current.ID); err != nil {
				return err
			}
		}
		if _, err = createTerm(ctx, tx, &a, p, now, ref); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *WalletStore) RenewalPreference(ctx context.Context, userID string, enabled bool, nextPlan string) error {
	if nextPlan != "" {
		var ok bool
		if err := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM plans WHERE id=? AND enabled=1 AND is_default=0)", nextPlan).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return wallet.ErrInvalid
		}
	}
	if _, err := s.Account(ctx, userID); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, "UPDATE wallets SET auto_renew=?,next_plan_id=?,updated_at=CURRENT_TIMESTAMP WHERE user_id=?", enabled, nextPlan, userID)
	return err
}
func (s *WalletStore) Ledger(ctx context.Context, userID string, limit, offset int) ([]wallet.LedgerEntry, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,user_id,kind,amount_micros,balance_micros,reference,description,created_at FROM wallet_ledger WHERE user_id=? ORDER BY created_at DESC,id DESC LIMIT ? OFFSET ?", userID, min(max(limit, 1), 100), max(offset, 0))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := []wallet.LedgerEntry{}
	for rows.Next() {
		var e wallet.LedgerEntry
		if err = rows.Scan(&e.ID, &e.UserID, &e.Kind, &e.Amount, &e.Balance, &e.Reference, &e.Description, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
func (s *WalletStore) Order(ctx context.Context, userID, id string) (wallet.Order, error) {
	return scanOrder(s.db.QueryRowContext(ctx, "SELECT "+orderColumns+" FROM payment_orders WHERE id=? AND user_id=?", id, userID))
}
func (s *WalletStore) Orders(ctx context.Context, userID string, limit int) ([]wallet.Order, error) {
	return s.OrdersPage(ctx, userID, limit, 0)
}
func (s *WalletStore) OrdersPage(ctx context.Context, userID string, limit, offset int) ([]wallet.Order, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+orderColumns+" FROM payment_orders WHERE (?='' OR user_id=?) ORDER BY created_at DESC,id DESC LIMIT ? OFFSET ?", userID, userID, min(max(limit, 1), 100), max(offset, 0))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	orders := []wallet.Order{}
	for rows.Next() {
		o, e := scanOrder(rows)
		if e != nil {
			return nil, e
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}
func (s *WalletStore) CreateOrder(ctx context.Context, o wallet.Order, operation string) (wallet.Order, error) {
	if o.Amount <= 0 || int64(o.Amount)%10_000 != 0 || o.Currency != "USD" || operation == "" || len(operation) > 128 {
		return o, wallet.ErrInvalid
	}
	if _, err := s.Account(ctx, o.UserID); err != nil {
		return o, err
	}
	if o.ID == "" {
		o.ID = strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	o.State = "pending"
	_, err := s.db.ExecContext(ctx, "INSERT INTO payment_orders(id,user_id,provider,amount_micros,currency,idempotency_key) VALUES(?,?,?,?,?,?) ON CONFLICT(user_id,idempotency_key) DO NOTHING", o.ID, o.UserID, o.Provider, o.Amount, o.Currency, operation)
	if err != nil {
		return o, err
	}
	existing, err := scanOrder(s.db.QueryRowContext(ctx, "SELECT "+orderColumns+" FROM payment_orders WHERE user_id=? AND idempotency_key=?", o.UserID, operation))
	if err != nil {
		return o, err
	}
	if existing.Amount != o.Amount || existing.Provider != o.Provider {
		return existing, wallet.ErrConflict
	}
	return existing, nil
}
func (s *WalletStore) SetCheckout(ctx context.Context, id string, c wallet.Checkout) error {
	// A verified webhook may have attached the provider ID before this response.
	result, err := s.db.ExecContext(ctx, "UPDATE payment_orders SET provider_id=CASE WHEN provider='lemonsqueezy' THEN provider_id ELSE ? END,provider_checkout_id=?,checkout_url=?,crypto_amount=?,crypto_token=?,expires_at=? WHERE id=? AND (provider='lemonsqueezy' OR provider_id IS NULL OR provider_id=?)", c.ProviderID, c.ProviderID, c.URL, c.CryptoAmount, c.CryptoToken, c.ExpiresAt, id, c.ProviderID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n != 1 {
		return wallet.ErrConflict
	}
	return err
}
func (s *WalletStore) ApplyPayment(ctx context.Context, provider string, e wallet.PaymentEvent) error {
	if e.EventID == "" || e.OrderID == "" || e.ProviderID == "" || provider == "epusdt" && e.Paid && e.TransactionID == "" {
		return wallet.ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = s.applyPayment(ctx, tx, provider, e); err != nil {
		return err
	}
	if e.Paid {
		// Verified reversals received first must consume the new credit before
		// any other transaction can reserve it. A bad inbox item keeps this
		// payment pending for review rather than exposing uncertain funds.
		rows, err := tx.QueryContext(ctx, "SELECT payload FROM payment_inbox WHERE provider=? AND order_id=? AND processed_at IS NULL ORDER BY created_at,event_id FOR UPDATE", provider, e.OrderID)
		if err != nil {
			return err
		}
		var reversals []wallet.PaymentEvent
		for rows.Next() {
			var payload []byte
			var reversal wallet.PaymentEvent
			if err = rows.Scan(&payload); err == nil {
				err = json.Unmarshal(payload, &reversal)
			}
			if err != nil {
				rows.Close()
				return err
			}
			reversals = append(reversals, reversal)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, reversal := range reversals {
			if reversal.OrderID != e.OrderID || reversal.Paid || (reversal.Kind != "refund" && reversal.Kind != "reversal") {
				return wallet.ErrConflict
			}
			if err = s.applyPayment(ctx, tx, provider, reversal); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// applyPayment runs inside the wallet owner's existing transaction.
func (s *WalletStore) applyPayment(ctx context.Context, tx *Tx, provider string, e wallet.PaymentEvent) error {
	var userID string
	if err := tx.QueryRowContext(ctx, "SELECT user_id FROM payment_orders WHERE id=?", e.OrderID).Scan(&userID); err != nil {
		return err
	}
	a, err := lockWallet(ctx, tx, userID)
	if err != nil {
		return err
	}
	o, err := scanOrder(tx.QueryRowContext(ctx, "SELECT "+orderColumns+" FROM payment_orders WHERE id=? FOR UPDATE", e.OrderID))
	if err != nil {
		return err
	}
	if o.Provider != provider || e.Amount != o.Amount || e.Currency != "" && e.Currency != o.Currency || o.ProviderID != "" && o.ProviderID != e.ProviderID {
		return wallet.ErrInvalid
	}
	if e.ActorID != "" {
		if e.Paid || e.Description == "" || e.ReversedAmount <= 0 {
			return wallet.ErrInvalid
		}
		fingerprint, _ := json.Marshal([]any{o.ID, e.ReversedAmount.String(), e.Description})
		done, err := claimOperation(ctx, tx, userID, "manual-reversal:"+e.EventID, string(fingerprint))
		if err != nil {
			return err
		}
		if done {
			return nil
		}
	}
	if !e.Paid && e.Kind != "refund" && e.Kind != "reversal" {
		return nil
	}
	if !e.Paid && (e.ReversedAmount <= 0 || e.ReversedAmount > o.Amount) {
		return wallet.ErrInvalid
	}
	if !e.Paid && o.State != "paid" && o.State != "reversed" {
		b, err := json.Marshal(e)
		if err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, `INSERT INTO payment_inbox(provider,event_id,order_id,payload) VALUES(?,?,?,?::jsonb)
		ON CONFLICT(provider,event_id) DO UPDATE SET event_id=EXCLUDED.event_id
		WHERE payment_inbox.order_id=EXCLUDED.order_id AND payment_inbox.payload=EXCLUDED.payload`, provider, e.EventID, o.ID, string(b))
		if err != nil {
			return err
		}
		if n, err := result.RowsAffected(); err != nil {
			return err
		} else if n == 0 {
			return wallet.ErrConflict
		}
		return nil
	}
	payload, err := json.Marshal(e)
	if err != nil {
		return err
	}
	var seenOrder string
	var samePayload bool
	err = tx.QueryRowContext(ctx, "SELECT order_id,payload=?::jsonb FROM payment_events WHERE provider=? AND event_id=?", string(payload), provider, e.EventID).Scan(&seenOrder, &samePayload)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if seenOrder != "" {
		if seenOrder != o.ID || !samePayload {
			return wallet.ErrConflict
		}
		_, err = tx.ExecContext(ctx, "UPDATE payment_inbox SET processed_at=CURRENT_TIMESTAMP WHERE provider=? AND event_id=?", provider, e.EventID)
		if err != nil {
			return err
		}
		return nil
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO payment_events(provider,event_id,order_id,payload) VALUES(?,?,?,?::jsonb)", provider, e.EventID, o.ID, string(payload)); err != nil {
		return err
	}
	if e.Paid {
		if e.TransactionID != "" {
			result, err := tx.ExecContext(ctx, "UPDATE payment_orders SET provider_transaction_id=? WHERE id=? AND (provider_transaction_id IS NULL OR provider_transaction_id=?)", e.TransactionID, o.ID, e.TransactionID)
			if err != nil {
				return err
			}
			if n, err := result.RowsAffected(); err != nil {
				return err
			} else if n != 1 {
				return wallet.ErrConflict
			}
		}
		if o.State == "paid" || o.State == "reversed" {
			return nil
		}
		credit := o.Amount
		reduce := min(credit, a.Shortfall)
		credit -= reduce
		a.Shortfall -= reduce
		if _, err = tx.ExecContext(ctx, "UPDATE wallets SET balance_micros=balance_micros+?,shortfall_micros=?,frozen=CASE WHEN ?>0 THEN TRUE ELSE frozen END WHERE user_id=?", credit, a.Shortfall, a.Shortfall, userID); err != nil {
			return err
		}
		if err = appendLedger(ctx, tx, userID, "top_up", "payment:"+o.ID, "Verified "+provider+" payment", "", o.Amount, a.Balance+credit); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE payment_orders SET state='paid',provider_id=? WHERE id=?", e.ProviderID, o.ID); err != nil {
			return err
		}
	} else {
		if o.State != "paid" && o.State != "reversed" {
			return wallet.ErrConflict
		}
		var reversed wallet.Money
		if err = tx.QueryRowContext(ctx, "SELECT reversed_micros FROM payment_orders WHERE id=?", o.ID).Scan(&reversed); err != nil {
			return err
		}
		total := e.ReversedAmount
		if e.ReversalIsDelta {
			if total <= 0 || total > o.Amount-reversed {
				return wallet.ErrInvalid
			}
			total += reversed
		}
		if total <= 0 || total > o.Amount {
			return wallet.ErrInvalid
		}
		if total <= reversed {
			return nil
		}
		delta := total - reversed
		debit := min(delta, a.Available)
		shortfall := delta - debit
		if _, err = tx.ExecContext(ctx, "UPDATE wallets SET balance_micros=balance_micros-?,shortfall_micros=shortfall_micros+?,frozen=frozen OR ?>0 WHERE user_id=?", debit, shortfall, shortfall, userID); err != nil {
			return err
		}
		if err = appendLedger(ctx, tx, userID, e.Kind, "reversal:"+provider+":"+e.EventID, func() string {
			if e.Description != "" {
				return e.Description
			}
			return "Payment reversal"
		}(), e.ActorID, -delta, a.Balance-debit); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE payment_orders SET state='reversed',reversed_micros=? WHERE id=?", total, o.ID); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, "UPDATE payment_inbox SET processed_at=CURRENT_TIMESTAMP WHERE provider=? AND event_id=?", provider, e.EventID); err != nil {
		return err
	}
	return nil
}
func (s *WalletStore) Adjust(ctx context.Context, userID string, amount wallet.Money, operation, actor, reason string) error {
	if amount == 0 || amount == wallet.Money(-1<<63) || operation == "" || len(operation) > 128 || actor == "" || reason == "" {
		return wallet.ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	a, err := lockWallet(ctx, tx, userID)
	if err != nil {
		return err
	}
	ref := "adjust:" + userID + ":" + operation
	fingerprint, err := json.Marshal([]any{amount.String(), actor, reason})
	if err != nil {
		return err
	}
	done, err := claimOperation(ctx, tx, userID, "adjust:"+operation, string(fingerprint))
	if err != nil {
		return err
	}
	if done {
		return tx.Commit()
	}
	if amount < 0 && a.Available < -amount {
		return wallet.ErrInsufficient
	}
	if amount > 0 && int64(a.Balance) > int64(^uint64(0)>>1)-int64(amount) {
		return wallet.ErrInvalid
	}
	credit := amount
	shortfall := a.Shortfall
	if credit > 0 {
		reduce := min(credit, shortfall)
		credit -= reduce
		shortfall -= reduce
	}
	if _, err = tx.ExecContext(ctx, "UPDATE wallets SET balance_micros=balance_micros+?,shortfall_micros=? WHERE user_id=?", credit, shortfall, userID); err != nil {
		return err
	}
	if err = appendLedger(ctx, tx, userID, "admin_adjustment", ref, reason, actor, amount, a.Balance+credit); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *WalletStore) Pending(ctx context.Context) ([]wallet.Reservation, error) {
	return s.PendingPage(ctx, 100, 0)
}
func (s *WalletStore) PendingPage(ctx context.Context, limit, offset int) ([]wallet.Reservation, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+reservationColumns+" FROM usage_reservations WHERE state='pending' AND created_at<CURRENT_TIMESTAMP-INTERVAL '5 minutes' ORDER BY created_at,id LIMIT ? OFFSET ?", min(max(limit, 1), 100), max(offset, 0))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []wallet.Reservation{}
	for rows.Next() {
		r, e := scanReservation(rows)
		if e != nil {
			return nil, e
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
func (s *WalletStore) Resolve(ctx context.Context, id string, charge bool, actor, reason string) error {
	if actor == "" || reason == "" {
		return wallet.ErrInvalid
	}
	return s.settle(ctx, id, charge, usage.Event{}, actor, reason)
}

// ProcessOutbox writes usage and acknowledges it in the same transaction.
func (s *WalletStore) ProcessOutbox(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, "SELECT id,payload FROM billing_outbox WHERE processed_at IS NULL AND event_type='usage.settled' ORDER BY created_at LIMIT 500 FOR UPDATE SKIP LOCKED")
	if err != nil {
		return err
	}
	ids := []string{}
	events := []usage.Event{}
	for rows.Next() {
		var id string
		var event usage.Event
		var b []byte
		if err = rows.Scan(&id, &b); err != nil {
			rows.Close()
			return err
		}
		if err = json.Unmarshal(b, &event); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
		events = append(events, event)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(events) == 0 {
		return tx.Commit()
	}
	// Keep decoding in Go so malformed events roll back the entire batch, and
	// re-encode typed values to preserve defaults for older outbox payloads.
	// Bulk SQL bounds transaction duration independently of the batch size.
	payload, err := json.Marshal(events)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO usage_events
        (id,key_id,user_id,method,path,status_code,latency_ms,request_bytes,response_bytes,cost_multiplier,ip_address,user_agent,timestamp,units,route_id,metered_value,metering_unit)
        SELECT "ID","KeyID","UserID","Method","Path","StatusCode","LatencyMs","RequestBytes","ResponseBytes","CostMultiplier","IPAddress","UserAgent","Timestamp","Units","RouteID","MeteredValue","MeteringUnit"
        FROM jsonb_to_recordset(?::jsonb) AS e(
            "ID" TEXT,"KeyID" TEXT,"UserID" TEXT,"Method" TEXT,"Path" TEXT,"StatusCode" BIGINT,
            "LatencyMs" BIGINT,"RequestBytes" BIGINT,"ResponseBytes" BIGINT,"CostMultiplier" DOUBLE PRECISION,
            "IPAddress" TEXT,"UserAgent" TEXT,"Timestamp" TIMESTAMPTZ,"Units" BIGINT,
            "RouteID" TEXT,"MeteredValue" DOUBLE PRECISION,"MeteringUnit" TEXT)
        ORDER BY "ID"
        ON CONFLICT(id) DO NOTHING`, string(payload)); err != nil {
		return fmt.Errorf("write durable usage: %w", err)
	}
	if _, err = tx.ExecContext(ctx, "UPDATE billing_outbox SET processed_at=CURRENT_TIMESTAMP WHERE id=ANY(?::text[])", ids); err != nil {
		return err
	}
	// Lock keys in a stable order when concurrent consumers touch overlapping
	// batches, and never move last_used backwards for delayed events.
	if _, err = tx.ExecContext(ctx, `WITH latest AS MATERIALIZED (
            SELECT "KeyID" AS id, MAX("Timestamp") AS at
            FROM jsonb_to_recordset(?::jsonb) AS e("KeyID" TEXT,"Timestamp" TIMESTAMPTZ)
            WHERE "KeyID"<>'' GROUP BY "KeyID"
        ), locked AS MATERIALIZED (
            SELECT k.id FROM api_keys k JOIN latest l ON l.id=k.id ORDER BY k.id FOR UPDATE OF k
        )
        UPDATE api_keys k SET last_used=GREATEST(k.last_used,l.at)
        FROM latest l JOIN locked ON locked.id=l.id WHERE k.id=l.id`, string(payload)); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *WalletStore) RenewExpired(ctx context.Context, now time.Time) error {
	rows, err := s.db.QueryContext(ctx, "SELECT DISTINCT user_id FROM plan_terms WHERE ends_at<=? AND renewal_attempted=FALSE LIMIT 100", now)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		tx, e := s.db.BeginTx(ctx, nil)
		if e != nil {
			return e
		}
		a, e := lockWallet(ctx, tx, id)
		if e == nil {
			_, _, e = resolveTerm(ctx, tx, &a, now)
		}
		if e != nil {
			tx.Rollback()
			return e
		}
		if e = tx.Commit(); e != nil {
			return e
		}
	}
	return nil
}

func queueNotification(ctx context.Context, tx *Tx, userID, kind, ref string, data map[string]any) error {
	hash := sha256.Sum256([]byte("notification:" + ref))
	id := hex.EncodeToString(hash[:])
	b, err := json.Marshal(map[string]any{"id": id, "type": kind, "user_id": userID, "timestamp": time.Now().UTC(), "data": data})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO billing_outbox(id,event_type,payload) VALUES(?, 'notification',?::jsonb) ON CONFLICT DO NOTHING", id, string(b))
	return err
}

// ProcessPaymentInbox retries verified reversals received before their payment.
func (s *WalletStore) ProcessPaymentInbox(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, "SELECT i.provider,i.payload FROM payment_inbox i JOIN payment_orders o ON o.id=i.order_id WHERE i.processed_at IS NULL AND o.state IN ('paid','reversed') ORDER BY i.created_at LIMIT 100")
	if err != nil {
		return err
	}
	type item struct {
		provider string
		event    wallet.PaymentEvent
	}
	var items []item
	for rows.Next() {
		var it item
		var b []byte
		if err = rows.Scan(&it.provider, &b); err == nil {
			err = json.Unmarshal(b, &it.event)
		}
		if err != nil {
			rows.Close()
			return err
		}
		items = append(items, it)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	var failures []error
	for _, it := range items {
		if err = s.ApplyPayment(ctx, it.provider, it.event); err != nil {
			// Preserve the unresolved event, but do not starve unrelated orders.
			failures = append(failures, fmt.Errorf("verified event %s/%s: %w", it.provider, it.event.EventID, err))
		}
	}
	return errors.Join(failures...)
}

// ReverseOrder records a provider reversal after an administrator verifies its
// external evidence. It never initiates a transfer at the payment provider.
func (s *WalletStore) ReverseOrder(ctx context.Context, id string, amount wallet.Money, operation, actor, reason string) error {
	if amount <= 0 || operation == "" || len(operation) > 128 || actor == "" || len(reason) < 10 {
		return wallet.ErrInvalid
	}
	var userID string
	if err := s.db.QueryRowContext(ctx, "SELECT user_id FROM payment_orders WHERE id=?", id).Scan(&userID); err != nil {
		return err
	}
	order, err := s.Order(ctx, userID, id)
	if err != nil {
		return err
	}
	if order.State != "paid" && order.State != "reversed" {
		return wallet.ErrConflict
	}
	return s.ApplyPayment(ctx, order.Provider, wallet.PaymentEvent{EventID: "admin:" + actor + ":" + operation, OrderID: id, ProviderID: order.ProviderID, Currency: order.Currency, Amount: order.Amount, Kind: "reversal", ReversedAmount: amount, ReversalIsDelta: true, ActorID: actor, Description: reason})
}
