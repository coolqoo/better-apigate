package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coolqoo/better-apigate/domain/proxy"
	"github.com/coolqoo/better-apigate/domain/route"
)

// RouteDraft lets the editor execute unsaved changes through the gateway's
// existing matcher and transformation engine.
type RouteDraft struct {
	ID                string              `json:"id"`
	Name              string              `json:"name"`
	PathPattern       string              `json:"path_pattern"`
	MatchType         route.MatchType     `json:"match_type"`
	Methods           []string            `json:"methods"`
	Headers           []route.HeaderMatch `json:"headers"`
	HostPattern       string              `json:"host_pattern"`
	HostMatchType     route.HostMatchType `json:"host_match_type"`
	UpstreamID        string              `json:"upstream_id"`
	PathRewrite       string              `json:"path_rewrite"`
	MethodOverride    string              `json:"method_override"`
	RequestTransform  *route.Transform    `json:"request_transform"`
	ResponseTransform *route.Transform    `json:"response_transform"`
	MeteringExpr      string              `json:"metering_expr"`
	MeteringUnit      string              `json:"metering_unit"`
	Protocol          route.Protocol      `json:"protocol"`
	UnitCost          int64               `json:"unit_cost"`
	AuthRequired      bool                `json:"auth_required"`
}

func (d RouteDraft) route() route.Route {
	return route.Route{ID: d.ID, Name: d.Name, PathPattern: d.PathPattern,
		MatchType: d.MatchType, Methods: d.Methods, Headers: d.Headers,
		HostPattern: d.HostPattern, HostMatchType: d.HostMatchType,
		UpstreamID: d.UpstreamID, PathRewrite: d.PathRewrite,
		MethodOverride: d.MethodOverride, RequestTransform: d.RequestTransform,
		ResponseTransform: d.ResponseTransform, MeteringExpr: d.MeteringExpr,
		MeteringUnit: d.MeteringUnit, Protocol: d.Protocol, UnitCost: d.UnitCost,
		Enabled: true, AuthRequired: d.AuthRequired}
}

// ExecuteRoute sends an administrator's request to the configured upstream,
// then measures and transforms the actual response. It does not debit a customer wallet.
func (s *RouteService) ExecuteRoute(ctx context.Context, in RouteTestRequest, transforms *TransformService, auth *proxy.AuthContext) RouteTestResult {
	result := RouteTestResult{}
	if in.Method == "" {
		in.Method = "GET"
	}
	u, err := url.ParseRequestURI(in.Path)
	if err != nil || !strings.HasPrefix(in.Path, "/") {
		result.Error = "Enter a request path starting with /."
		return result
	}
	headers := make(map[string]string, len(in.Headers))
	for k, v := range in.Headers {
		headers[k] = v
	}
	req := proxy.Request{Method: in.Method, Path: u.Path, Query: u.RawQuery, Headers: headers, Body: []byte(in.Body)}
	cache := s.cache.Load()
	if cache == nil || cache.Matcher == nil {
		result.Error = "Route service not initialized"
		return result
	}
	var rt *route.Route
	var params map[string]string
	if in.Draft != nil {
		draft := in.Draft.route()
		matcher, e := route.NewMatcher([]route.Route{draft})
		if e != nil {
			result.Error = e.Error()
			return result
		}
		if match := matcher.Match(req.Method, req.Path, req.Headers); match != nil {
			rt, params = match.Route, match.PathParams
			result.MatchReason = "Matched the current editor configuration"
		}
	} else if in.RouteID != "" {
		for i := range cache.Routes {
			if cache.Routes[i].ID == in.RouteID {
				rt = &cache.Routes[i]
				break
			}
		}
		if rt == nil {
			result.Error = "Route not found"
			return result
		}
		result.MatchReason = "Tested directly by route ID"
		if matcher, e := route.NewMatcher([]route.Route{*rt}); e == nil {
			if match := matcher.Match(req.Method, req.Path, req.Headers); match != nil {
				params = match.PathParams
			}
		}
	} else if match := cache.Matcher.Match(req.Method, req.Path, req.Headers); match != nil {
		rt, params = match.Route, match.PathParams
		result.MatchReason = "Matched by pattern: " + string(rt.MatchType) + " " + rt.PathPattern
	}
	if rt == nil {
		result.MatchReason = "No route matched the method, path, host and header conditions"
		return result
	}
	result.Matched, result.RouteID, result.RouteName, result.PathParams = true, rt.ID, rt.Name, params
	if transforms == nil {
		result.Error = "Transformation service unavailable"
		return result
	}
	req, err = transforms.TransformRequest(ctx, req, rt.RequestTransform, auth)
	if err == nil && rt.PathRewrite != "" {
		req.Path, err = transforms.EvalString(ctx, rt.PathRewrite, map[string]any{"path": req.Path, "method": req.Method, "pathParams": params})
	}
	if err != nil {
		result.Error = "Request transformation: " + err.Error()
		return result
	}
	if rt.MethodOverride != "" {
		req.Method = rt.MethodOverride
	}
	upstream, ok := cache.Upstreams[rt.UpstreamID]
	if ok {
		result.UpstreamName = upstream.Name
		req.Headers = s.ApplyUpstreamAuth(&upstream, req.Headers)
		if target, e := s.ResolveUpstreamURL(&upstream, req.Path, req.Query); e == nil {
			result.UpstreamURL = target.String()
		} else {
			result.Error = e.Error()
		}
	} else {
		result.Error = "Select an available upstream"
	}
	result.TransformedMethod, result.TransformedPath = req.Method, req.Path
	result.TransformedQuery, result.TransformedHeaders, result.TransformedBody = req.Query, req.Headers, string(req.Body)
	result.UnitCost, result.MeteringUnit, result.MeteringExpr = max(rt.UnitCost, 1), rt.MeteringUnit, rt.MeteringExpr
	result.AuthRequired = rt.AuthRequired
	if result.MeteringExpr == "" {
		result.MeteringExpr = "1"
	}
	if result.MeteringUnit == "" {
		result.MeteringUnit = "requests"
	}
	if result.Error != "" {
		return result
	}
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, result.UpstreamURL, bytes.NewReader(req.Body))
	if err != nil {
		result.Error = "Request: " + err.Error()
		return result
	}
	for name, value := range req.Headers {
		if strings.EqualFold(name, "Host") {
			httpReq.Host = value
		} else {
			httpReq.Header.Set(name, value)
		}
	}
	client := s.BuildUpstreamClient(&upstream)
	if client.Timeout <= 0 {
		client.Timeout = 30 * time.Second
	}
	defer client.CloseIdleConnections()
	start := time.Now()
	httpResp, err := client.Do(httpReq)
	if err != nil {
		result.LatencyMS = time.Since(start).Milliseconds()
		result.Error = "Upstream request: " + err.Error()
		return result
	}
	defer httpResp.Body.Close()
	responseHeaders := make(map[string]string, len(httpResp.Header))
	for name, values := range httpResp.Header {
		responseHeaders[name] = strings.Join(values, ", ")
	}
	const maxResponseBytes = 50 << 20
	responseBody, readErr := io.ReadAll(io.LimitReader(httpResp.Body, maxResponseBytes+1))
	result.LatencyMS = time.Since(start).Milliseconds()
	result.ResponseStatus = httpResp.StatusCode
	result.UpstreamResponseHeaders, result.UpstreamResponseBody = responseHeaders, string(responseBody)
	if len(responseBody) > maxResponseBytes {
		result.UpstreamResponseBody = string(responseBody[:maxResponseBytes])
		result.Error = "Upstream response exceeds the 50 MB gateway limit"
		return result
	}
	if readErr != nil {
		result.Error = "Read upstream response: " + readErr.Error()
		return result
	}
	resp := proxy.Response{Status: httpResp.StatusCode, Headers: make(map[string]string, len(responseHeaders)), Body: responseBody}
	for name, value := range responseHeaders {
		resp.Headers[name] = value
	}
	measured, meteringErr := transforms.MeasureResponse(ctx, result.MeteringExpr, resp, rt.Protocol, auth, proxy.Request{Method: req.Method, Path: u.Path, Body: req.Body})
	if meteringErr != nil {
		result.Error = "Metering expression: " + meteringErr.Error()
	} else {
		result.MeteredValue = &measured
	}
	resp, err = transforms.TransformResponse(ctx, resp, rt.ResponseTransform, auth)
	if err != nil {
		result.Error = strings.TrimSpace(result.Error + " " + fmt.Sprintf("Response transformation: %v", err))
		return result
	}
	result.ResponseHeaders, result.ResponseBody = resp.Headers, string(resp.Body)
	return result
}
