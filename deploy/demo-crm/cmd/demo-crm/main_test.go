package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRunHealthCheck(t *testing.T) {
	t.Run("200 is healthy", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()
		if got := runHealthCheck(srv.URL + "/healthz"); got != 0 {
			t.Errorf("runHealthCheck = %d, want 0", got)
		}
	})

	t.Run("non-200 is unhealthy", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer srv.Close()
		if got := runHealthCheck(srv.URL + "/healthz"); got == 0 {
			t.Error("runHealthCheck = 0 for a 503, want non-zero")
		}
	})

	t.Run("unreachable is unhealthy", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		url := srv.URL + "/healthz"
		srv.Close()
		if got := runHealthCheck(url); got == 0 {
			t.Error("runHealthCheck = 0 against a closed port, want non-zero")
		}
	})
}
