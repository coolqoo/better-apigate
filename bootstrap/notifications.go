package bootstrap

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"time"

	"github.com/artpar/apigate/adapters/email"
	"github.com/artpar/apigate/domain/webhook"
	"github.com/artpar/apigate/ports"
)

// Notifications are at-least-once externally. Stable event IDs let receivers
// deduplicate a delivery whose acknowledgement was lost during a crash.
func (a *App) processNotifications(ctx context.Context) error {
	tx, err := a.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, "SELECT id,payload FROM billing_outbox WHERE event_type='notification' AND processed_at IS NULL ORDER BY created_at LIMIT 100 FOR UPDATE SKIP LOCKED")
	if err != nil {
		return err
	}
	type notice struct {
		ID     string         `json:"id"`
		UserID string         `json:"user_id"`
		Type   string         `json:"type"`
		Data   map[string]any `json:"data"`
		raw    []byte
	}
	var notices []notice
	for rows.Next() {
		var id string
		var b []byte
		if err = rows.Scan(&id, &b); err != nil {
			rows.Close()
			return err
		}
		var n notice
		if err = json.Unmarshal(b, &n); err != nil {
			rows.Close()
			return err
		}
		n.ID = id
		n.raw = b
		notices = append(notices, n)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, n := range notices {
		hooks, err := tx.QueryContext(ctx, `SELECT url,secret FROM webhooks WHERE enabled=1 AND (user_id=? OR user_id IN (SELECT id FROM users WHERE role='admin')) AND EXISTS(SELECT 1 FROM jsonb_array_elements_text(COALESCE(events,'[]')::jsonb) e WHERE e.value=?)`, n.UserID, n.Type)
		if err != nil {
			return err
		}
		type dest struct{ url, secret string }
		var destinations []dest
		for hooks.Next() {
			var d dest
			if err = hooks.Scan(&d.url, &d.secret); err != nil {
				hooks.Close()
				return err
			}
			destinations = append(destinations, d)
		}
		err = hooks.Err()
		hooks.Close()
		if err != nil {
			return err
		}
		enqueue := func(channel, target, secret string) error {
			h := sha256.Sum256([]byte(n.ID + ":" + channel + ":" + target))
			_, e := tx.ExecContext(ctx, "INSERT INTO notification_deliveries(id,event_id,channel,target,secret,payload) VALUES(?,?,?,?,?,?::jsonb) ON CONFLICT DO NOTHING", hex.EncodeToString(h[:]), n.ID, channel, target, secret, string(n.raw))
			return e
		}
		for _, d := range destinations {
			if err = enqueue("webhook", d.url, d.secret); err != nil {
				return err
			}
		}
		if provider := a.Settings.GetValue("email.provider"); provider != "" && provider != "none" {
			var recipient string
			if err = tx.QueryRowContext(ctx, "SELECT email FROM users WHERE id=?", n.UserID).Scan(&recipient); err != nil {
				return err
			}
			if err = enqueue("email", recipient, ""); err != nil {
				return err
			}
		}
		if _, err = tx.ExecContext(ctx, "UPDATE billing_outbox SET processed_at=CURRENT_TIMESTAMP WHERE id=?", n.ID); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return a.deliverNotification(ctx)
}
func (a *App) deliverNotification(ctx context.Context) error {
	tx, err := a.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id, channel, target, secret string
	var b []byte
	var attempts int
	err = tx.QueryRowContext(ctx, "SELECT id,channel,target,secret,payload,attempts FROM notification_deliveries WHERE delivered_at IS NULL AND next_attempt<=CURRENT_TIMESTAMP ORDER BY next_attempt LIMIT 1 FOR UPDATE SKIP LOCKED").Scan(&id, &channel, &target, &secret, &b, &attempts)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if channel == "webhook" {
		var event struct{ ID, Type string }
		json.Unmarshal(b, &event)
		var req *http.Request
		req, err = http.NewRequestWithContext(sendCtx, http.MethodPost, target, bytes.NewReader(b))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Event-ID", event.ID)
			req.Header.Set("X-Event-Type", event.Type)
			req.Header.Set("X-Webhook-Signature", webhook.SignPayload(b, secret))
			client := http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
			var resp *http.Response
			resp, err = client.Do(req)
			if err == nil {
				io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
				resp.Body.Close()
				if resp.StatusCode < 200 || resp.StatusCode >= 300 {
					err = fmt.Errorf("HTTP %d", resp.StatusCode)
				}
			}
		}
	} else {
		var event struct {
			Type string
			Data map[string]any
		}
		json.Unmarshal(b, &event)
		var sender ports.EmailSender
		sender, err = email.NewSender(a.Settings.Get())
		if err == nil {
			err = sender.Send(sendCtx, ports.EmailMessage{To: target, Subject: "APIGate · " + event.Type, HTMLBody: "<p>" + html.EscapeString(fmt.Sprint(event.Data["description"])) + "</p><p>Review your wallet and plan in the customer portal.</p>"})
		}
	}
	if err == nil {
		_, err = tx.ExecContext(ctx, "UPDATE notification_deliveries SET delivered_at=CURRENT_TIMESTAMP,attempts=attempts+1,last_error='' WHERE id=?", id)
	} else {
		delay := time.Second * time.Duration(1<<min(attempts+1, 12))
		_, err = tx.ExecContext(ctx, "UPDATE notification_deliveries SET attempts=attempts+1,next_attempt=?,last_error=? WHERE id=?", time.Now().Add(delay), err.Error(), id)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}
