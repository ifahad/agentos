// Package spec loads and parses an OpenAPI 3 JSON document into the flat
// operation list the agentos-rest connector exposes as MCP tools. Parsing is
// deliberately minimal (plain encoding/json structs, no OpenAPI library): only
// servers, paths, methods, operationId, summary and path/query parameters are
// read.
package spec

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"unicode"
)

// httpMethods are the OpenAPI path-item keys treated as operations. Other
// keys (parameters, summary, description, servers, ...) are ignored.
var httpMethods = []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}

// Parameter is a path or query parameter of an operation.
type Parameter struct {
	Name     string `json:"name"`
	In       string `json:"in"`
	Required bool   `json:"required"`
}

// Operation is one method+path pair from the spec, ready to be registered as
// an MCP tool.
type Operation struct {
	OperationID string
	ToolName    string // OperationID snake_cased
	Method      string // upper-case HTTP method
	Path        string // spec path template, e.g. /customers/{id}
	Summary     string
	Parameters  []Parameter // path and query parameters only
}

// Document is the parsed spec: server URLs plus the flat operation list.
type Document struct {
	Servers    []string
	Operations []Operation
}

type rawSpec struct {
	Servers []struct {
		URL string `json:"url"`
	} `json:"servers"`
	Paths map[string]map[string]json.RawMessage `json:"paths"`
}

type rawOperation struct {
	OperationID string      `json:"operationId"`
	Summary     string      `json:"summary"`
	Parameters  []Parameter `json:"parameters"`
}

// Load fetches the spec from specURL — http(s):// or a local file path — and
// parses it.
func Load(specURL string) (*Document, error) {
	data, err := fetch(specURL)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

func fetch(specURL string) ([]byte, error) {
	if strings.HasPrefix(specURL, "http://") || strings.HasPrefix(specURL, "https://") {
		resp, err := http.Get(specURL)
		if err != nil {
			return nil, fmt.Errorf("fetch spec %s: %w", specURL, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("fetch spec %s: unexpected status %d", specURL, resp.StatusCode)
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read spec %s: %w", specURL, err)
		}
		return data, nil
	}
	data, err := os.ReadFile(specURL)
	if err != nil {
		return nil, fmt.Errorf("read spec file: %w", err)
	}
	return data, nil
}

// Parse decodes an OpenAPI 3 JSON document. Operations without an operationId
// are skipped with a log line; operations are returned sorted by tool name so
// registration order is deterministic.
func Parse(data []byte) (*Document, error) {
	var raw rawSpec
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse spec: %w", err)
	}

	doc := &Document{}
	for _, s := range raw.Servers {
		if s.URL != "" {
			doc.Servers = append(doc.Servers, s.URL)
		}
	}

	for path, item := range raw.Paths {
		for _, method := range httpMethods {
			rawOp, ok := item[method]
			if !ok {
				continue
			}
			var op rawOperation
			if err := json.Unmarshal(rawOp, &op); err != nil {
				return nil, fmt.Errorf("parse spec: %s %s: %w", strings.ToUpper(method), path, err)
			}
			if op.OperationID == "" {
				log.Printf("rest-connector: skipping %s %s: no operationId", strings.ToUpper(method), path)
				continue
			}
			params := make([]Parameter, 0, len(op.Parameters))
			for _, p := range op.Parameters {
				if p.In == "path" || p.In == "query" {
					params = append(params, p)
				}
			}
			doc.Operations = append(doc.Operations, Operation{
				OperationID: op.OperationID,
				ToolName:    SnakeCase(op.OperationID),
				Method:      strings.ToUpper(method),
				Path:        path,
				Summary:     op.Summary,
				Parameters:  params,
			})
		}
	}

	sort.Slice(doc.Operations, func(i, j int) bool {
		return doc.Operations[i].ToolName < doc.Operations[j].ToolName
	})
	return doc, nil
}

// SnakeCase converts an operationId to a snake_case tool name: CamelCase
// becomes camel_case (acronym runs are kept together: HTTPGet -> http_get)
// and dashes, dots and spaces become underscores.
func SnakeCase(s string) string {
	runes := []rune(s)
	var b strings.Builder
	for i, r := range runes {
		switch {
		case r == '-' || r == '.' || r == ' ':
			b.WriteRune('_')
		case unicode.IsUpper(r):
			if i > 0 {
				prev := runes[i-1]
				nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
				if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && nextLower) {
					b.WriteRune('_')
				}
			}
			b.WriteRune(unicode.ToLower(r))
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
