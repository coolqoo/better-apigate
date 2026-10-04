package v1

import (
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/mail"
	"strings"
	"time"

	emailadapter "github.com/artpar/apigate/adapters/email"
	"github.com/artpar/apigate/adapters/postgres"
	"github.com/artpar/apigate/domain/ratelimit"
	"github.com/artpar/apigate/pkg/jsonapi"
	"github.com/artpar/apigate/ports"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type credentials struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	Name       string `json:"name"`
	SetupToken string `json:"setup_token,omitempty"`
}

func validIdentity(c credentials) bool {
	a, e := mail.ParseAddress(c.Email)
	return e == nil && a.Address == c.Email && len(c.Name) > 0 && len(c.Name) <= 120
}
func hashPassword(password string) ([]byte, error) {
	if len(password) < 12 || len(password) > 72 {
		return nil, errors.New("password must contain 12–72 bytes")
	}
	return bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
}
func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	var count int
	if e := s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM users").Scan(&count); e != nil {
		failure(w, e)
		return
	}
	resource(w, 200, "installation", "apigate", map[string]any{"setup_required": count == 0, "app_name": s.Settings.GetValue("portal.app_name"), "currency": "USD", "top_up_amounts": s.topUpAmounts(), "paddle_client_token": s.Settings.GetValue("payment.paddle.client_token"), "paddle_sandbox": s.Settings.Get().GetBool("payment.paddle.sandbox")})
}
func (s *Server) authLimit(w http.ResponseWriter, r *http.Request) bool {
	if s.Limiter == nil {
		return true
	}
	ip := r.RemoteAddr
	if i := strings.LastIndex(ip, ":"); i >= 0 {
		ip = ip[:i]
	}
	result, e := s.Limiter.Allow(r.Context(), "auth:"+ip, ratelimit.Config{Limit: 20, Window: time.Minute}, time.Now())
	if e != nil {
		failure(w, e)
		return false
	}
	if !result.Allowed {
		jsonapi.WriteError(w, jsonapi.Error{Status: "429", Code: "rate_limit", Title: "Too many attempts", Detail: "Wait a minute and try again."})
		return false
	}
	return true
}
func (s *Server) setup(w http.ResponseWriter, r *http.Request) {
	if !s.authLimit(w, r) {
		return
	}
	var c credentials
	if !decode(w, r, &c) {
		return
	}
	if s.SetupToken == "" || subtle.ConstantTimeCompare([]byte(c.SetupToken), []byte(s.SetupToken)) != 1 {
		jsonapi.WriteForbidden(w, "Enter the deployment setup token.")
		return
	}
	if !validIdentity(c) {
		jsonapi.WriteValidationError(w, "credentials", "Use a valid email, a name, and a password of 12–72 characters.")
		return
	}
	hash, e := hashPassword(c.Password)
	if e != nil {
		jsonapi.WriteValidationError(w, "password", "Use a password of 12–72 characters.")
		return
	}
	tx, e := s.DB.BeginTx(r.Context(), nil)
	if e != nil {
		failure(w, e)
		return
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(r.Context(), "SELECT pg_advisory_xact_lock(734018922)"); e != nil {
		failure(w, e)
		return
	}
	var count int
	if e = tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM users").Scan(&count); e != nil {
		failure(w, e)
		return
	}
	if count > 0 {
		jsonapi.WriteConflict(w, "Setup is already complete.")
		return
	}
	id := uuid.NewString()
	_, e = tx.ExecContext(r.Context(), "INSERT INTO users(id,email,name,password_hash,role,plan_id,email_verified) VALUES(?,?,?,?,'admin','paygo',TRUE)", id, strings.ToLower(c.Email), c.Name, hash)
	if e == nil {
		e = tx.Commit()
	}
	if e == nil {
		e = s.startSession(w, r, id)
	}
	if e != nil {
		failure(w, e)
		return
	}
	resource(w, 201, "account", id, map[string]any{"user_id": id})
}
func (s *Server) signup(w http.ResponseWriter, r *http.Request) {
	if !s.authLimit(w, r) {
		return
	}
	var c credentials
	if !decode(w, r, &c) {
		return
	}
	c.Email = strings.ToLower(c.Email)
	if !validIdentity(c) || c.SetupToken != "" {
		jsonapi.WriteValidationError(w, "credentials", "Use a valid email, a name, and a password of 12–72 characters.")
		return
	}
	var count int
	if e := s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM users").Scan(&count); e != nil {
		failure(w, e)
		return
	}
	if count == 0 {
		jsonapi.WriteConflict(w, "Complete administrator setup first.")
		return
	}
	hash, e := hashPassword(c.Password)
	if e != nil {
		jsonapi.WriteValidationError(w, "password", "Use a password of 12–72 characters.")
		return
	}
	id := uuid.NewString()
	e = s.Users.Create(r.Context(), ports.User{ID: id, Email: c.Email, Name: c.Name, PasswordHash: hash, Role: "user", PlanID: "paygo", Status: "active"})
	if errors.Is(e, postgres.ErrDuplicate) {
		jsonapi.WriteConflict(w, "An account with this email already exists.")
		return
	}
	if e != nil {
		failure(w, e)
		return
	}
	if s.Settings.Get().GetBool("auth.require_verification") {
		if e = s.sendChallenge(r, id, c.Email, c.Name, "verify"); e != nil {
			failure(w, e)
			return
		}
	}
	if e = s.startSession(w, r, id); e != nil {
		failure(w, e)
		return
	}
	resource(w, 201, "account", id, map[string]any{"user_id": id, "verification_required": s.Settings.Get().GetBool("auth.require_verification")})
}

// Stores share this sentinel without exposing database errors to clients.

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if !s.authLimit(w, r) {
		return
	}
	var c credentials
	if !decode(w, r, &c) {
		return
	}
	u, e := s.Users.GetByEmail(r.Context(), strings.ToLower(c.Email))
	if e != nil && !errors.Is(e, postgres.ErrNotFound) && !errors.Is(e, sql.ErrNoRows) {
		failure(w, e)
		return
	}
	if e != nil || u.Status != "active" || bcrypt.CompareHashAndPassword(u.PasswordHash, []byte(c.Password)) != nil {
		jsonapi.WriteUnauthorized(w, "Email or password is incorrect.")
		return
	}
	if e = s.startSession(w, r, u.ID); e != nil {
		failure(w, e)
		return
	}
	resource(w, 200, "account", u.ID, map[string]any{"user_id": u.ID})
}
func (s *Server) sendChallenge(r *http.Request, id, email, name, purpose string) error {
	raw := token()
	_, e := s.DB.ExecContext(r.Context(), "INSERT INTO auth_challenges(digest,user_id,purpose,expires_at) VALUES(?,?,?,?)", digest(raw), id, purpose, time.Now().Add(time.Hour))
	if e != nil {
		return e
	}
	path := "/verify"
	subject := "Verify your email"
	if purpose == "reset" {
		path = "/reset-password"
		subject = "Reset your password"
	}
	link := s.base.String() + path + "?token=" + raw
	sender, err := emailadapter.NewSender(s.Settings.Get())
	if err != nil {
		return err
	}
	if s.Settings.GetValue("email.provider") == "none" || s.Settings.GetValue("email.provider") == "" {
		return errors.New("email delivery unavailable")
	}
	return sender.Send(r.Context(), ports.EmailMessage{To: email, Subject: subject, TextBody: fmt.Sprintf("Hi %s,\n\n%s: %s\n\nThis link expires in one hour.", name, subject, link), HTMLBody: fmt.Sprintf("<p>Hi %s,</p><p><a href=\"%s\">%s</a></p><p>This link expires in one hour.</p>", html.EscapeString(name), html.EscapeString(link), subject)})
}
func (s *Server) forgot(w http.ResponseWriter, r *http.Request) {
	if !s.authLimit(w, r) {
		return
	}
	var in struct {
		Email string `json:"email"`
	}
	if !decode(w, r, &in) {
		return
	}
	u, e := s.Users.GetByEmail(r.Context(), strings.ToLower(in.Email))
	if e == nil {
		if e = s.sendChallenge(r, u.ID, u.Email, u.Name, "reset"); e != nil {
			s.Logger.Error().Err(e).Msg("password reset delivery failed")
		}
	}
	resource(w, 200, "message", "reset", map[string]any{"message": "If an account exists, we sent a password reset link."})
}
func (s *Server) resend(w http.ResponseWriter, r *http.Request) {
	if !s.authLimit(w, r) {
		return
	}
	u := current(r)
	if !u.Verified {
		if e := s.sendChallenge(r, u.UserID, u.Email, u.Name, "verify"); e != nil {
			failure(w, e)
			return
		}
	}
	jsonapi.WriteNoContent(w)
}
func (s *Server) verify(w http.ResponseWriter, r *http.Request) { s.consumeChallenge(w, r, "verify") }
func (s *Server) reset(w http.ResponseWriter, r *http.Request)  { s.consumeChallenge(w, r, "reset") }
func (s *Server) consumeChallenge(w http.ResponseWriter, r *http.Request, purpose string) {
	if !s.authLimit(w, r) {
		return
	}
	var in struct {
		Token    string `json:"token"`
		Password string `json:"password,omitempty"`
	}
	if !decode(w, r, &in) {
		return
	}
	var hash []byte
	var e error
	if purpose == "reset" {
		hash, e = hashPassword(in.Password)
		if e != nil {
			jsonapi.WriteValidationError(w, "password", "Use 12–72 characters.")
			return
		}
	}
	tx, e := s.DB.BeginTx(r.Context(), nil)
	if e != nil {
		failure(w, e)
		return
	}
	defer tx.Rollback()
	var id string
	e = tx.QueryRowContext(r.Context(), "SELECT user_id FROM auth_challenges WHERE digest=? AND purpose=? AND used_at IS NULL AND expires_at>CURRENT_TIMESTAMP FOR UPDATE", digest(in.Token), purpose).Scan(&id)
	if errors.Is(e, sql.ErrNoRows) {
		jsonapi.WriteBadRequest(w, "This link is invalid or expired. Request a new one.")
		return
	}
	if e != nil {
		failure(w, e)
		return
	}
	if purpose == "verify" {
		_, e = tx.ExecContext(r.Context(), "UPDATE users SET email_verified=TRUE WHERE id=?", id)
	} else {
		_, e = tx.ExecContext(r.Context(), "UPDATE users SET password_hash=? WHERE id=?", hash, id)
		if e == nil {
			_, e = tx.ExecContext(r.Context(), "DELETE FROM user_sessions WHERE user_id=?", id)
		}
	}
	if e == nil {
		_, e = tx.ExecContext(r.Context(), "UPDATE auth_challenges SET used_at=CURRENT_TIMESTAMP WHERE user_id=? AND purpose=? AND used_at IS NULL", id, purpose)
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
func (s *Server) updateAccount(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.Name) < 1 || len(in.Name) > 120 {
		jsonapi.WriteValidationError(w, "name", "Enter a name of up to 120 characters.")
		return
	}
	_, e := s.DB.ExecContext(r.Context(), "UPDATE users SET name=?,updated_at=CURRENT_TIMESTAMP WHERE id=?", in.Name, current(r).UserID)
	if e != nil {
		failure(w, e)
		return
	}
	jsonapi.WriteNoContent(w)
}
func (s *Server) password(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Current  string `json:"current_password"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	u, e := s.Users.Get(r.Context(), current(r).UserID)
	if e != nil {
		failure(w, e)
		return
	}
	if bcrypt.CompareHashAndPassword(u.PasswordHash, []byte(in.Current)) != nil {
		jsonapi.WriteUnauthorized(w, "Current password is incorrect.")
		return
	}
	hash, e := hashPassword(in.Password)
	if e != nil {
		jsonapi.WriteValidationError(w, "password", "Use 12–72 characters.")
		return
	}
	tx, e := s.DB.BeginTx(r.Context(), nil)
	if e != nil {
		failure(w, e)
		return
	}
	defer tx.Rollback()
	_, e = tx.ExecContext(r.Context(), "UPDATE users SET password_hash=? WHERE id=?", hash, u.ID)
	if e == nil {
		_, e = tx.ExecContext(r.Context(), "DELETE FROM user_sessions WHERE user_id=?", u.ID)
	}
	if e == nil {
		e = tx.Commit()
	}
	if e == nil {
		e = s.startSession(w, r, u.ID)
	}
	if e != nil {
		failure(w, e)
		return
	}
	jsonapi.WriteNoContent(w)
}
