package v1

import (
	"encoding/json"
	"github.com/artpar/apigate/internal/contracts"
	"net/http"
	"strings"
)

func (s *Server) openAPI(w http.ResponseWriter, r *http.Request) {
	doc := contracts.Document()
	paths := doc["paths"].(map[string]any)
	rows, e := s.DB.QueryContext(r.Context(), "SELECT name,path_pattern,COALESCE(methods,'[]'),unit_cost,auth_required,match_type FROM routes WHERE enabled=1")
	if e != nil {
		failure(w, e)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var name, path, raw, matchType string
		var units int64
		var auth int
		if e = rows.Scan(&name, &path, &raw, &units, &auth, &matchType); e != nil {
			failure(w, e)
			return
		}
		if matchType == "regex" {
			continue
		}
		path = strings.TrimSuffix(path, "*") + func() string {
			if strings.HasSuffix(path, "*") {
				return "{path}"
			}
			return ""
		}()
		methods := []string{}
		if e = json.Unmarshal([]byte(raw), &methods); e != nil {
			methods = []string{"GET"}
		}
		if len(methods) == 0 {
			methods = []string{"GET", "POST"}
		}
		operations := map[string]any{}
		for _, method := range methods {
			security := []any{}
			if auth == 1 {
				security = []any{map[string]any{"apiKey": []string{}}}
			}
			operation := map[string]any{"servers": []any{map[string]any{"url": "/"}}, "summary": name, "x-unit-cost": units, "security": security, "responses": map[string]any{"200": map[string]any{"description": "Upstream response"}}}
			if strings.Contains(path, "{path}") {
				operation["parameters"] = []any{map[string]any{"name": "path", "in": "path", "required": true, "schema": map[string]any{"type": "string"}}}
			}
			operations[strings.ToLower(method)] = operation
		}
		paths[path] = operations
	}
	if e = rows.Err(); e != nil {
		failure(w, e)
		return
	}
	doc["components"].(map[string]any)["securitySchemes"].(map[string]any)["apiKey"] = map[string]any{"type": "apiKey", "in": "header", "name": "X-API-Key"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(doc)
}
func (s *Server) documentation(w http.ResponseWriter, r *http.Request) {
	rows, e := s.DB.QueryContext(r.Context(), "SELECT id,name,COALESCE(description,''),path_pattern,COALESCE(methods,'[]'),unit_cost,auth_required,COALESCE(example_request,'') FROM routes WHERE enabled=1 ORDER BY priority DESC,name")
	if e != nil {
		failure(w, e)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, name, description, path, raw, example string
		var units int64
		var auth int
		if e = rows.Scan(&id, &name, &description, &path, &raw, &units, &auth, &example); e != nil {
			failure(w, e)
			return
		}
		methods := []string{}
		if e = json.Unmarshal([]byte(raw), &methods); e != nil {
			failure(w, e)
			return
		}
		out = append(out, map[string]any{"id": id, "name": name, "description": description, "path_pattern": path, "methods": methods, "unit_cost": units, "auth_required": auth == 1, "example_request": example})
	}
	if e = rows.Err(); e != nil {
		failure(w, e)
		return
	}
	collection(w, "api-endpoint", out)
}
