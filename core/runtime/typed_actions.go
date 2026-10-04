package runtime

import "context"

// ActionHandler is a typed service boundary; its implementation owns validation,
// authorization, transaction boundaries, and idempotency.
type ActionHandler func(context.Context, ActionInput) (ActionResult, error)

func (r *Runtime) RegisterActionHandler(module, action string, handler ActionHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.typedActions[module+"."+action] = handler
}
func (r *Runtime) ProtectModule(module string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.protectedModules[module] = true
}

// WithAuth lets a trusted transport propagate its verified principal to audit events.
type authContextKey struct{}

func WithAuth(ctx context.Context, auth AuthContext) context.Context {
	return context.WithValue(ctx, authContextKey{}, auth)
}
func AuthFromContext(ctx context.Context) AuthContext {
	a, _ := ctx.Value(authContextKey{}).(AuthContext)
	return a
}
