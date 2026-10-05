package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/coolqoo/better-apigate/core/runtime"
	"github.com/coolqoo/better-apigate/ports"
	"time"
)

func RegisterBillingActions(rt *runtime.Runtime, store ports.PrepaidStore) {
	rt.ProtectModule("billing")
	rt.RegisterActionHandler("billing", "purchase", func(ctx context.Context, in runtime.ActionInput) (runtime.ActionResult, error) {
		if in.Auth.UserID == "" {
			return runtime.ActionResult{}, fmt.Errorf("authentication required")
		}
		planID, _ := in.Data["plan_id"].(string)
		op, _ := in.Data["operation"].(string)
		if err := store.Purchase(ctx, in.Auth.UserID, planID, op, time.Now().UTC()); err != nil {
			return runtime.ActionResult{}, err
		}
		return runtime.ActionResult{ID: in.Auth.UserID}, nil
	})
	rt.RegisterActionHandler("billing", "account", func(ctx context.Context, in runtime.ActionInput) (runtime.ActionResult, error) {
		if in.Auth.UserID == "" {
			return runtime.ActionResult{}, fmt.Errorf("authentication required")
		}
		a, e := store.Account(ctx, in.Auth.UserID)
		if e != nil {
			return runtime.ActionResult{}, e
		}
		b, e := json.Marshal(a)
		if e != nil {
			return runtime.ActionResult{}, e
		}
		var data map[string]any
		e = json.Unmarshal(b, &data)
		return runtime.ActionResult{ID: in.Auth.UserID, Data: data}, e
	})
}
