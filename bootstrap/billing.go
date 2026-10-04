package bootstrap

import (
	"context"
	"encoding/json"
	"time"

	"github.com/artpar/apigate/adapters/postgres"
	"github.com/artpar/apigate/domain/usage"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// DurableUsageRecorder persists explicitly free-route analytics to the outbox.
type DurableUsageRecorder struct {
	db  *postgres.DB
	log zerolog.Logger
}

func NewDurableUsageRecorder(db *postgres.DB, log zerolog.Logger) *DurableUsageRecorder {
	return &DurableUsageRecorder{db, log}
}
func (r *DurableUsageRecorder) Record(e usage.Event) {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	b, err := json.Marshal(e)
	if err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err = r.db.ExecContext(ctx, "INSERT INTO billing_outbox(id,event_type,payload) VALUES(?,'usage.settled',?::jsonb) ON CONFLICT DO NOTHING", e.ID, string(b))
	}
	if err != nil {
		r.log.Error().Err(err).Msg("public-route usage persistence failed")
	}
}
func (r *DurableUsageRecorder) Flush(context.Context) error { return nil }
func (r *DurableUsageRecorder) Close() error                { return nil }
func (a *App) startBillingWorkers() {
	ctx, cancel := context.WithCancel(context.Background())
	a.workersCancel = cancel
	a.workersDone = make(chan struct{})
	go func() {
		defer close(a.workersDone)
		notificationDone := make(chan struct{})
		go func() {
			defer close(notificationDone)
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if err := a.processNotifications(ctx); err != nil && ctx.Err() == nil {
						a.Logger.Error().Err(err).Msg("notification retry pending")
					}
				}
			}
		}()
		defer func() { cancel(); <-notificationDone }()
		outbox := time.NewTicker(100 * time.Millisecond)
		defer outbox.Stop()
		renew := time.NewTicker(time.Minute)
		defer renew.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-outbox.C:
				if e := a.wallet.ProcessOutbox(ctx); e != nil && ctx.Err() == nil {
					a.Logger.Error().Err(e).Msg("outbox retry pending")
				}
			case now := <-renew.C:
				if e := a.wallet.ProcessPaymentInbox(ctx); e != nil {
					a.Logger.Error().Err(e).Msg("verified payment inbox retry pending")
				}
				if e := a.wallet.RenewExpired(ctx, now.UTC()); e != nil {
					a.Logger.Error().Err(e).Msg("renewal retry pending")
				}
				if e := a.Settings.Load(ctx); e != nil {
					a.Logger.Error().Err(e).Msg("settings reload failed")
				} else {
					_ = a.ReloadPlans(ctx)
				}
			}
		}
	}()
}
