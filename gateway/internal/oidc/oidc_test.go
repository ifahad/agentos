package oidc

import (
	"context"
	"errors"
	"testing"
	"time"
)

const testAdminKey = "admin-secret"

func newTestProvider(t *testing.T, m *mockOIDC) *Provider {
	t.Helper()
	p, err := New(context.Background(), Config{
		Issuer:       m.server.URL,
		ClientID:     m.clientID,
		ClientSecret: "client-secret",
		RedirectURL:  "http://gateway.test/auth/oidc/callback",
		PostLoginURL: "http://console.test/",
		DefaultOrg:   "org_default",
		DefaultRole:  "member",
	}, testAdminKey)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p
}

func TestNewRequiresClientCredentials(t *testing.T) {
	m := newMockOIDC(t, "client-abc", "user@corp.test")
	_, err := New(context.Background(), Config{Issuer: m.server.URL, ClientID: "", ClientSecret: "s"}, testAdminKey)
	if err == nil {
		t.Error("New with empty client id: want error")
	}
	_, err = New(context.Background(), Config{Issuer: m.server.URL, ClientID: "c", ClientSecret: ""}, testAdminKey)
	if err == nil {
		t.Error("New with empty client secret: want error")
	}
}

func TestNewDiscoveryFailureIsFatal(t *testing.T) {
	_, err := New(context.Background(), Config{
		Issuer: "http://127.0.0.1:0", ClientID: "c", ClientSecret: "s",
	}, testAdminKey)
	if err == nil {
		t.Error("New with unreachable issuer: want error")
	}
}

func TestStateRoundTrip(t *testing.T) {
	m := newMockOIDC(t, "client-abc", "user@corp.test")
	p := newTestProvider(t, m)
	now := time.Unix(1000, 0)

	state, err := p.NewState(now)
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}
	if err := p.VerifyState(state, now); err != nil {
		t.Errorf("VerifyState valid: %v", err)
	}
	// Still valid just before expiry (10 min).
	if err := p.VerifyState(state, now.Add(9*time.Minute)); err != nil {
		t.Errorf("VerifyState before expiry: %v", err)
	}
	// Expired.
	if err := p.VerifyState(state, now.Add(11*time.Minute)); err == nil {
		t.Error("VerifyState after expiry: want error")
	}
}

func TestStateTamperRejected(t *testing.T) {
	m := newMockOIDC(t, "client-abc", "user@corp.test")
	p := newTestProvider(t, m)
	now := time.Unix(1000, 0)
	state, _ := p.NewState(now)

	// Garbage and structurally-invalid states.
	for _, bad := range []string{"", "nodot", "a.b.c.d", state + "x", "x." + state} {
		if err := p.VerifyState(bad, now); err == nil {
			t.Errorf("VerifyState(%q): want error", bad)
		}
	}

	// A state signed with a different admin key must not verify.
	other, err := New(context.Background(), Config{
		Issuer: m.server.URL, ClientID: m.clientID, ClientSecret: "s",
	}, "different-admin-key")
	if err != nil {
		t.Fatal(err)
	}
	otherState, _ := other.NewState(now)
	if err := p.VerifyState(otherState, now); err == nil {
		t.Error("VerifyState with foreign-key state: want signature mismatch")
	}
}

func TestExchangeVerifiesIDToken(t *testing.T) {
	m := newMockOIDC(t, "client-abc", "alice@corp.test")
	p := newTestProvider(t, m)

	id, err := p.Exchange(context.Background(), "any-code")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if id.Email != "alice@corp.test" {
		t.Errorf("email = %q, want alice@corp.test", id.Email)
	}
}

func TestExchangeRejectsWrongAudience(t *testing.T) {
	// The mock issues a token for "client-abc"; a provider expecting a different
	// client id must reject it (audience check).
	m := newMockOIDC(t, "client-abc", "alice@corp.test")
	p, err := New(context.Background(), Config{
		Issuer: m.server.URL, ClientID: "client-XYZ", ClientSecret: "s",
	}, testAdminKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exchange(context.Background(), "any-code"); err == nil {
		t.Error("Exchange with wrong audience: want verification error")
	}
}

func TestEnabledNilProvider(t *testing.T) {
	var p *Provider
	if p.Enabled() {
		t.Error("nil provider Enabled() = true, want false")
	}
}

func TestFromEnvDisabledWithoutIssuer(t *testing.T) {
	t.Setenv("AGENTOS_OIDC_ISSUER", "")
	p, enabled, err := FromEnv(context.Background(), testAdminKey)
	if err != nil || enabled || p != nil {
		t.Errorf("FromEnv without issuer = (%v, %v, %v), want (nil, false, nil)", p, enabled, err)
	}
}

func TestExchangeRequiresVerifiedEmail(t *testing.T) {
	m := newMockOIDC(t, "client-abc", "alice@corp.test")
	m.emailVerified = false // IdP asserts the email is NOT verified
	p := newTestProvider(t, m)

	if _, err := p.Exchange(context.Background(), "any-code"); !errors.Is(err, ErrEmailNotVerified) {
		t.Errorf("Exchange with email_verified=false err = %v, want ErrEmailNotVerified", err)
	}
}

func TestExchangeReturnsSub(t *testing.T) {
	m := newMockOIDC(t, "client-abc", "alice@corp.test")
	m.sub = "stable-subject-42"
	p := newTestProvider(t, m)

	id, err := p.Exchange(context.Background(), "any-code")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if id.Sub != "stable-subject-42" {
		t.Errorf("sub = %q, want stable-subject-42", id.Sub)
	}
	if id.Email != "alice@corp.test" {
		t.Errorf("email = %q", id.Email)
	}
}
