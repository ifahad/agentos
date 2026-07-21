package spec

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const sampleSpec = `{
  "openapi": "3.0.3",
  "info": {"title": "Sample", "version": "1.0.0"},
  "servers": [{"url": "http://sample:8095"}, {"url": "https://backup.example"}],
  "paths": {
    "/customers": {
      "get": {
        "operationId": "listCustomers",
        "summary": "List customers",
        "parameters": [
          {"name": "city", "in": "query", "required": false, "schema": {"type": "string"}}
        ]
      },
      "post": {
        "operationId": "createCustomer",
        "summary": "Create a customer"
      }
    },
    "/customers/{id}": {
      "get": {
        "operationId": "getCustomer",
        "summary": "Get one customer",
        "parameters": [
          {"name": "id", "in": "path", "required": true, "schema": {"type": "string"}},
          {"name": "verbose", "in": "header", "required": false, "schema": {"type": "string"}}
        ]
      },
      "delete": {
        "summary": "No operationId here, must be skipped"
      }
    }
  }
}`

func TestParse(t *testing.T) {
	doc, err := Parse([]byte(sampleSpec))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	wantServers := []string{"http://sample:8095", "https://backup.example"}
	if !reflect.DeepEqual(doc.Servers, wantServers) {
		t.Errorf("Servers = %v, want %v", doc.Servers, wantServers)
	}

	// The delete on /customers/{id} has no operationId and must be skipped.
	want := []Operation{
		{
			OperationID: "createCustomer",
			ToolName:    "create_customer",
			Method:      "POST",
			Path:        "/customers",
			Summary:     "Create a customer",
			Parameters:  []Parameter{},
		},
		{
			OperationID: "getCustomer",
			ToolName:    "get_customer",
			Method:      "GET",
			Path:        "/customers/{id}",
			Summary:     "Get one customer",
			// The header parameter must be dropped: only path+query survive.
			Parameters: []Parameter{{Name: "id", In: "path", Required: true}},
		},
		{
			OperationID: "listCustomers",
			ToolName:    "list_customers",
			Method:      "GET",
			Path:        "/customers",
			Summary:     "List customers",
			Parameters:  []Parameter{{Name: "city", In: "query", Required: false}},
		},
	}
	if !reflect.DeepEqual(doc.Operations, want) {
		t.Errorf("Operations mismatch:\n got %+v\nwant %+v", doc.Operations, want)
	}
}

func TestParseInvalidJSON(t *testing.T) {
	if _, err := Parse([]byte("{not json")); err == nil {
		t.Fatal("Parse of invalid JSON succeeded, want error")
	}
}

func TestSnakeCase(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"listCustomers", "list_customers"},
		{"getCustomer", "get_customer"},
		{"listTickets", "list_tickets"},
		{"already_snake", "already_snake"},
		{"list-open-tickets", "list_open_tickets"},
		{"HTTPGetItem", "http_get_item"},
		{"getHTTPStatus", "get_http_status"},
		{"v2ListItems", "v2_list_items"},
		{"Simple", "simple"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := SnakeCase(tt.in); got != tt.want {
			t.Errorf("SnakeCase(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestLoadFromHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleSpec))
	}))
	defer srv.Close()

	doc, err := Load(srv.URL)
	if err != nil {
		t.Fatalf("Load(%s): %v", srv.URL, err)
	}
	if len(doc.Operations) != 3 {
		t.Errorf("got %d operations, want 3", len(doc.Operations))
	}
}

func TestLoadFromHTTPErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer srv.Close()

	if _, err := Load(srv.URL); err == nil {
		t.Fatal("Load from 500 endpoint succeeded, want error")
	}
}

func TestLoadFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spec.json")
	if err := os.WriteFile(path, []byte(sampleSpec), 0o600); err != nil {
		t.Fatalf("write temp spec: %v", err)
	}

	doc, err := Load(path)
	if err != nil {
		t.Fatalf("Load(%s): %v", path, err)
	}
	if len(doc.Operations) != 3 {
		t.Errorf("got %d operations, want 3", len(doc.Operations))
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "absent.json")); err == nil {
		t.Fatal("Load of missing file succeeded, want error")
	}
}
