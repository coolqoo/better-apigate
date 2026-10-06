package v1

import (
	"net/http"
	"time"

	"github.com/coolqoo/better-apigate/app"
	"github.com/coolqoo/better-apigate/domain/portal"
	"github.com/coolqoo/better-apigate/pkg/jsonapi"
	"github.com/go-chi/chi/v5"
)

func (s *Server) validateExpression(w http.ResponseWriter, r *http.Request) {
	var in app.ExpressionRequest
	if !decode(w, r, &in) {
		return
	}
	if s.Transforms == nil {
		jsonapi.WriteError(w, jsonapi.NewError(503, "unavailable", "Expression validation unavailable").Build())
		return
	}
	resource(w, 200, "expression-validation", "preview", s.Transforms.ValidateExpr(in.Expression, in.Context))
}

func (s *Server) testRoute(w http.ResponseWriter, r *http.Request) {
	var in app.RouteTestRequest
	if !decode(w, r, &in) {
		return
	}
	if s.Routes == nil || s.Transforms == nil {
		jsonapi.WriteError(w, jsonapi.NewError(503, "unavailable", "Route testing unavailable").Build())
		return
	}
	resource(w, 200, "route-test", "preview", s.Routes.PreviewRoute(r.Context(), in, s.Transforms))
}

func (s *Server) upstreamHealth(w http.ResponseWriter, r *http.Request) {
	if s.Upstreams == nil || s.Routes == nil {
		jsonapi.WriteError(w, jsonapi.NewError(503, "unavailable", "Upstream health check unavailable").Build())
		return
	}
	upstream, err := s.Upstreams.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		failure(w, err)
		return
	}
	start := time.Now()
	result := portal.UpstreamHealth{}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodHead, upstream.BaseURL, nil)
	if err == nil {
		for k, v := range s.Routes.ApplyUpstreamAuth(&upstream, map[string]string{}) {
			req.Header.Set(k, v)
		}
		timeout := upstream.Timeout
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		client := &http.Client{Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		var resp *http.Response
		resp, err = client.Do(req)
		if err == nil {
			result.Reachable, result.StatusCode = true, resp.StatusCode
			resp.Body.Close()
		}
	}
	if err != nil {
		result.Error = err.Error()
	}
	result.LatencyMS = time.Since(start).Milliseconds()
	resource(w, 200, "upstream-health", upstream.ID, result)
}
