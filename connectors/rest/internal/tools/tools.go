// Package tools registers the agentos-rest MCP tools: list_operations plus
// one proxy tool per included spec operation. Phase 3 supports path and query
// parameters only — request bodies are not forwarded (mutations are gated off
// by default anyway).
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/ifahad/agentos/connectors/rest/internal/safehttp"
	"github.com/ifahad/agentos/connectors/rest/internal/spec"
)

// DefaultMaxBodyBytes is the response-body cap applied when
// AGENTOS_REST_MAX_BODY_BYTES is unset.
const DefaultMaxBodyBytes = 65536

// Config carries the connector settings derived from the environment.
type Config struct {
	// BaseURL is the upstream API root every operation path is appended to.
	BaseURL string
	// AllowMutations includes non-GET operations when true.
	AllowMutations bool
	// AuthHeaderName/AuthHeaderValue, when set, are attached to every
	// upstream request.
	AuthHeaderName  string
	AuthHeaderValue string
	// MaxBodyBytes caps the upstream response body. <= 0 falls back to
	// DefaultMaxBodyBytes.
	MaxBodyBytes int
	// Client is the upstream HTTP client; nil gets a 30 s-timeout default
	// hardened with a redirect policy (see safehttp.NewClient).
	Client *http.Client
	// IsDisallowedHost screens the initial upstream host and every redirect
	// target for private/loopback/link-local addresses. nil defaults to
	// safehttp.IsDisallowedHost; tests inject an override so loopback httptest
	// servers still work.
	IsDisallowedHost func(host string) bool
}

// Tools holds the included operations and shared dependencies of the
// agentos-rest MCP tool handlers.
type Tools struct {
	cfg Config
	ops []spec.Operation
}

// FilterOps returns the operations to expose: all of them when
// allowMutations, otherwise GET operations only.
func FilterOps(ops []spec.Operation, allowMutations bool) []spec.Operation {
	if allowMutations {
		return ops
	}
	included := make([]spec.Operation, 0, len(ops))
	for _, op := range ops {
		if op.Method == http.MethodGet {
			included = append(included, op)
		}
	}
	return included
}

// New returns a Tools exposing the operations in doc, filtered per
// cfg.AllowMutations.
func New(doc *spec.Document, cfg Config) *Tools {
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = DefaultMaxBodyBytes
	}
	if cfg.IsDisallowedHost == nil {
		cfg.IsDisallowedHost = safehttp.IsDisallowedHost
	}
	if cfg.Client == nil {
		cfg.Client = safehttp.NewClient(30*time.Second, cfg.AuthHeaderName, cfg.IsDisallowedHost)
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	return &Tools{cfg: cfg, ops: FilterOps(doc.Operations, cfg.AllowMutations)}
}

// Operations returns the included operations, in registration order.
func (t *Tools) Operations() []spec.Operation {
	return t.ops
}

// Register adds list_operations and one tool per included operation to s.
func (t *Tools) Register(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("list_operations",
		mcp.WithDescription("List the upstream REST operations exposed by this connector. Returns a JSON array of {operation_id, method, path, summary} objects."),
	), t.ListOperations)

	for _, op := range t.ops {
		op := op
		desc := op.Summary
		if desc == "" {
			desc = fmt.Sprintf("%s %s", op.Method, op.Path)
		}
		opts := []mcp.ToolOption{mcp.WithDescription(desc)}
		for _, p := range op.Parameters {
			strOpts := []mcp.PropertyOption{
				mcp.Description(fmt.Sprintf("%s parameter %q of %s %s", p.In, p.Name, op.Method, op.Path)),
			}
			if p.Required {
				strOpts = append(strOpts, mcp.Required())
			}
			opts = append(opts, mcp.WithString(p.Name, strOpts...))
		}
		s.AddTool(mcp.NewTool(op.ToolName, opts...), t.CallHandler(op))
	}
}

type operationSummary struct {
	OperationID string `json:"operation_id"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	Summary     string `json:"summary"`
}

// ListOperations implements the list_operations tool.
func (t *Tools) ListOperations(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	out := make([]operationSummary, 0, len(t.ops))
	for _, op := range t.ops {
		out = append(out, operationSummary{
			OperationID: op.OperationID,
			Method:      op.Method,
			Path:        op.Path,
			Summary:     op.Summary,
		})
	}
	return jsonResult(out)
}

type callResult struct {
	Status    int  `json:"status"`
	Body      any  `json:"body"`
	Truncated bool `json:"truncated,omitempty"`
}

// CallHandler returns the MCP handler proxying op to the upstream API.
func (t *Tools) CallHandler(op spec.Operation) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		target, err := t.buildURL(op, req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		httpReq, err := http.NewRequestWithContext(ctx, op.Method, target, nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build request: %v", err)), nil
		}
		if host := httpReq.URL.Hostname(); t.cfg.IsDisallowedHost(host) {
			return mcp.NewToolResultError(fmt.Sprintf("upstream host %q is not permitted", host)), nil
		}
		if t.cfg.AuthHeaderName != "" {
			httpReq.Header.Set(t.cfg.AuthHeaderName, t.cfg.AuthHeaderValue)
		}

		resp, err := t.cfg.Client.Do(httpReq)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("upstream request: %v", err)), nil
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(io.LimitReader(resp.Body, int64(t.cfg.MaxBodyBytes)+1))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("read upstream response: %v", err)), nil
		}
		truncated := len(body) > t.cfg.MaxBodyBytes
		if truncated {
			body = body[:t.cfg.MaxBodyBytes]
		}

		res := callResult{Status: resp.StatusCode, Truncated: truncated}
		contentType := resp.Header.Get("Content-Type")
		if !truncated && isJSON(contentType) && json.Valid(body) {
			res.Body = json.RawMessage(body)
		} else {
			// Text bodies, invalid JSON, and JSON bodies over the cap are
			// returned as a (possibly truncated) raw string.
			res.Body = string(body)
		}
		return jsonResult(res)
	}
}

// buildURL substitutes path parameters (url.PathEscape) into op.Path and
// appends query parameters (url.Values), honoring required flags.
func (t *Tools) buildURL(op spec.Operation, req mcp.CallToolRequest) (string, error) {
	path := op.Path
	query := url.Values{}
	for _, p := range op.Parameters {
		val := req.GetString(p.Name, "")
		switch p.In {
		case "path":
			// Path placeholders cannot be left unfilled, whatever the
			// spec's required flag says.
			if val == "" {
				return "", fmt.Errorf("missing required path parameter %q", p.Name)
			}
			path = strings.ReplaceAll(path, "{"+p.Name+"}", url.PathEscape(val))
		case "query":
			if val == "" {
				if p.Required {
					return "", fmt.Errorf("missing required query parameter %q", p.Name)
				}
				continue
			}
			query.Set(p.Name, val)
		}
	}
	target := t.cfg.BaseURL + path
	if encoded := query.Encode(); encoded != "" {
		target += "?" + encoded
	}
	return target, nil
}

func isJSON(contentType string) bool {
	mediaType := strings.TrimSpace(strings.Split(contentType, ";")[0])
	return strings.EqualFold(mediaType, "application/json") ||
		strings.HasSuffix(strings.ToLower(mediaType), "+json")
}

func jsonResult(v any) (*mcp.CallToolResult, error) {
	out, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("encode result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(out)), nil
}
