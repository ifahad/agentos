package main

import "testing"

func TestParseSCIMGroupRoles(t *testing.T) {
	t.Run("unset means groups grant nothing", func(t *testing.T) {
		for _, raw := range []string{"", "   "} {
			m, err := parseSCIMGroupRoles(raw)
			if err != nil || len(m) != 0 {
				t.Errorf("parseSCIMGroupRoles(%q) = %v, %v; want empty, nil", raw, m, err)
			}
		}
	})

	t.Run("pairs are parsed and lowercased", func(t *testing.T) {
		m, err := parseSCIMGroupRoles(" AgentOS Admins = admin , Contractors=viewer ")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if m["agentos admins"] != "admin" || m["contractors"] != "viewer" {
			t.Errorf("mapping = %v", m)
		}
	})

	t.Run("an unknown role is fatal, never skipped", func(t *testing.T) {
		// A silently dropped mapping is the worst outcome: the operator believes
		// the group grants admin, the gateway starts clean, and nobody is
		// granted anything until somebody notices.
		if _, err := parseSCIMGroupRoles("Admins=superuser"); err == nil {
			t.Error("an unknown role was accepted")
		}
	})

	t.Run("malformed and duplicate entries are rejected", func(t *testing.T) {
		for _, raw := range []string{"Admins", "=admin", "Admins=", "A=admin,a=viewer"} {
			if _, err := parseSCIMGroupRoles(raw); err == nil {
				t.Errorf("parseSCIMGroupRoles(%q) was accepted", raw)
			}
		}
	})
}
