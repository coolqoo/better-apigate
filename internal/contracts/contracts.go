// Package contracts derives OpenAPI and TypeScript schemas from Go API models.
package contracts

import (
	"fmt"
	"github.com/coolqoo/better-apigate/app"
	"github.com/coolqoo/better-apigate/domain/portal"
	"github.com/coolqoo/better-apigate/domain/route"
	"github.com/coolqoo/better-apigate/domain/wallet"
	"reflect"
	"sort"
	"strings"
	"time"
)

type Endpoint struct {
	Method, Path, Response, Request string
	Public, Collection              bool
}

var Endpoints = []Endpoint{
	{"get", "/documentation", "APIEndpoint", "", true, true},
	{"get", "/usage/metered", "MeteredUsage", "", false, true},
	{"post", "/admin/routes/test", "RouteTestResult", "RouteTestRequest", false, false},
	{"post", "/admin/expressions/validate", "ExprValidationResult", "ExpressionRequest", false, false},
	{"get", "/admin/upstreams/{id}/health", "UpstreamHealth", "", false, false},
	{"get", "/usage/summary", "UsageSummary", "", false, false},
	{"get", "/admin/audit", "AuditEntry", "", false, true},
	{"post", "/admin/orders/{id}/reversals", "Order", "ReversalRequest", false, false},
	{"get", "/status", "Installation", "", true, false},
	{"post", "/auth/setup", "AccountCreated", "Credentials", true, false}, {"post", "/auth/signup", "AccountCreated", "Credentials", true, false}, {"post", "/auth/login", "AccountCreated", "Credentials", true, false},
	{"post", "/auth/forgot-password", "Message", "EmailRequest", true, false}, {"post", "/auth/reset-password", "", "ChallengeRequest", true, false}, {"post", "/auth/verify", "", "ChallengeRequest", true, false},
	{"post", "/auth/logout", "", "", false, false}, {"post", "/auth/resend-verification", "", "", false, false},
	{"get", "/session", "Session", "", false, false}, {"get", "/wallet", "Wallet", "", false, false}, {"get", "/ledger", "LedgerEntry", "", false, true},
	{"get", "/plans", "Plan", "", true, true}, {"get", "/providers", "ProviderInfo", "", true, true},
	{"get", "/orders", "Order", "", false, true}, {"get", "/orders/{id}", "Order", "", false, false},
	{"post", "/top-ups", "Order", "TopUpRequest", false, false}, {"post", "/plan/purchase", "Wallet", "PurchaseRequest", false, false}, {"patch", "/plan/renewal", "Wallet", "RenewalRequest", false, false},
	{"get", "/keys", "APIKey", "", false, true}, {"post", "/keys", "APIKey", "KeyRequest", false, false}, {"delete", "/keys/{id}", "", "", false, false}, {"get", "/usage", "UsageDay", "", false, true},
	{"patch", "/account", "", "AccountRequest", false, false}, {"post", "/account/password", "", "PasswordRequest", false, false},
	{"get", "/admin/overview", "Overview", "", false, false}, {"get", "/admin/customers", "Customer", "", false, true}, {"patch", "/admin/customers/{id}", "", "CustomerStateRequest", false, false},
	{"get", "/admin/customers/{id}/wallet", "Wallet", "", false, false}, {"post", "/admin/customers/{id}/adjustments", "Wallet", "AdjustmentRequest", false, false}, {"post", "/admin/customers/{id}/unfreeze", "", "ReasonRequest", false, false},
	{"get", "/admin/orders", "Order", "", false, true}, {"post", "/admin/orders/{id}/reconcile", "Order", "", false, false},
	{"get", "/admin/reservations", "Reservation", "", false, true}, {"post", "/admin/reservations/{id}/resolve", "", "ResolveRequest", false, false},
	{"get", "/admin/plans", "Plan", "", false, true}, {"post", "/admin/plans", "Plan", "Plan", false, false}, {"patch", "/admin/plans/{id}", "Plan", "Plan", false, false},
	{"get", "/admin/settings", "Settings", "", false, false}, {"patch", "/admin/settings", "Settings", "Settings", false, false},
}
var Models = map[string]any{"APIEndpoint": portal.APIEndpoint{}, "MeteredUsage": portal.MeteredUsage{}, "UpstreamHealth": portal.UpstreamHealth{}, "RouteTestRequest": app.RouteTestRequest{}, "RouteTestResult": app.RouteTestResult{}, "RouteDraft": app.RouteDraft{}, "ExpressionRequest": app.ExpressionRequest{}, "ExprValidationResult": app.ExprValidationResult{}, "Transform": route.Transform{}, "HeaderMatch": route.HeaderMatch{}, "UsageSummary": portal.UsageSummary{}, "AuditEntry": portal.AuditEntry{}, "Pagination": portal.Pagination{}, "ReversalRequest": portal.ReversalRequest{}, "AccountCreated": portal.AccountCreated{}, "Message": portal.Message{}, "Wallet": wallet.Account{}, "Term": wallet.Term{}, "Plan": wallet.Plan{}, "Order": wallet.Order{}, "Reservation": wallet.Reservation{}, "LedgerEntry": wallet.LedgerEntry{}, "ProviderInfo": wallet.ProviderInfo{}, "Session": portal.Session{}, "APIKey": portal.APIKey{}, "UsageDay": portal.UsageDay{}, "Installation": portal.Installation{}, "Customer": portal.Customer{}, "Overview": portal.Overview{}, "Settings": portal.Settings{}, "Credentials": portal.Credentials{}, "TopUpRequest": portal.TopUpRequest{}, "PurchaseRequest": portal.PurchaseRequest{}, "RenewalRequest": portal.RenewalRequest{}, "KeyRequest": portal.KeyRequest{}, "AdjustmentRequest": portal.AdjustmentRequest{}, "ResolveRequest": portal.ResolveRequest{}, "CustomerStateRequest": portal.CustomerStateRequest{}, "ReasonRequest": portal.ReasonRequest{}, "AccountRequest": portal.AccountRequest{}, "PasswordRequest": portal.PasswordRequest{}, "EmailRequest": portal.EmailRequest{}, "ChallengeRequest": portal.ChallengeRequest{}}

func schema(t reflect.Type) map[string]any {
	if t == reflect.TypeOf(wallet.Money(0)) {
		return map[string]any{"type": "string", "pattern": "^-?[0-9]+\\.[0-9]{6}$", "description": "Exact USD decimal, stored as integer millionths"}
	}
	if t == reflect.TypeOf(time.Time{}) {
		return map[string]any{"type": "string", "format": "date-time"}
	}
	if t.Kind() == reflect.Pointer {
		return map[string]any{"anyOf": []any{schema(t.Elem()), map[string]any{"type": "null"}}}
	}
	switch t.Kind() {
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int64:
		return map[string]any{"type": "integer", "format": "int64"}
	case reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.Map:
		return map[string]any{"type": "object", "additionalProperties": schema(t.Elem())}
	case reflect.Slice:
		return map[string]any{"type": "array", "items": schema(t.Elem())}
	case reflect.Struct:
		props := map[string]any{}
		required := []string{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			tag := f.Tag.Get("json")
			if tag == "-" {
				continue
			}
			name := strings.Split(tag, ",")[0]
			if name == "" {
				name = f.Name
			}
			props[name] = schema(f.Type)
			if !strings.Contains(tag, "omitempty") {
				required = append(required, name)
			}
		}
		return map[string]any{"type": "object", "properties": props, "required": required}
	}
	return map[string]any{}
}
func envelope(name string, collection bool) map[string]any {
	attributes := schema(reflect.TypeOf(Models[name]))
	props := attributes["properties"].(map[string]any)
	delete(props, "id")
	required := []string{}
	for _, field := range attributes["required"].([]string) {
		if field != "id" {
			required = append(required, field)
		}
	}
	attributes["required"] = required
	data := map[string]any{"type": "object", "required": []string{"type", "id", "attributes"}, "properties": map[string]any{"type": map[string]any{"type": "string"}, "id": map[string]any{"type": "string"}, "attributes": attributes}}
	var value any = data
	if collection {
		value = map[string]any{"type": "array", "items": data}
	}
	return map[string]any{"type": "object", "required": []string{"data"}, "properties": map[string]any{"data": value}}
}
func paginated(path string) bool {
	switch path {
	case "/ledger", "/orders", "/admin/customers", "/admin/orders", "/admin/reservations", "/admin/audit":
		return true
	}
	return false
}
func Document() map[string]any {
	schemas := map[string]any{}
	for name, model := range Models {
		schemas[name] = schema(reflect.TypeOf(model))
	}
	paths := map[string]any{}
	for _, e := range Endpoints {
		path := "/api/v1" + e.Path
		p, _ := paths[path].(map[string]any)
		if p == nil {
			p = map[string]any{}
		}
		security := []any{map[string]any{"sessionCookie": []string{}}}
		if e.Public {
			security = []any{}
		}
		params := []any{}
		if strings.Contains(path, "{id}") {
			params = append(params, map[string]any{"name": "id", "in": "path", "required": true, "schema": map[string]any{"type": "string"}})
		}
		if e.Method == "get" && paginated(e.Path) {
			params = append(params,
				map[string]any{"name": "page[number]", "in": "query", "schema": map[string]any{"type": "integer", "minimum": 1, "default": 1}},
				map[string]any{"name": "page[size]", "in": "query", "schema": map[string]any{"type": "integer", "minimum": 1, "maximum": 100, "default": 50}},
			)
		}
		if e.Path == "/admin/customers" {
			params = append(params, map[string]any{"name": "q", "in": "query", "description": "Case-insensitive customer name or email search", "schema": map[string]any{"type": "string", "maxLength": 200}})
		}
		if e.Method != "get" && !e.Public {
			params = append(params, map[string]any{"name": "X-CSRF-Token", "in": "header", "required": true, "schema": map[string]any{"type": "string"}})
		}
		if e.Path == "/top-ups" || e.Path == "/plan/purchase" || strings.HasSuffix(e.Path, "/adjustments") || strings.HasSuffix(e.Path, "/reversals") {
			params = append(params, map[string]any{"name": "Idempotency-Key", "in": "header", "required": true, "schema": map[string]any{"type": "string", "maxLength": 128}})
		}
		responses := map[string]any{}
		if e.Response != "" {
			code := "200"
			if e.Method == "post" && (e.Path == "/keys" || e.Path == "/top-ups" || e.Path == "/auth/setup" || e.Path == "/auth/signup") {
				code = "201"
			}
			responseSchema := envelope(e.Response, e.Collection)
			if paginated(e.Path) {
				properties := responseSchema["properties"].(map[string]any)
				properties["meta"] = map[string]any{"$ref": "#/components/schemas/Pagination"}
				properties["links"] = map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}}
			}
			responses[code] = map[string]any{"description": "Success", "content": map[string]any{"application/vnd.api+json": map[string]any{"schema": responseSchema}}}

			if e.Path == "/top-ups" {
				responses["200"] = responses[code]
				responses["202"] = responses[code]
			}
		} else {
			responses["204"] = map[string]any{"description": "Completed"}
		}
		for _, code := range []string{"400", "401", "402", "403", "404", "409", "422", "429", "503"} {
			responses[code] = map[string]any{"description": "JSON:API error; see errors[0].code and detail"}
		}
		op := map[string]any{"operationId": strings.ReplaceAll(e.Method+e.Path, "/", "_"), "security": security, "parameters": params, "responses": responses, "x-admin-required": strings.HasPrefix(e.Path, "/admin/")}
		if e.Request != "" {
			requestSchema := map[string]any{"$ref": "#/components/schemas/" + e.Request}
			if e.Request == "Plan" {
				requestSchema = schema(reflect.TypeOf(wallet.Plan{}))
				required := []string{}
				for _, field := range requestSchema["required"].([]string) {
					if field != "id" {
						required = append(required, field)
					}
				}
				requestSchema["required"] = required
			}
			op["requestBody"] = map[string]any{"required": true, "content": map[string]any{"application/vnd.api+json": map[string]any{"schema": map[string]any{"type": "object", "required": []string{"data"}, "properties": map[string]any{"data": map[string]any{"type": "object", "required": []string{"attributes"}, "properties": map[string]any{"attributes": requestSchema}}}}}}}
		}
		p[e.Method] = op
		paths[path] = p
	}
	return map[string]any{"openapi": "3.1.0", "info": map[string]any{"title": "better-apigate prepaid API", "version": "1.0.0", "description": "HttpOnly browser sessions and CSRF; gateway API keys. Exact decimal USD money, integer usage units."}, "servers": []any{map[string]any{"url": "/"}}, "paths": paths, "components": map[string]any{"schemas": schemas, "securitySchemes": map[string]any{"sessionCookie": map[string]any{"type": "apiKey", "in": "cookie", "name": "apigate_session"}}}}
}
func tsType(t reflect.Type) string {
	if t == reflect.TypeOf(wallet.Money(0)) {
		return "Money"
	}
	if t == reflect.TypeOf(time.Time{}) {
		return "string"
	}
	if t.Kind() == reflect.Pointer {
		return tsType(t.Elem()) + " | null"
	}
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int64, reflect.Float64:
		return "number"
	case reflect.Slice:
		return "(" + tsType(t.Elem()) + ")[]"
	case reflect.Map:
		return "Record<string, " + tsType(t.Elem()) + ">"
	case reflect.Struct:
		for name, model := range Models {
			if reflect.TypeOf(model) == t {
				return name
			}
		}
	}
	return "unknown"
}
func TypeScript() string {
	var b strings.Builder
	b.WriteString("// Generated by go run ./cmd/contracts -typescript. Do not edit.\nexport type Money = string\n")
	names := []string{}
	for name := range Models {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(&b, "export interface %s {\n", name)
		t := reflect.TypeOf(Models[name])
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			tag := f.Tag.Get("json")
			if tag == "-" {
				continue
			}
			key := strings.Split(tag, ",")[0]
			if key == "" {
				key = f.Name
			}
			optional := ""
			if strings.Contains(tag, "omitempty") {
				optional = "?"
			}
			fmt.Fprintf(&b, "  %s%s: %s\n", key, optional, tsType(f.Type))
		}
		b.WriteString("}\n")
	}
	return b.String()
}
