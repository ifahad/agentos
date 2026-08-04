package crm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func doGet(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	NewHandler().ServeHTTP(rec, req)
	return rec
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
}

func TestHealthz(t *testing.T) {
	rec := doGet(t, "/healthz")
	if rec.Code != http.StatusOK {
		t.Errorf("GET /healthz = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); body != "ok" {
		t.Errorf("GET /healthz body = %q, want %q", body, "ok")
	}
}

func TestListCustomers(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantCount int
		wantNames []string // must all be present
	}{
		{"all customers", "/customers", 8, []string{"Al-Faisal Trading Co.", "Red Sea Logistics", "Tabuk Construction Partners"}},
		{"filter by city", "/customers?city=Jeddah", 2, []string{"Red Sea Logistics", "Hejaz Retail Group"}},
		{"filter is case-insensitive", "/customers?city=jEDDAH", 2, []string{"Red Sea Logistics"}},
		{"unknown city empty list", "/customers?city=Atlantis", 0, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := doGet(t, tt.path)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			var got []Customer
			decodeJSON(t, rec, &got)
			if len(got) != tt.wantCount {
				t.Fatalf("got %d customers, want %d", len(got), tt.wantCount)
			}
			names := make(map[string]bool, len(got))
			for _, c := range got {
				names[c.Name] = true
			}
			for _, want := range tt.wantNames {
				if !names[want] {
					t.Errorf("customer %q missing from response", want)
				}
			}
		})
	}
}

func TestGetCustomer(t *testing.T) {
	rec := doGet(t, "/customers/3")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var c Customer
	decodeJSON(t, rec, &c)
	if c.Name != "Red Sea Logistics" || c.City != "Jeddah" {
		t.Errorf("customer 3 = %+v, want Red Sea Logistics in Jeddah", c)
	}
}

func TestGetCustomerNotFound(t *testing.T) {
	for _, path := range []string{"/customers/99", "/customers/notanumber"} {
		rec := doGet(t, path)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want 404", path, rec.Code)
			continue
		}
		var body map[string]string
		decodeJSON(t, rec, &body)
		if body["error"] == "" {
			t.Errorf("GET %s: missing error field in %q", path, rec.Body.String())
		}
	}
}

func TestListTickets(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantCount int
		wantOnly  string // every ticket must have this status; "" to skip
	}{
		{"all tickets", "/tickets", 10, ""},
		{"open tickets", "/tickets?status=open", 5, "open"},
		{"closed tickets", "/tickets?status=closed", 5, "closed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := doGet(t, tt.path)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			var got []Ticket
			decodeJSON(t, rec, &got)
			if len(got) != tt.wantCount {
				t.Fatalf("got %d tickets, want %d", len(got), tt.wantCount)
			}
			for _, tk := range got {
				if tt.wantOnly != "" && tk.Status != tt.wantOnly {
					t.Errorf("ticket %d status = %q, want %q", tk.ID, tk.Status, tt.wantOnly)
				}
				if tk.Subject == "" || tk.OpenedAt == "" {
					t.Errorf("ticket %d missing subject/opened_at: %+v", tk.ID, tk)
				}
			}
		})
	}
}

// TestRedSeaLogisticsOpenTickets pins the invariant the Phase 3 smoke test
// asserts: Red Sea Logistics (customer 3) has exactly two open tickets.
func TestRedSeaLogisticsOpenTickets(t *testing.T) {
	rec := doGet(t, "/tickets?status=open")
	var got []Ticket
	decodeJSON(t, rec, &got)
	count := 0
	for _, tk := range got {
		if tk.CustomerID == 3 {
			count++
		}
	}
	if count != 2 {
		t.Errorf("Red Sea Logistics has %d open tickets, want exactly 2", count)
	}
}

func TestListTicketsInvalidStatus(t *testing.T) {
	rec := doGet(t, "/tickets?status=pending")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var body map[string]string
	decodeJSON(t, rec, &body)
	if body["error"] == "" {
		t.Errorf("missing error field in %q", rec.Body.String())
	}
}

func TestOpenAPISpec(t *testing.T) {
	rec := doGet(t, "/openapi.json")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var doc map[string]any
	decodeJSON(t, rec, &doc)

	if v, _ := doc["openapi"].(string); !strings.HasPrefix(v, "3.") {
		t.Errorf("openapi version = %q, want 3.x", v)
	}

	raw := rec.Body.String()
	for _, opID := range []string{"listCustomers", "getCustomer", "listTickets"} {
		if !strings.Contains(raw, `"operationId": "`+opID+`"`) {
			t.Errorf("spec is missing operationId %q", opID)
		}
	}
	servers, _ := doc["servers"].([]any)
	if len(servers) == 0 {
		t.Fatal("spec declares no servers")
	}
	first, _ := servers[0].(map[string]any)
	if first["url"] != "http://demo-crm:8095" {
		t.Errorf("first server url = %v, want http://demo-crm:8095", first["url"])
	}
}
