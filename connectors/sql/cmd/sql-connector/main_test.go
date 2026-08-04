package main

import (
	"net"
	"testing"
)

// The probe is what compose gates `depends_on: service_healthy` on, so both
// outcomes are pinned: a listener that accepts, and a port nothing is on.
func TestRunHealthCheck(t *testing.T) {
	t.Run("listening is healthy", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		defer ln.Close()
		if got := runHealthCheck(ln.Addr().String()); got != 0 {
			t.Errorf("runHealthCheck = %d, want 0", got)
		}
	})

	t.Run("closed port is unhealthy", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		addr := ln.Addr().String()
		ln.Close()
		if got := runHealthCheck(addr); got == 0 {
			t.Error("runHealthCheck = 0 against a closed port, want non-zero")
		}
	})
}
