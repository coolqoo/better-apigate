package app

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/artpar/apigate/domain/key"
	"github.com/artpar/apigate/domain/proxy"
	"github.com/artpar/apigate/domain/quota"
	"github.com/artpar/apigate/domain/ratelimit"
	"github.com/artpar/apigate/domain/route"
	"github.com/artpar/apigate/domain/usage"
	"github.com/artpar/apigate/domain/wallet"
	"github.com/artpar/apigate/ports"
)

type admission struct {
	key                    key.Key
	user                   ports.User
	auth                   proxy.AuthContext
	rate                   ratelimit.CheckResult
	quota                  quota.CheckResult
	periodStart, periodEnd time.Time
	reservation            *wallet.Reservation
}

var errEnforcement = proxy.ErrorResponse{Status: 503, Code: "enforcement_unavailable", Message: "Request controls are temporarily unavailable. Retry shortly."}
var errFunding = proxy.ErrorResponse{Status: 402, Code: "insufficient_funds", Message: "Add funds to your prepaid wallet to continue."}

// admit is shared by buffered and streaming requests. A successful return
// always means the complete fixed cost is covered before upstream dispatch.
func (s *ProxyService) admit(ctx context.Context, req proxy.Request, matched *route.Route) (admission, *proxy.ErrorResponse) {
	var a admission
	now := s.clock.Now()
	cfg := s.getDynamicConfig()
	if req.APIKey == "" {
		return a, &proxy.ErrMissingKey
	}
	prefix, isKey := key.ValidateFormat(req.APIKey, s.keyPrefix)
	if isKey {
		if store, ok := s.keys.(interface {
			GetByDigest(context.Context, []byte) (key.Key, error)
		}); ok {
			k, err := store.GetByDigest(ctx, key.Digest(req.APIKey, s.keySecret))
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) || errors.Is(err, ports.ErrNotFound) {
					return a, &proxy.ErrInvalidKey
				}
				return a, &errEnforcement
			}
			a.key = k
		} else {
			keys, err := s.keys.Get(ctx, prefix)
			if err != nil {
				return a, &errEnforcement
			}
			found := false
			for _, k := range keys {
				if key.Verify(k.Hash, req.APIKey, s.keySecret) {
					a.key = k
					found = true
					break
				}
			}
			if !found {
				return a, &proxy.ErrInvalidKey
			}
		}
		valid := key.Validate(a.key, now)
		if !valid.Valid {
			return a, &proxy.ErrorResponse{Status: 401, Code: valid.Reason, Message: reasonToMessage(valid.Reason)}
		}
	} else {
		return a, &proxy.ErrInvalidKey
	}
	user, err := s.users.Get(ctx, a.key.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, ports.ErrNotFound) {
			return a, &proxy.ErrInvalidKey
		}
		return a, &errEnforcement
	}
	a.user = user
	if user.Status != "active" {
		return a, &proxy.ErrorResponse{Status: 403, Code: "user_suspended", Message: "Account is suspended"}
	}
	if len(a.key.Scopes) > 0 {
		allowed := false
		for _, scope := range a.key.Scopes {
			if scope == "*" || scope == req.Path || strings.HasSuffix(scope, "*") && strings.HasPrefix(req.Path, strings.TrimSuffix(scope, "*")) {
				allowed = true
				break
			}
		}
		if !allowed {
			return a, &proxy.ErrorResponse{Status: 403, Code: "scope_denied", Message: "This API key cannot access this endpoint"}
		}
	}
	limit := 0
	a.periodStart, a.periodEnd = quota.PeriodBounds(now)
	if s.prepaid == nil || s.atomicLimiter == nil {
		return a, &errEnforcement
	}
	{
		units := int64(1)
		routeID := ""
		if matched != nil {
			routeID = matched.ID
			if matched.UnitCost > 0 {
				units = matched.UnitCost
			}
		}
		keyID := a.key.ID
		if !isKey {
			keyID = ""
		}
		reserved, e := s.prepaid.Reserve(ctx, user.ID, keyID, routeID, units, now)
		if e != nil {
			if errors.Is(e, wallet.ErrInsufficient) {
				return a, &errFunding
			}
			if errors.Is(e, wallet.ErrFrozen) {
				return a, &proxy.ErrorResponse{Status: 403, Code: "billing_frozen", Message: "Account access requires payment reconciliation"}
			}
			return a, &errEnforcement
		}
		a.reservation = &reserved
		user.PlanID = reserved.PlanID
		a.user.PlanID = reserved.PlanID
		limit = reserved.RateLimit
	}
	rlcfg := ratelimit.Config{Limit: limit, Window: time.Duration(cfg.RateWindow) * time.Second, BurstTokens: cfg.RateBurst}
	if rlcfg.Window <= 0 {
		rlcfg.Window = time.Minute
	}
	a.rate, err = s.atomicLimiter.Allow(ctx, user.ID, rlcfg, now)
	if err != nil || !a.rate.Allowed {
		if a.reservation != nil {
			if s.SettleReservation(a.reservation, false, usage.Event{Method: req.Method, Path: req.Path, StatusCode: 429}) != nil {
				return a, &errEnforcement
			}
			a.reservation = nil
		}
		if err != nil {
			return a, &errEnforcement
		}
		return a, &proxy.ErrRateLimited
	}
	a.auth = proxy.AuthContext{KeyID: a.key.ID, UserID: user.ID, Email: user.Email, Role: user.Role, PlanID: user.PlanID, RateLimit: limit, Scopes: a.key.Scopes}
	return a, nil
}
func (s *ProxyService) SettleReservation(r *wallet.Reservation, charge bool, event usage.Event) error {
	if r == nil || s.prepaid == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.prepaid.Settle(ctx, r.ID, charge, event)
}
