// Package crm implements the demo "legacy CRM" REST API the agentos-rest
// connector points at in the Phase 3 demo: static in-memory customers and
// tickets behind a hand-written OpenAPI 3 spec. Stdlib only.
package crm

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// NewHandler returns the demo CRM HTTP handler.
func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /openapi.json", handleOpenAPI)
	mux.HandleFunc("GET /customers", handleListCustomers)
	mux.HandleFunc("GET /customers/{id}", handleGetCustomer)
	mux.HandleFunc("GET /tickets", handleListTickets)
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("demo-crm: encode response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func handleOpenAPI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(openAPISpec)); err != nil {
		log.Printf("demo-crm: write spec: %v", err)
	}
}

func handleListCustomers(w http.ResponseWriter, r *http.Request) {
	city := r.URL.Query().Get("city")
	out := make([]Customer, 0, len(customers))
	for _, c := range customers {
		if city == "" || strings.EqualFold(c.City, city) {
			out = append(out, c)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func handleGetCustomer(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err == nil {
		for _, c := range customers {
			if c.ID == id {
				writeJSON(w, http.StatusOK, c)
				return
			}
		}
	}
	writeError(w, http.StatusNotFound, "customer not found")
}

func handleListTickets(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status != "" && status != "open" && status != "closed" {
		writeError(w, http.StatusBadRequest, `status must be "open" or "closed"`)
		return
	}
	out := make([]Ticket, 0, len(tickets))
	for _, t := range tickets {
		if status == "" || t.Status == status {
			out = append(out, t)
		}
	}
	writeJSON(w, http.StatusOK, out)
}
