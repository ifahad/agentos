package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/ifahad/agentos/connectors/soap/internal/wsdl"
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

var testOps = []wsdl.Operation{
	{Name: "Add", ToolName: "add", SOAPAction: "http://tempuri.org/Add", Doc: "Adds two integers.",
		Element: "Add", Namespace: "http://tempuri.org/", Parts: []string{"intA", "intB"}},
	{Name: "Subtract", ToolName: "subtract", SOAPAction: "http://tempuri.org/Subtract",
		Element: "Subtract", Namespace: "http://tempuri.org/", Parts: []string{"intA", "intB"}},
	{Name: "Divide", ToolName: "divide", SOAPAction: "http://tempuri.org/Divide",
		Element: "Divide", Namespace: "http://tempuri.org/", Parts: []string{"intA", "intB"}},
}

func testDef() *wsdl.Definition { return &wsdl.Definition{Operations: testOps} }

func TestFilterOps(t *testing.T) {
	tests := []struct {
		name  string
		allow []string
		want  []string
	}{
		{"empty exposes all", nil, []string{"Add", "Subtract", "Divide"}},
		{"allowlist filters", []string{"Add", "Divide"}, []string{"Add", "Divide"}},
		{"whitespace and unknown ignored", []string{" Add ", "Nope"}, []string{"Add"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterOps(testOps, tt.allow)
			var names []string
			for _, op := range got {
				names = append(names, op.Name)
			}
			if strings.Join(names, ",") != strings.Join(tt.want, ",") {
				t.Errorf("filtered = %v, want %v", names, tt.want)
			}
		})
	}
}

func TestListOperations(t *testing.T) {
	tr := New(testDef(), Config{Endpoint: "http://upstream", AllowOperations: []string{"Add"}})
	res, err := tr.ListOperations(context.Background(), callReq(nil))
	if err != nil {
		t.Fatalf("ListOperations: %v", err)
	}
	var ops []struct {
		Name       string `json:"name"`
		SOAPAction string `json:"soap_action"`
		Doc        string `json:"doc"`
	}
	if err := json.Unmarshal([]byte(resultText(t, res)), &ops); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(ops) != 1 {
		t.Fatalf("got %d operations, want 1 (allowlist)", len(ops))
	}
	if ops[0].Name != "Add" || ops[0].SOAPAction != "http://tempuri.org/Add" || ops[0].Doc != "Adds two integers." {
		t.Errorf("unexpected op %+v", ops[0])
	}
}

func TestBuildEnvelope(t *testing.T) {
	tests := []struct {
		name     string
		op       wsdl.Operation
		args     map[string]string
		xmlBody  string
		contains []string
		absent   []string
	}{
		{
			name:     "part args in order",
			op:       testOps[0],
			args:     map[string]string{"intA": "3", "intB": "4"},
			contains: []string{`<Add xmlns="http://tempuri.org/">`, "<intA>3</intA>", "<intB>4</intB>", "</Add>"},
		},
		{
			name:     "values are xml-escaped",
			op:       testOps[0],
			args:     map[string]string{"intA": "a<b&c", "intB": ""},
			contains: []string{"<intA>a&lt;b&amp;c</intA>"},
			absent:   []string{"<intB>"}, // empty value omitted
		},
		{
			name:     "xml_body escape hatch replaces parts",
			op:       testOps[0],
			args:     map[string]string{"intA": "3", "intB": "4"},
			xmlBody:  "<intA>99</intA><nested><x>1</x></nested>",
			contains: []string{"<intA>99</intA><nested><x>1</x></nested>", "</Add>"},
			absent:   []string{"<intB>", "<intA>3</intA>"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := BuildEnvelope(tt.op, tt.args, tt.xmlBody)
			if !strings.HasPrefix(env, `<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"><soapenv:Body>`) {
				t.Errorf("envelope prefix wrong: %s", env)
			}
			for _, c := range tt.contains {
				if !strings.Contains(env, c) {
					t.Errorf("envelope missing %q:\n%s", c, env)
				}
			}
			for _, a := range tt.absent {
				if strings.Contains(env, a) {
					t.Errorf("envelope should not contain %q:\n%s", a, env)
				}
			}
		})
	}
}

// echoServer records the last request and replies with the configured handler.
type echoServer struct {
	srv      *httptest.Server
	lastReq  *http.Request
	lastBody string
}

func newEcho(t *testing.T, handler http.HandlerFunc) *echoServer {
	t.Helper()
	e := &echoServer{}
	e.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf, _ := io.ReadAll(r.Body)
		e.lastBody = string(buf)
		e.lastReq = r.Clone(context.Background())
		handler(w, r)
	}))
	t.Cleanup(e.srv.Close)
	return e
}

func newTools(e *echoServer, cfg Config) *Tools {
	cfg.Endpoint = e.srv.URL
	return New(testDef(), cfg)
}

const addResponse = `<?xml version="1.0" encoding="utf-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <AddResponse xmlns="http://tempuri.org/">
      <AddResult>7</AddResult>
    </AddResponse>
  </soap:Body>
</soap:Envelope>`

func xmlHandler(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func TestCallEnvelopeAndHeaders(t *testing.T) {
	e := newEcho(t, xmlHandler(200, addResponse))
	tr := newTools(e, Config{})
	res, err := tr.CallHandler(testOps[0])(context.Background(), callReq(map[string]any{"intA": "3", "intB": "4"}))
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	status, body, _ := decodeCall(t, res)
	if status != http.StatusOK {
		t.Errorf("status = %d, want 200", status)
	}

	// SOAPAction header must be quoted.
	if got := e.lastReq.Header.Get("SOAPAction"); got != `"http://tempuri.org/Add"` {
		t.Errorf("SOAPAction = %q, want quoted action", got)
	}
	if ct := e.lastReq.Header.Get("Content-Type"); ct != "text/xml; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	// Envelope shape reached the upstream.
	for _, want := range []string{"<soapenv:Body>", `<Add xmlns="http://tempuri.org/">`, "<intA>3</intA>", "<intB>4</intB>"} {
		if !strings.Contains(e.lastBody, want) {
			t.Errorf("upstream body missing %q:\n%s", want, e.lastBody)
		}
	}
	// Response XML is parsed into a structure, not a raw string.
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("body is not a JSON object: %s", body)
	}
	env, _ := parsed["Envelope"].(map[string]any)
	if env == nil {
		t.Fatalf("parsed body missing Envelope: %s", body)
	}
}

func TestCallXMLBodyEscapeHatch(t *testing.T) {
	e := newEcho(t, xmlHandler(200, addResponse))
	tr := newTools(e, Config{})
	_, err := tr.CallHandler(testOps[0])(context.Background(), callReq(map[string]any{
		"intA":     "3", // must be ignored
		"xml_body": "<intA>10</intA><intB>20</intB>",
	}))
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if !strings.Contains(e.lastBody, "<intA>10</intA><intB>20</intB>") {
		t.Errorf("xml_body not used verbatim:\n%s", e.lastBody)
	}
	if strings.Contains(e.lastBody, "<intA>3</intA>") {
		t.Errorf("part args should be ignored when xml_body is set:\n%s", e.lastBody)
	}
}

func TestCallAuthHeaderPassThrough(t *testing.T) {
	e := newEcho(t, xmlHandler(200, addResponse))
	tr := newTools(e, Config{AuthHeaderName: "Authorization", AuthHeaderValue: "Bearer sekret"})
	if _, err := tr.CallHandler(testOps[0])(context.Background(), callReq(nil)); err != nil {
		t.Fatalf("call: %v", err)
	}
	if got := e.lastReq.Header.Get("Authorization"); got != "Bearer sekret" {
		t.Errorf("Authorization = %q, want the auth header", got)
	}
}

func TestCallNoAuthHeaderByDefault(t *testing.T) {
	e := newEcho(t, xmlHandler(200, addResponse))
	tr := newTools(e, Config{})
	if _, err := tr.CallHandler(testOps[0])(context.Background(), callReq(nil)); err != nil {
		t.Fatalf("call: %v", err)
	}
	if got := e.lastReq.Header.Get("Authorization"); got != "" {
		t.Errorf("unexpected Authorization header %q", got)
	}
}

func TestCallBodyHandling(t *testing.T) {
	tests := []struct {
		name          string
		contentType   string
		body          string
		maxBodyBytes  int
		wantParsedXML bool
		wantStrBody   string // when non-XML/truncated; "" to skip
		wantTruncated bool
	}{
		{
			name:          "xml body parsed to structure",
			contentType:   "text/xml",
			body:          `<Result><Value>42</Value></Result>`,
			wantParsedXML: true,
		},
		{
			name:          "xml detected by leading angle bracket without content type",
			contentType:   "",
			body:          `<Result>ok</Result>`,
			wantParsedXML: true,
		},
		{
			name:        "non-xml body returned as string",
			contentType: "text/plain",
			body:        "plain text",
			wantStrBody: "plain text",
		},
		{
			name:          "over-cap body returned as truncated string",
			contentType:   "text/xml",
			body:          "<Result>" + strings.Repeat("a", 100) + "</Result>",
			maxBodyBytes:  10,
			wantStrBody:   ("<Result>" + strings.Repeat("a", 100) + "</Result>")[:10],
			wantTruncated: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEcho(t, func(w http.ResponseWriter, _ *http.Request) {
				if tt.contentType != "" {
					w.Header().Set("Content-Type", tt.contentType)
				}
				_, _ = w.Write([]byte(tt.body))
			})
			tr := newTools(e, Config{MaxBodyBytes: tt.maxBodyBytes})
			res, err := tr.CallHandler(testOps[0])(context.Background(), callReq(nil))
			if err != nil {
				t.Fatalf("call: %v", err)
			}
			_, body, truncated := decodeCall(t, res)
			if truncated != tt.wantTruncated {
				t.Errorf("truncated = %t, want %t", truncated, tt.wantTruncated)
			}
			if tt.wantParsedXML {
				var m map[string]any
				if err := json.Unmarshal(body, &m); err != nil {
					t.Fatalf("expected parsed XML object, got %s", body)
				}
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

const faultResponse = `<?xml version="1.0" encoding="utf-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <soap:Fault>
      <faultcode>soap:Server</faultcode>
      <faultstring>Cannot divide by zero.</faultstring>
    </soap:Fault>
  </soap:Body>
</soap:Envelope>`

func TestCallSOAPFaultReturnedNotErrored(t *testing.T) {
	e := newEcho(t, xmlHandler(http.StatusInternalServerError, faultResponse))
	tr := newTools(e, Config{})
	res, err := tr.CallHandler(testOps[2])(context.Background(), callReq(map[string]any{"intA": "1", "intB": "0"}))
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool errored on SOAP Fault, want a normal result: %s", resultText(t, res))
	}
	status, body, _ := decodeCall(t, res)
	if status != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", status)
	}
	if !strings.Contains(string(body), "Cannot divide by zero.") {
		t.Errorf("fault body not returned: %s", body)
	}
}

func TestCallUpstreamUnreachable(t *testing.T) {
	// Point at a closed server to force a transport error → tool error result.
	tr := New(testDef(), Config{Endpoint: "http://127.0.0.1:1"})
	res, err := tr.CallHandler(testOps[0])(context.Background(), callReq(map[string]any{"intA": "1", "intB": "2"}))
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected a tool error for an unreachable upstream")
	}
}
