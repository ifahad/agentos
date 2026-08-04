package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The container probe is the only thing standing between a wedged gateway and
// a compose stack that still reports it as up, so its three outcomes are worth
// pinning: serving, answering wrongly, and not answering at all.
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
		// A gateway that has lost its store answers, but not with 200. Reading
		// "it responded" as healthy is exactly the failure this guards.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer srv.Close()
		if got := runHealthCheck(srv.URL + "/healthz"); got == 0 {
			t.Error("runHealthCheck = 0 for a 503, want non-zero")
		}
	})

	t.Run("unreachable is unhealthy", func(t *testing.T) {
		// Bind and immediately close, so the port is one nothing is listening on.
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		url := srv.URL + "/healthz"
		srv.Close()
		if got := runHealthCheck(url); got == 0 {
			t.Error("runHealthCheck = 0 against a closed port, want non-zero")
		}
	})
}
