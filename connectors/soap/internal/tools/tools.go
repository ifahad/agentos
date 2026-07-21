// Package tools registers the agentos-soap MCP tools: list_operations plus one
// tool per allowed WSDL operation. Each operation tool takes a string arg per
// top-level input part, plus an xml_body escape hatch (a raw inner body used
// verbatim for nested/complex types, in which case the per-part args are
// ignored). A tool builds a SOAP 1.1 envelope, POSTs it to the endpoint with
// the SOAPAction header, and returns {"status": N, "body": ...}.
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/ifahad/agentos/connectors/soap/internal/safehttp"
	"github.com/ifahad/agentos/connectors/soap/internal/wsdl"
)

const (
	// DefaultTimeoutSeconds is the upstream request timeout applied when
	// AGENTOS_SOAP_TIMEOUT_S is unset.
	DefaultTimeoutSeconds = 20
	// DefaultMaxBodyBytes is the response-body cap applied when
	// AGENTOS_SOAP_MAX_BODY_BYTES is unset.
	DefaultMaxBodyBytes = 131072

	// xmlBodyArg is the escape-hatch argument name.
	xmlBodyArg = "xml_body"

	soapEnvelopeNS  = "http://schemas.xmlsoap.org/soap/envelope/"
	soapContentType = "text/xml; charset=utf-8"
)

// Config carries the connector settings derived from the environment.
type Config struct {
	// Endpoint is the SOAP service URL every operation is POSTed to.
	Endpoint string
	// AllowOperations, when non-empty, restricts the exposed operations to the
	// listed names; empty exposes all operations.
	AllowOperations []string
	// AuthHeaderName/AuthHeaderValue, when set, are attached to every upstream
	// request.
	AuthHeaderName  string
	AuthHeaderValue string
	// MaxBodyBytes caps the upstream response body. <= 0 falls back to
	// DefaultMaxBodyBytes.
	MaxBodyBytes int
	// Timeout is the upstream request timeout. <= 0 falls back to
	// DefaultTimeoutSeconds.
	Timeout time.Duration
	// Client is the upstream HTTP client; nil gets a Timeout-bounded default
	// hardened with a redirect policy (see safehttp.NewClient).
	Client *http.Client
	// IsDisallowedHost screens the initial endpoint host and every redirect
	// target for private/loopback/link-local addresses. nil defaults to
	// safehttp.IsDisallowedHost; tests inject an override so loopback httptest
	// servers still work.
	IsDisallowedHost func(host string) bool
}

// Tools holds the exposed operations and shared dependencies of the
// agentos-soap MCP tool handlers.
type Tools struct {
	cfg Config
	ops []wsdl.Operation
}

// FilterOps returns the operations to expose: all of them when allow is empty,
// otherwise only those whose name appears in allow.
func FilterOps(ops []wsdl.Operation, allow []string) []wsdl.Operation {
	if len(allow) == 0 {
		return ops
	}
	allowed := make(map[string]bool, len(allow))
	for _, a := range allow {
		if a = strings.TrimSpace(a); a != "" {
			allowed[a] = true
		}
	}
	included := make([]wsdl.Operation, 0, len(ops))
	for _, op := range ops {
		if allowed[op.Name] {
			included = append(included, op)
		}
	}
	return included
}

// New returns a Tools exposing def's operations filtered per
// cfg.AllowOperations.
func New(def *wsdl.Definition, cfg Config) *Tools {
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = DefaultMaxBodyBytes
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultTimeoutSeconds * time.Second
	}
	if cfg.IsDisallowedHost == nil {
		cfg.IsDisallowedHost = safehttp.IsDisallowedHost
	}
	if cfg.Client == nil {
		cfg.Client = safehttp.NewClient(cfg.Timeout, cfg.AuthHeaderName, cfg.IsDisallowedHost)
	}
	return &Tools{cfg: cfg, ops: FilterOps(def.Operations, cfg.AllowOperations)}
}

// Operations returns the exposed operations, in registration order.
func (t *Tools) Operations() []wsdl.Operation {
	return t.ops
}

// Register adds list_operations and one tool per exposed operation to s.
func (t *Tools) Register(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("list_operations",
		mcp.WithDescription("List the SOAP operations exposed by this connector. Returns a JSON array of {name, soap_action, doc} objects."),
	), t.ListOperations)

	for _, op := range t.ops {
		op := op
		desc := op.Doc
		if desc == "" {
			desc = fmt.Sprintf("SOAP operation %s (action %q)", op.Name, op.SOAPAction)
		}
		opts := []mcp.ToolOption{mcp.WithDescription(desc)}
		for _, part := range op.Parts {
			opts = append(opts, mcp.WithString(part,
				mcp.Description(fmt.Sprintf("String value for input part %q of %s", part, op.Name)),
			))
		}
		opts = append(opts, mcp.WithString(xmlBodyArg,
			mcp.Description("Escape hatch: raw XML used verbatim as the inner body of the operation element (for nested/complex types). When set, the per-part args above are ignored."),
		))
		s.AddTool(mcp.NewTool(op.ToolName, opts...), t.CallHandler(op))
	}
}

type operationSummary struct {
	Name       string `json:"name"`
	SOAPAction string `json:"soap_action"`
	Doc        string `json:"doc"`
}

// ListOperations implements the list_operations tool.
func (t *Tools) ListOperations(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	out := make([]operationSummary, 0, len(t.ops))
	for _, op := range t.ops {
		out = append(out, operationSummary{
			Name:       op.Name,
			SOAPAction: op.SOAPAction,
			Doc:        op.Doc,
		})
	}
	return jsonResult(out)
}

type callResult struct {
	Status    int  `json:"status"`
	Body      any  `json:"body"`
	Truncated bool `json:"truncated,omitempty"`
}

// CallHandler returns the MCP handler that POSTs op's SOAP envelope upstream.
func (t *Tools) CallHandler(op wsdl.Operation) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		xmlBody := strings.TrimSpace(req.GetString(xmlBodyArg, ""))
		args := map[string]string{}
		for _, part := range op.Parts {
			args[part] = req.GetString(part, "")
		}
		envelope := BuildEnvelope(op, args, xmlBody)

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, t.cfg.Endpoint, strings.NewReader(envelope))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build request: %v", err)), nil
		}
		if host := httpReq.URL.Hostname(); t.cfg.IsDisallowedHost(host) {
			return mcp.NewToolResultError(fmt.Sprintf("upstream host %q is not permitted", host)), nil
		}
		httpReq.Header.Set("Content-Type", soapContentType)
		httpReq.Header.Set("SOAPAction", `"`+op.SOAPAction+`"`)
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

		// A SOAP Fault is just XML with (usually) a 500 status; return it
		// normally rather than erroring the tool.
		res := callResult{Status: resp.StatusCode, Truncated: truncated}
		if !truncated && looksXML(resp.Header.Get("Content-Type"), body) {
			if parsed, ok := xmlToValue(body); ok {
				res.Body = parsed
			} else {
				res.Body = string(body)
			}
		} else {
			// Non-XML, unparseable, or over-cap bodies are returned as a
			// (possibly truncated) raw string.
			res.Body = string(body)
		}
		return jsonResult(res)
	}
}

// BuildEnvelope constructs a SOAP 1.1 envelope for op. When xmlBody is
// non-empty it is used verbatim as the inner body of the operation element and
// args are ignored; otherwise one element per declared part (in order, when a
// value is present) is emitted.
func BuildEnvelope(op wsdl.Operation, args map[string]string, xmlBody string) string {
	var b strings.Builder
	b.WriteString(`<soapenv:Envelope xmlns:soapenv="`)
	b.WriteString(soapEnvelopeNS)
	b.WriteString(`"><soapenv:Body>`)

	element := op.Element
	if element == "" {
		element = op.Name
	}
	b.WriteString("<")
	b.WriteString(element)
	if op.Namespace != "" {
		b.WriteString(` xmlns="`)
		writeEscaped(&b, op.Namespace)
		b.WriteString(`"`)
	}
	b.WriteString(">")

	if strings.TrimSpace(xmlBody) != "" {
		b.WriteString(xmlBody)
	} else {
		for _, part := range op.Parts {
			v, ok := args[part]
			if !ok || v == "" {
				continue
			}
			b.WriteString("<")
			b.WriteString(part)
			b.WriteString(">")
			writeEscaped(&b, v)
			b.WriteString("</")
			b.WriteString(part)
			b.WriteString(">")
		}
	}

	b.WriteString("</")
	b.WriteString(element)
	b.WriteString("></soapenv:Body></soapenv:Envelope>")
	return b.String()
}

func writeEscaped(b *strings.Builder, s string) {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	b.Write(buf.Bytes())
}

// looksXML reports whether body should be treated as XML, by Content-Type or
// by a leading '<'.
func looksXML(contentType string, body []byte) bool {
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	if strings.Contains(mediaType, "xml") {
		return true
	}
	trimmed := bytes.TrimSpace(body)
	return len(trimmed) > 0 && trimmed[0] == '<'
}

// xmlToValue parses well-formed XML into a generic, JSON-encodable value:
// {rootLocalName: <node>}, where a node is its trimmed text (leaf) or a map of
// child local names -> node (repeated children become arrays; attributes are
// keyed "@name"; mixed text is keyed "#text"). Returns false if the body is not
// well-formed XML.
func xmlToValue(data []byte) (any, bool) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, false
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		node, err := parseElem(dec, start)
		if err != nil {
			return nil, false
		}
		return map[string]any{start.Name.Local: node}, true
	}
}

func parseElem(dec *xml.Decoder, start xml.StartElement) (any, error) {
	children := map[string]any{}
	attrs := map[string]any{}
	var text strings.Builder
	hasChild := false

	for _, a := range start.Attr {
		if a.Name.Local == "xmlns" || a.Name.Space == "xmlns" || a.Name.Space == "http://www.w3.org/2000/xmlns/" {
			continue
		}
		attrs["@"+a.Name.Local] = a.Value
	}

	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			hasChild = true
			v, err := parseElem(dec, t)
			if err != nil {
				return nil, err
			}
			addChild(children, t.Name.Local, v)
		case xml.CharData:
			text.Write(t)
		case xml.EndElement:
			trimmed := strings.TrimSpace(text.String())
			if !hasChild && len(attrs) == 0 {
				return trimmed, nil
			}
			m := make(map[string]any, len(children)+len(attrs)+1)
			for k, v := range children {
				m[k] = v
			}
			for k, v := range attrs {
				m[k] = v
			}
			if trimmed != "" {
				m["#text"] = trimmed
			}
			return m, nil
		}
	}
}

// addChild adds v under key, promoting to a slice when the key repeats.
func addChild(m map[string]any, key string, v any) {
	existing, ok := m[key]
	if !ok {
		m[key] = v
		return
	}
	if slice, ok := existing.([]any); ok {
		m[key] = append(slice, v)
		return
	}
	m[key] = []any{existing, v}
}

func jsonResult(v any) (*mcp.CallToolResult, error) {
	out, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("encode result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(out)), nil
}
