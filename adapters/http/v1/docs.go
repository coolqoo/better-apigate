package v1

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/coolqoo/better-apigate/domain/portal"
	"github.com/coolqoo/better-apigate/internal/contracts"
)

func (s *Server) documentedRoutes(r *http.Request) ([]portal.APIEndpoint, error) {
	rows, err := s.DB.QueryContext(r.Context(), `SELECT id,name,COALESCE(description,''),path_pattern,match_type,COALESCE(methods,'[]'),unit_cost,auth_required,protocol,COALESCE(metering_unit,'requests'),COALESCE(example_request,''),COALESCE(example_response,'') FROM routes WHERE enabled=1 ORDER BY priority DESC,name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []portal.APIEndpoint{}
	for rows.Next() {
		var item portal.APIEndpoint
		var raw string
		var auth int
		if err = rows.Scan(&item.ID, &item.Name, &item.Description, &item.PathPattern, &item.MatchType, &raw, &item.UnitCost, &auth, &item.Protocol, &item.MeteringUnit, &item.ExampleRequest, &item.ExampleResponse); err != nil {
			return nil, err
		}
		item.Methods = []string{}
		if err = json.Unmarshal([]byte(raw), &item.Methods); err != nil {
			return nil, err
		}
		item.AuthRequired = auth == 1
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Server) documentation(w http.ResponseWriter, r *http.Request) {
	routes, err := s.documentedRoutes(r)
	if err != nil {
		failure(w, err)
		return
	}
	collection(w, "api-endpoint", routes)
}

func (s *Server) openAPI(w http.ResponseWriter, r *http.Request) {
	routes, err := s.documentedRoutes(r)
	if err != nil {
		failure(w, err)
		return
	}
	doc := contracts.Document()
	paths := doc["paths"].(map[string]any)
	for _, rt := range routes {
		if rt.MatchType == "regex" {
			continue
		}
		path := strings.TrimSuffix(rt.PathPattern, "*")
		if strings.HasSuffix(rt.PathPattern, "*") {
			path += "{path}"
		}
		methods := rt.Methods
		if len(methods) == 0 {
			methods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
		}
		operations, _ := paths[path].(map[string]any)
		if operations == nil {
			operations = map[string]any{}
		}
		for _, method := range methods {
			security := []any{}
			if rt.AuthRequired {
				security = []any{map[string]any{"apiKey": []string{}}}
			}
			response := map[string]any{"description": "Upstream response"}
			if rt.ExampleResponse != "" {
				response["content"] = exampleContent(rt.ExampleResponse)
			}
			op := map[string]any{"servers": []any{map[string]any{"url": "/"}}, "summary": rt.Name, "description": rt.Description, "x-unit-cost": rt.UnitCost, "x-metering-unit": rt.MeteringUnit, "x-protocol": rt.Protocol, "security": security, "responses": map[string]any{"200": response}}
			parameters := []any{}
			for _, match := range regexp.MustCompile(`\{([^}]+)\}`).FindAllStringSubmatch(path, -1) {
				parameters = append(parameters, map[string]any{"name": match[1], "in": "path", "required": true, "schema": map[string]any{"type": "string"}})
			}
			if len(parameters) > 0 {
				op["parameters"] = parameters
			}
			if rt.ExampleRequest != "" && method != "GET" && method != "HEAD" {
				op["requestBody"] = map[string]any{"content": exampleContent(rt.ExampleRequest)}
			}
			operations[strings.ToLower(method)] = op
		}
		paths[path] = operations
	}
	doc["components"].(map[string]any)["securitySchemes"].(map[string]any)["apiKey"] = map[string]any{"type": "apiKey", "in": "header", "name": "X-API-Key"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(doc)
}

func exampleContent(raw string) map[string]any {
	var value any
	if json.Unmarshal([]byte(raw), &value) == nil {
		return map[string]any{"application/json": map[string]any{"example": value}}
	}
	return map[string]any{"text/plain": map[string]any{"schema": map[string]any{"type": "string"}, "example": raw}}
}
