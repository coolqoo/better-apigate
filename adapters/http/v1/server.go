// Package v1 exposes browser sessions and typed prepaid actions.
package v1

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/coolqoo/better-apigate/adapters/auth"
	"github.com/coolqoo/better-apigate/adapters/payment"
	"github.com/coolqoo/better-apigate/adapters/postgres"
	"github.com/coolqoo/better-apigate/app"
	"github.com/coolqoo/better-apigate/core/runtime"
	"github.com/coolqoo/better-apigate/domain/portal"
	"github.com/coolqoo/better-apigate/domain/wallet"
	"github.com/coolqoo/better-apigate/pkg/jsonapi"
	"github.com/coolqoo/better-apigate/ports"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type Deps struct {
	Actions        *runtime.Runtime
	DB             *postgres.DB
	Wallet         ports.PrepaidStore
	Keys           ports.KeyStore
	Users          ports.UserStore
	Settings       *app.SettingsService
	Email          ports.EmailSender
	Limiter        ports.AtomicRateLimiter
	KeySecret      []byte
	SetupToken     string
	MetricsToken   string
	PublicURL      string
	Modules        http.Handler
	Routes         *app.RouteService
	Transforms     *app.TransformService
	Upstreams      ports.UpstreamStore
	Tokens         *auth.TokenService
	OnConfigChange func(context.Context) error
	Logger         zerolog.Logger
}
type Server struct {
	Deps
	base   *url.URL
	secure bool
}
type session struct {
	UserID, Email, Name, Role, PlanID, CSRF string
	Verified                                bool
}
type sessionKey struct{}

func New(d Deps) (*Server, error) {
	if d.MetricsToken != "" && len(d.MetricsToken) < 32 {
		return nil, errors.New("APIGATE_METRICS_TOKEN must contain at least 32 characters")
	}
	u, e := url.Parse(d.PublicURL)
	if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("APIGATE_PUBLIC_URL must be an absolute HTTP origin without a path, query or fragment")
	}
	u.Path, u.RawPath = "", ""
	return &Server{Deps: d, base: u, secure: u.Scheme == "https"}, nil
}
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(s.origin)
	r.Get("/status", s.status)
	r.Post("/auth/setup", s.setup)
	r.Post("/auth/signup", s.signup)
	r.Post("/auth/login", s.login)
	r.Post("/auth/forgot-password", s.forgot)
	r.Post("/auth/reset-password", s.reset)
	r.Post("/auth/verify", s.verify)
	r.Get("/documentation", s.documentation)
	r.Get("/openapi.json", s.openAPI)
	r.Get("/plans", s.plans)
	r.Get("/providers", s.providers)
	r.Post("/payment-webhooks/{provider}", s.webhook)
	r.Group(func(r chi.Router) {
		r.Use(s.RequireSession)
		r.Get("/session", s.me)
		r.Post("/auth/logout", s.logout)
		r.Post("/auth/resend-verification", s.resend)
		r.Get("/wallet", s.account)
		r.Get("/ledger", s.ledger)
		r.Get("/orders", s.orders)
		r.Get("/orders/{id}", s.order)
		r.Post("/top-ups", s.topup)
		r.Post("/plan/purchase", s.purchase)
		r.Patch("/plan/renewal", s.renewal)
		r.Get("/keys", s.keys)
		r.Post("/keys", s.createKey)
		r.Delete("/keys/{id}", s.revokeKey)
		r.Get("/usage", s.usage)
		r.Get("/usage/summary", s.usageSummary)
		r.Get("/usage/metered", s.meteredUsage)
		r.Patch("/account", s.updateAccount)
		r.Post("/account/password", s.password)
		r.Route("/admin", func(r chi.Router) {
			r.Use(s.RequireAdmin)
			r.Get("/overview", s.adminOverview)
			r.Get("/audit", s.auditTrail)
			r.Get("/customers", s.customers)
			r.Patch("/customers/{id}", s.customerState)
			r.Get("/customers/{id}/wallet", s.customerWallet)
			r.Post("/customers/{id}/adjustments", s.adjust)
			r.Post("/customers/{id}/unfreeze", s.unfreeze)
			r.Get("/orders", s.adminOrders)
			r.Post("/orders/{id}/reconcile", s.reconcile)
			r.Post("/orders/{id}/reversals", s.reverseOrder)
			r.Get("/reservations", s.pending)
			r.Post("/reservations/{id}/resolve", s.resolve)
			r.Get("/plans", s.adminPlans)
			r.Post("/plans", s.savePlan)
			r.Patch("/plans/{id}", s.savePlan)
			r.Get("/settings", s.getSettings)
			r.Patch("/settings", s.saveSettings)
			r.Post("/routes/test", s.testRoute)
			r.Post("/expressions/validate", s.validateExpression)
			r.Get("/upstreams/{id}/health", s.upstreamHealth)
			if s.Modules != nil {
				r.Mount("/config", http.StripPrefix("/api/v1/admin/config", s.moduleHandler()))
			}
		})
	})
	return r
}
func token() string {
	b := make([]byte, 32)
	// Go's crypto/rand.Read fills the buffer or terminates on an entropy failure.
	rand.Read(b)
	return hex.EncodeToString(b)
}
func digest(t string) []byte { d := sha256.Sum256([]byte(t)); return d[:] }
func resource(w http.ResponseWriter, status int, kind, id string, v any) {
	b, e := json.Marshal(v)
	if e != nil {
		failure(w, e)
		return
	}
	var attrs map[string]any
	if e = json.Unmarshal(b, &attrs); e != nil {
		failure(w, e)
		return
	}
	delete(attrs, "id")
	jsonapi.WriteResource(w, status, jsonapi.Resource{Type: kind, ID: id, Attributes: attrs})
}
func collection(w http.ResponseWriter, kind string, v any) {
	collectionPage(w, kind, v, nil)
}
func collectionPage(w http.ResponseWriter, kind string, v any, pagination *jsonapi.Pagination) {
	b, e := json.Marshal(v)
	if e != nil {
		failure(w, e)
		return
	}
	var rows []map[string]any
	if e = json.Unmarshal(b, &rows); e != nil {
		failure(w, e)
		return
	}
	data := []jsonapi.Resource{}
	for i, row := range rows {
		id, _ := row["id"].(string)
		if id == "" {
			id = strconv.Itoa(i)
		}
		delete(row, "id")
		data = append(data, jsonapi.Resource{Type: kind, ID: id, Attributes: row})
	}
	jsonapi.WriteCollection(w, 200, data, pagination)
}
func decode(w http.ResponseWriter, r *http.Request, out any) bool {
	ct := strings.Split(r.Header.Get("Content-Type"), ";")[0]
	if ct != "application/json" && ct != jsonapi.ContentType {
		jsonapi.WriteBadRequest(w, "Send a JSON request body.")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()
	var doc struct {
		Data struct {
			Attributes json.RawMessage `json:"attributes"`
		} `json:"data"`
	}
	b, e := io.ReadAll(r.Body)
	if e == nil {
		e = json.Unmarshal(b, &doc)
	}
	if e == nil && len(doc.Data.Attributes) > 0 {
		b = doc.Data.Attributes
	}
	if e == nil {
		d := json.NewDecoder(strings.NewReader(string(b)))
		d.DisallowUnknownFields()
		e = d.Decode(out)
	}
	if e != nil {
		jsonapi.WriteBadRequest(w, "Invalid request fields.")
		return false
	}
	return true
}
func failure(w http.ResponseWriter, e error) {
	switch {
	case errors.Is(e, wallet.ErrInsufficient):
		jsonapi.WriteError(w, jsonapi.Error{Status: "402", Code: "insufficient_funds", Title: "Insufficient funds", Detail: "Add funds to your wallet before continuing."})
	case errors.Is(e, wallet.ErrFrozen):
		jsonapi.WriteForbidden(w, "Paid access is frozen pending reconciliation.")
	case errors.Is(e, wallet.ErrInvalid):
		jsonapi.WriteBadRequest(w, "The billing operation is invalid.")
	case errors.Is(e, wallet.ErrConflict):
		jsonapi.WriteConflict(w, "This operation conflicts with existing state.")
	case errors.Is(e, sql.ErrNoRows) || errors.Is(e, ports.ErrNotFound):
		jsonapi.WriteNotFound(w, "resource")
	default:
		jsonapi.WriteError(w, jsonapi.Error{Status: "503", Code: "service_unavailable", Title: "Temporarily unavailable", Detail: "Retry shortly. No unverified payment has been credited."})
	}
}
func current(r *http.Request) session { return r.Context().Value(sessionKey{}).(session) }
func (s *Server) origin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Method != "GET" && r.Method != "HEAD" {
			o := r.Header.Get("Origin")
			if o != "" && o != s.base.Scheme+"://"+s.base.Host {
				jsonapi.WriteForbidden(w, "Cross-origin requests are not permitted.")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
func (s *Server) RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, e := r.Cookie("apigate_session")
		if e != nil {
			jsonapi.WriteUnauthorized(w, "Sign in to continue.")
			return
		}
		var u session
		e = s.DB.QueryRowContext(r.Context(), `SELECT u.id,u.email,u.name,u.role,u.plan_id,u.email_verified,se.csrf_token FROM user_sessions se JOIN users u ON u.id=se.user_id WHERE se.token_digest=? AND se.expires_at>CURRENT_TIMESTAMP AND u.status='active'`, digest(cookie.Value)).Scan(&u.UserID, &u.Email, &u.Name, &u.Role, &u.PlanID, &u.Verified, &u.CSRF)
		if e != nil {
			if errors.Is(e, sql.ErrNoRows) {
				jsonapi.WriteUnauthorized(w, "Your session has expired.")
			} else {
				failure(w, e)
			}
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" && subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(u.CSRF)) != 1 {
			jsonapi.WriteForbidden(w, "Refresh the page and retry with its CSRF token.")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionKey{}, u)))
	})
}
func (s *Server) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if current(r).Role != "admin" {
			jsonapi.WriteForbidden(w, "Administrator access required.")
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (s *Server) startSession(w http.ResponseWriter, r *http.Request, userID string) error {
	raw, csrf := token(), token()
	expires := time.Now().Add(7 * 24 * time.Hour)
	_, e := s.DB.ExecContext(r.Context(), `INSERT INTO user_sessions(id,user_id,email,token_digest,csrf_token,expires_at,ip_address,user_agent) SELECT ?,id,email,?,?,?, ?,? FROM users WHERE id=?`, uuid.NewString(), digest(raw), csrf, expires, r.RemoteAddr, r.UserAgent(), userID)
	if e != nil {
		return e
	}
	http.SetCookie(w, &http.Cookie{Name: "apigate_session", Value: raw, Path: "/", HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode, Expires: expires})
	return nil
}
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	u := current(r)
	resource(w, 200, "session", u.UserID, portal.Session{UserID: u.UserID, Email: u.Email, Name: u.Name, Role: u.Role, PlanID: u.PlanID, EmailVerified: u.Verified, CSRFToken: u.CSRF})
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	c, _ := r.Cookie("apigate_session")
	_, e := s.DB.ExecContext(r.Context(), "DELETE FROM user_sessions WHERE token_digest=?", digest(c.Value))
	if e != nil {
		failure(w, e)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "apigate_session", Path: "/", Value: "", MaxAge: -1, HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode})
	jsonapi.WriteNoContent(w)
}
func (s *Server) moduleHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowed := false
		for _, prefix := range []string{"/api/routes", "/api/upstreams", "/api/entitlements", "/api/plan-entitlements", "/api/webhooks", "/_schema"} {
			if r.URL.Path == prefix || strings.HasPrefix(r.URL.Path, prefix+"/") {
				allowed = true
			}
		}
		if !allowed {
			jsonapi.WriteForbidden(w, "Use typed account and billing actions.")
			return
		}
		u := current(r)
		raw, _, e := s.Tokens.GenerateToken(u.UserID, u.Email, u.Role, u.PlanID)
		if e != nil {
			failure(w, e)
			return
		}
		r.Header.Set("Authorization", "Bearer "+raw)
		s.Modules.ServeHTTP(w, r.WithContext(runtime.WithAuth(r.Context(), runtime.AuthContext{UserID: u.UserID, Role: u.Role, IsAdmin: true})))
	})
}
func (s *Server) registry() (*payment.Registry, error) { return payment.NewRegistry(s.Settings.Get()) }

// MetricsAccess permits a deployment-managed scrape token or a current administrator session.
func (s *Server) MetricsAccess(next http.Handler) http.Handler {
	sessions := s.RequireSession(s.RequireAdmin(next))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.MetricsToken != "" && strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if subtle.ConstantTimeCompare(digest(provided), digest(s.MetricsToken)) == 1 {
				next.ServeHTTP(w, r)
				return
			}
			jsonapi.WriteUnauthorized(w, "Invalid metrics credential.")
			return
		}
		sessions.ServeHTTP(w, r)
	})
}
