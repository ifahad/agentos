package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/ifahad/agentos/connectors/rest/internal/spec"
)

func callReq(args map[string]any) mcp.CallToolRequest {
	var req mcp.CallToolRequest
	req.Params.Arguments = args
	return req
}

func resultText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if len(res.Content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(res.Content))
	}
	tc, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}
	return tc.Text
}

func decodeCall(t *testing.T, res *mcp.CallToolResult) (status int, body json.RawMessage, truncated bool) {
	t.Helper()
	if res.IsError {
		t.Fatalf("tool returned error: %s", resultText(t, res))
	}
	var out struct {
		Status    int             `json:"status"`
		Body      json.RawMessage `json:"body"`
		Truncated bool            `json:"truncated"`
	}
	if err := json.Unmarshal([]byte(resultText(t, res)), &out); err != nil {
		t.Fatalf("decode call result: %v", err)
	}
	return out.Status, out.Body, out.Truncated
}

var testOps = []spec.Operation{
	{OperationID: "listItems", ToolName: "list_items", Method: "GET", Path: "/items", Summary: "List items",
		Parameters: []spec.Parameter{{Name: "kind", In: "query"}}},
	{OperationID: "getItem", ToolName: "get_item", Method: "GET", Path: "/items/{id}", Summary: "Get one item",
		Parameters: []spec.Parameter{{Name: "id", In: "path", Required: true}}},
	{OperationID: "createItem", ToolName: "create_item", Method: "POST", Path: "/items", Summary: "Create an item"},
	{OperationID: "deleteItem", ToolName: "delete_item", Method: "DELETE", Path: "/items/{id}", Summary: "Delete an item",
		Parameters: []spec.Parameter{{Name: "id", In: "path", Required: true}}},
}

func TestFilterOps(t *testing.T) {
	tests := []struct {
		name           string
		allowMutations bool
		wantTools      []string
	}{
		{"default excludes mutations", false, []string{"list_items", "get_item"}},
		{"mutations allowed includes all", true, []string{"list_items", "get_item", "create_item", "delete_item"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterOps(testOps, tt.allowMutations)
			var names []string
			for _, op := range got {
				names = append(names, op.ToolName)
			}
			if strings.Join(names, ",") != strings.Join(tt.wantTools, ",") {
				t.Errorf("filtered tools = %v, want %v", names, tt.wantTools)
			}
		})
	}
}

func TestListOperations(t *testing.T) {
	tr := New(&spec.Document{Operations: testOps}, Config{BaseURL: "http://upstream"})
	res, err := tr.ListOperations(context.Background(), callReq(nil))
	if err != nil {
		t.Fatalf("ListOperations: %v", err)
	}
	var ops []struct {
		OperationID string `json:"operation_id"`
		Method      string `json:"method"`
		Path        string `json:"path"`
		Summary     string `json:"summary"`
	}
	if err := json.Unmarshal([]byte(resultText(t, res)), &ops); err != nil {
		t.Fatalf("decode list_operations: %v", err)
	}
	if len(ops) != 2 {
		t.Fatalf("got %d operations, want 2 (GET only by default)", len(ops))
	}
	if ops[0].OperationID != "listItems" || ops[0].Method != "GET" || ops[0].Path != "/items" || ops[0].Summary != "List items" {
		t.Errorf("unexpected first operation: %+v", ops[0])
	}
}

// upstream records the last request and replies with the configured handler.
type upstream struct {
	srv     *httptest.Server
	lastReq *http.Request
}

func newUpstream(t *testing.T, handler http.HandlerFunc) *upstream {
	t.Helper()
	u := &upstream{}
	u.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u.lastReq = r.Clone(context.Background())
		handler(w, r)
	}))
	t.Cleanup(u.srv.Close)
	return u
}

func newTools(u *upstream, cfg Config) *Tools {
	cfg.BaseURL = u.srv.URL
	if cfg.IsDisallowedHost == nil {
		// httptest servers listen on loopback, which the real screen rejects;
		// allow everything here so the non-redirect tests exercise loopback.
		cfg.IsDisallowedHost = func(string) bool { return false }
	}
	return New(&spec.Document{Operations: testOps}, cfg)
}

// allowLoopback permits 127.0.0.1/localhost (the httptest servers) while
// rejecting any other host, so the redirect policy can be exercised against a
// private-IP target without real network access.
func allowLoopback(host string) bool { return host != "127.0.0.1" && host != "localhost" }

func TestRedirectCrossHostDropsAuthHeader(t *testing.T) {
	var finalGotHeader string
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		finalGotHeader = r.Header.Get("X-Api-Key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer final.Close()
	// Use localhost so the redirect target host differs from 127.0.0.1 (both
	// resolve to loopback, so the request still reaches the final server).
	finalURL := strings.Replace(final.URL, "127.0.0.1", "localhost", 1)

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, finalURL+"/final", http.StatusFound)
	}))
	defer redirector.Close()

	tr := New(&spec.Document{Operations: testOps}, Config{
		BaseURL:          redirector.URL,
		AuthHeaderName:   "X-Api-Key",
		AuthHeaderValue:  "sekret",
		IsDisallowedHost: allowLoopback,
	})
	res, err := tr.CallHandler(testOps[0])(context.Background(), callReq(nil))
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if status, _, _ := decodeCall(t, res); status != http.StatusOK {
		t.Errorf("status = %d, want 200", status)
	}
	if finalGotHeader != "" {
		t.Errorf("auth header leaked across the cross-host redirect: %q", finalGotHeader)
	}
}

func TestRedirectSameHostKeepsAuthHeader(t *testing.T) {
	var finalGotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/final" {
			finalGotHeader = r.Header.Get("X-Api-Key")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		http.Redirect(w, r, "/final", http.StatusFound)
	}))
	defer srv.Close()

	tr := New(&spec.Document{Operations: testOps}, Config{
		BaseURL:          srv.URL,
		AuthHeaderName:   "X-Api-Key",
		AuthHeaderValue:  "sekret",
		IsDisallowedHost: allowLoopback,
	})
	res, err := tr.CallHandler(testOps[0])(context.Background(), callReq(nil))
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if status, _, _ := decodeCall(t, res); status != http.StatusOK {
		t.Errorf("status = %d, want 200", status)
	}
	if finalGotHeader != "sekret" {
		t.Errorf("same-host redirect dropped the auth header: %q", finalGotHeader)
	}
}

func TestRedirectToPrivateIPRefused(t *testing.T) {
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusFound)
	}))
	defer redirector.Close()

	tr := New(&spec.Document{Operations: testOps}, Config{
		BaseURL:          redirector.URL,
		AuthHeaderName:   "X-Api-Key",
		AuthHeaderValue:  "sekret",
		IsDisallowedHost: allowLoopback,
	})
	res, err := tr.CallHandler(testOps[0])(context.Background(), callReq(nil))
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected an error result for a redirect to a private/link-local IP")
	}
}

func TestInitialHostDisallowed(t *testing.T) {
	tr := New(&spec.Document{Operations: testOps}, Config{
		BaseURL:          "http://169.254.169.254",
		IsDisallowedHost: func(host string) bool { return host == "169.254.169.254" },
	})
	res, err := tr.CallHandler(testOps[0])(context.Background(), callReq(nil))
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected an error result for a disallowed initial host")
	}
}

func TestRedirectCapped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/next", http.StatusFound) // endless same-host loop
	}))
	defer srv.Close()

	tr := New(&spec.Document{Operations: testOps}, Config{
		BaseURL:          srv.URL,
		IsDisallowedHost: allowLoopback,
	})
	res, err := tr.CallHandler(testOps[0])(context.Background(), callReq(nil))
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected an error result once the redirect cap is exceeded")
	}
}

func jsonHandler(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}
}

func TestCallPathAndQuerySubstitution(t *testing.T) {
	tests := []struct {
		name      string
		op        spec.Operation
		args      map[string]any
		wantURI   string
		wantQuery map[string]string
	}{
		{
			name:    "path parameter substituted",
			op:      testOps[1], // get_item
			args:    map[string]any{"id": "42"},
			wantURI: "/items/42",
		},
		{
			name:    "path parameter escaped",
			op:      testOps[1],
			args:    map[string]any{"id": "a b/c"},
			wantURI: "/items/a%20b%2Fc",
		},
		{
			name:      "query parameter set",
			op:        testOps[0], // list_items
			args:      map[string]any{"kind": "wid get&more"},
			wantQuery: map[string]string{"kind": "wid get&more"},
			wantURI:   "/items",
		},
		{
			name:    "optional query parameter omitted",
			op:      testOps[0],
			args:    map[string]any{},
			wantURI: "/items",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := newUpstream(t, jsonHandler(`{"ok":true}`))
			tr := newTools(u, Config{})
			res, err := tr.CallHandler(tt.op)(context.Background(), callReq(tt.args))
			if err != nil {
				t.Fatalf("call: %v", err)
			}
			status, _, _ := decodeCall(t, res)
			if status != http.StatusOK {
				t.Errorf("status = %d, want 200", status)
			}
			if got := u.lastReq.URL.EscapedPath(); got != tt.wantURI {
				t.Errorf("upstream path = %q, want %q", got, tt.wantURI)
			}
			for k, v := range tt.wantQuery {
				if got := u.lastReq.URL.Query().Get(k); got != v {
					t.Errorf("query %q = %q, want %q", k, got, v)
				}
			}
		})
	}
}

func TestCallMissingRequiredPathParam(t *testing.T) {
	u := newUpstream(t, jsonHandler(`{}`))
	tr := newTools(u, Config{})
	res, err := tr.CallHandler(testOps[1])(context.Background(), callReq(map[string]any{}))
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error result for missing path parameter")
	}
	if txt := resultText(t, res); !strings.Contains(txt, "id") {
		t.Errorf("error %q does not mention the missing parameter", txt)
	}
}

func TestCallAuthHeaderPassThrough(t *testing.T) {
	u := newUpstream(t, jsonHandler(`{}`))
	tr := newTools(u, Config{AuthHeaderName: "X-Api-Key", AuthHeaderValue: "sekret"})
	if _, err := tr.CallHandler(testOps[0])(context.Background(), callReq(nil)); err != nil {
		t.Fatalf("call: %v", err)
	}
	if got := u.lastReq.Header.Get("X-Api-Key"); got != "sekret" {
		t.Errorf("upstream X-Api-Key = %q, want %q", got, "sekret")
	}
}

func TestCallNoAuthHeaderByDefault(t *testing.T) {
	u := newUpstream(t, jsonHandler(`{}`))
	tr := newTools(u, Config{})
	if _, err := tr.CallHandler(testOps[0])(context.Background(), callReq(nil)); err != nil {
		t.Fatalf("call: %v", err)
	}
	if got := u.lastReq.Header.Get("X-Api-Key"); got != "" {
		t.Errorf("unexpected X-Api-Key header %q", got)
	}
}

func TestCallBodyHandling(t *testing.T) {
	bigJSON := `{"data":"` + strings.Repeat("x", 200) + `"}`
	tests := []struct {
		name          string
		contentType   string
		body          string
		maxBodyBytes  int
		wantStatus    int
		wantJSONBody  string // exact raw JSON expected, "" to skip
		wantStrBody   string // expected string body, "" to skip
		wantTruncated bool
	}{
		{
			name:         "json body kept as raw json",
			contentType:  "application/json",
			body:         `{"items":[1,2,3]}`,
			wantStatus:   200,
			wantJSONBody: `{"items":[1,2,3]}`,
		},
		{
			name:        "text body returned as string",
			contentType: "text/plain",
			body:        "hello, world",
			wantStatus:  200,
			wantStrBody: "hello, world",
		},
		{
			name:        "invalid json with json content type falls back to string",
			contentType: "application/json",
			body:        "{broken",
			wantStatus:  200,
			wantStrBody: "{broken",
		},
		{
			name:          "text body truncated at cap",
			contentType:   "text/plain",
			body:          strings.Repeat("a", 100),
			maxBodyBytes:  10,
			wantStatus:    200,
			wantStrBody:   strings.Repeat("a", 10),
			wantTruncated: true,
		},
		{
			name:          "json body over cap returned as truncated string",
			contentType:   "application/json",
			body:          bigJSON,
			maxBodyBytes:  16,
			wantStatus:    200,
			wantStrBody:   bigJSON[:16],
			wantTruncated: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := newUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				_, _ = w.Write([]byte(tt.body))
			})
			tr := newTools(u, Config{MaxBodyBytes: tt.maxBodyBytes})
			res, err := tr.CallHandler(testOps[0])(context.Background(), callReq(nil))
			if err != nil {
				t.Fatalf("call: %v", err)
			}
			status, body, truncated := decodeCall(t, res)
			if status != tt.wantStatus {
				t.Errorf("status = %d, want %d", status, tt.wantStatus)
			}
			if truncated != tt.wantTruncated {
				t.Errorf("truncated = %t, want %t", truncated, tt.wantTruncated)
			}
			if tt.wantJSONBody != "" && string(body) != tt.wantJSONBody {
				t.Errorf("body = %s, want %s", body, tt.wantJSONBody)
			}
			if tt.wantStrBody != "" {
				var s string
				if err := json.Unmarshal(body, &s); err != nil {
					t.Fatalf("body %s is not a JSON string: %v", body, err)
				}
				if s != tt.wantStrBody {
					t.Errorf("body string = %q, want %q", s, tt.wantStrBody)
				}
			}
		})
	}
}

func TestCallUpstreamErrorStatusPassedThrough(t *testing.T) {
	u := newUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	})
	tr := newTools(u, Config{})
	res, err := tr.CallHandler(testOps[1])(context.Background(), callReq(map[string]any{"id": "999"}))
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	status, body, _ := decodeCall(t, res)
	if status != http.StatusNotFound {
		t.Errorf("status = %d, want 404", status)
	}
	if string(body) != `{"error":"not found"}` {
		t.Errorf("body = %s, want the upstream error JSON", body)
	}
}
