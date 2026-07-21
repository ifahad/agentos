package server

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ifahad/agentos/gateway/internal/oidc"
	"github.com/ifahad/agentos/gateway/internal/provider"
	"github.com/ifahad/agentos/gateway/internal/store"
)

// --- mock OIDC provider (RS256, real go-oidc verification path) ---

type mockOIDCProvider struct {
	server        *httptest.Server
	key           *rsa.PrivateKey
	clientID      string
	email         string
	emailVerified bool   // default true
	sub           string // default "sub-1"
	kid           string
}

func newMockOIDCProvider(t *testing.T, clientID, email string) *mockOIDCProvider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa key: %v", err)
	}
	m := &mockOIDCProvider{key: key, clientID: clientID, email: email, emailVerified: true, sub: "sub-1", kid: "k1"}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		mockWriteJSON(w, map[string]any{
			"issuer":                 m.server.URL,
			"authorization_endpoint": m.server.URL + "/authorize",
			"token_endpoint":         m.server.URL + "/token",
			"jwks_uri":               m.server.URL + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		pub := m.key.Public().(*rsa.PublicKey)
		mockWriteJSON(w, map[string]any{"keys": []map[string]any{{
			"kty": "RSA", "kid": m.kid, "alg": "RS256", "use": "sig",
			"n": mockB64(pub.N.Bytes()), "e": mockB64(big.NewInt(int64(pub.E)).Bytes()),
		}}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		mockWriteJSON(w, map[string]any{
			"access_token": "at", "token_type": "Bearer", "expires_in": 3600,
			"id_token": m.signIDToken(t),
		})
	})
	m.server = httptest.NewServer(mux)
	t.Cleanup(m.server.Close)
	return m
}

func mockWriteJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func mockB64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func (m *mockOIDCProvider) signIDToken(t *testing.T) string {
	t.Helper()
	header, _ := json.Marshal(map[string]any{"alg": "RS256", "typ": "JWT", "kid": m.kid})
	claims, _ := json.Marshal(map[string]any{
		"iss": m.server.URL, "aud": m.clientID, "sub": m.sub,
		"email": m.email, "email_verified": m.emailVerified,
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
	})
	input := mockB64(header) + "." + mockB64(claims)
	sum := sha256.Sum256([]byte(input))
	sig, err := rsa.SignPKCS1v15(rand.Reader, m.key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return input + "." + mockB64(sig)
}

// newOIDCGateway wires a gateway with SSO enabled against the mock provider.
func newOIDCGateway(t *testing.T, m *mockOIDCProvider) (*store.Memory, string) {
	t.Helper()
	prov, err := oidc.New(context.Background(), oidc.Config{
		Issuer:       m.server.URL,
		ClientID:     m.clientID,
		ClientSecret: "client-secret",
		RedirectURL:  "http://gateway.test/auth/oidc/callback",
		PostLoginURL: "http://console.test/",
		DefaultOrg:   store.DefaultOrgID,
		DefaultRole:  "member",
	}, testAdminKey)
	if err != nil {
		t.Fatalf("oidc.New: %v", err)
	}
	mem := store.NewMemory()
	if err := mem.EnsureOrg(context.Background(), store.DefaultOrgID, "default", 0); err != nil {
		t.Fatal(err)
	}
	router := &provider.Router{OpenAIBaseURL: "http://unused", AnthropicBaseURL: "http://unused"}
	ts := newHTTPServer(t, New(mem, router, testAdminKey, WithOIDC(prov)).Handler())
	return mem, ts
}

// noRedirectClient returns a client that surfaces 3xx responses instead of
// following them, so the test can inspect Location.
func noRedirectClient() *http.Client {
	return &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
}

func TestOIDCStatusEnabled(t *testing.T) {
	m := newMockOIDCProvider(t, "client-abc", "user@corp.test")
	_, ts := newOIDCGateway(t, m)

	resp, body := doJSON(t, http.MethodGet, ts+"/auth/oidc/status", "", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if body["enabled"] != true {
		t.Errorf("enabled = %v, want true", body["enabled"])
	}
	if body["issuer"] != m.server.URL {
		t.Errorf("issuer = %v, want %q", body["issuer"], m.server.URL)
	}
}

func TestOIDCStatusDisabled(t *testing.T) {
	_, _, srv := newTestGateway(t) // no WithOIDC
	resp, body := doJSON(t, http.MethodGet, srv.URL+"/auth/oidc/status", "", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if body["enabled"] != false {
		t.Errorf("enabled = %v, want false", body["enabled"])
	}
	if _, ok := body["issuer"]; ok {
		t.Error("disabled status should omit issuer")
	}
}

func TestOIDCLoginDisabled404(t *testing.T) {
	_, _, srv := newTestGateway(t)
	for _, path := range []string{"/auth/oidc/login", "/auth/oidc/callback"} {
		resp, body := doJSON(t, http.MethodGet, srv.URL+path, "", "")
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s status = %d, want 404", path, resp.StatusCode)
		}
		if got := errorType(t, body); got != errSSODisabled {
			t.Errorf("%s error type = %q, want sso_disabled", path, got)
		}
	}
}

func TestOIDCFullLoginFlow(t *testing.T) {
	m := newMockOIDCProvider(t, "client-abc", "newuser@corp.test")
	mem, ts := newOIDCGateway(t, m)
	client := noRedirectClient()

	// 1. Login → 302 to the provider authorize endpoint carrying a signed state.
	loginResp, err := client.Get(ts + "/auth/oidc/login")
	if err != nil {
		t.Fatalf("login GET: %v", err)
	}
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusFound {
		t.Fatalf("login status = %d, want 302", loginResp.StatusCode)
	}
	authURL, err := url.Parse(loginResp.Header.Get("Location"))
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	if !strings.HasPrefix(loginResp.Header.Get("Location"), m.server.URL+"/authorize") {
		t.Errorf("login redirect = %q, want provider authorize URL", loginResp.Header.Get("Location"))
	}
	if authURL.Query().Get("response_type") != "code" {
		t.Errorf("response_type = %q, want code", authURL.Query().Get("response_type"))
	}
	if authURL.Query().Get("client_id") != "client-abc" {
		t.Errorf("client_id = %q", authURL.Query().Get("client_id"))
	}
	if got := authURL.Query().Get("scope"); !strings.Contains(got, "openid") || !strings.Contains(got, "email") {
		t.Errorf("scope = %q, want openid+email", got)
	}
	state := authURL.Query().Get("state")
	if state == "" {
		t.Fatal("login redirect missing state")
	}

	// 2. Callback with the code + valid state → 302 to the console with a token
	//    in the URL fragment.
	cbResp, err := client.Get(ts + "/auth/oidc/callback?code=the-code&state=" + url.QueryEscape(state))
	if err != nil {
		t.Fatalf("callback GET: %v", err)
	}
	cbResp.Body.Close()
	if cbResp.StatusCode != http.StatusFound {
		t.Fatalf("callback status = %d, want 302", cbResp.StatusCode)
	}
	loc := cbResp.Header.Get("Location")
	if !strings.HasPrefix(loc, "http://console.test/#") {
		t.Fatalf("callback redirect = %q, want console URL with fragment", loc)
	}
	frag := loc[strings.Index(loc, "#")+1:]
	vals, err := url.ParseQuery(frag)
	if err != nil {
		t.Fatalf("parse fragment: %v", err)
	}
	token := vals.Get("token")
	if !strings.HasPrefix(token, "agu-") {
		t.Errorf("fragment token = %q, want agu- token", token)
	}
	if vals.Get("email") != "newuser@corp.test" {
		t.Errorf("fragment email = %q", vals.Get("email"))
	}
	if vals.Get("role") != "member" {
		t.Errorf("fragment role = %q, want member", vals.Get("role"))
	}

	// 3. The minted token authenticates as the upserted user.
	u, err := mem.AuthenticateUser(context.Background(), token)
	if err != nil {
		t.Fatalf("AuthenticateUser(minted): %v", err)
	}
	if u.Email != "newuser@corp.test" || u.OrgID != store.DefaultOrgID || u.Role != "member" {
		t.Errorf("upserted user = %+v", u)
	}
}

func TestOIDCReturningUserReusesIdentity(t *testing.T) {
	m := newMockOIDCProvider(t, "client-abc", "existing@corp.test")
	mem, ts := newOIDCGateway(t, m)

	// Pre-create the user as an owner; SSO must reuse it and keep the role.
	existing, _, err := mem.CreateUser(context.Background(), store.DefaultOrgID, "existing@corp.test", "owner")
	if err != nil {
		t.Fatal(err)
	}
	client := noRedirectClient()

	loginResp, _ := client.Get(ts + "/auth/oidc/login")
	loginResp.Body.Close()
	authURL, _ := url.Parse(loginResp.Header.Get("Location"))
	state := authURL.Query().Get("state")

	cbResp, _ := client.Get(ts + "/auth/oidc/callback?code=c&state=" + url.QueryEscape(state))
	cbResp.Body.Close()
	loc := cbResp.Header.Get("Location")
	frag := loc[strings.Index(loc, "#")+1:]
	vals, _ := url.ParseQuery(frag)

	if vals.Get("role") != "owner" {
		t.Errorf("returning user role = %q, want owner (kept)", vals.Get("role"))
	}
	// Only one user should exist for this email (no duplicate provisioning).
	users, _ := mem.Users(context.Background(), store.DefaultOrgID)
	var count int
	for _, u := range users {
		if u.Email == "existing@corp.test" {
			count++
			if u.ID != existing.ID {
				t.Errorf("upsert created a new user id %q, want reuse of %q", u.ID, existing.ID)
			}
		}
	}
	if count != 1 {
		t.Errorf("users with email = %d, want 1 (no duplicate)", count)
	}

	// The freshly minted token works.
	if _, err := mem.AuthenticateUser(context.Background(), vals.Get("token")); err != nil {
		t.Errorf("minted token invalid: %v", err)
	}
}

func TestOIDCCallbackInvalidState401(t *testing.T) {
	m := newMockOIDCProvider(t, "client-abc", "user@corp.test")
	_, ts := newOIDCGateway(t, m)

	resp, body := doJSON(t, http.MethodGet, ts+"/auth/oidc/callback?code=c&state=forged", "", "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	if got := errorType(t, body); got != errSSOFailed {
		t.Errorf("error type = %q, want sso_failed", got)
	}
}

// ssoCallback drives login+callback against the mock and returns the callback
// response plus the parsed fragment values (token/email/role), if any.
func ssoCallback(t *testing.T, ts string) (*http.Response, url.Values) {
	t.Helper()
	client := noRedirectClient()
	loginResp, err := client.Get(ts + "/auth/oidc/login")
	if err != nil {
		t.Fatalf("login GET: %v", err)
	}
	loginResp.Body.Close()
	authURL, _ := url.Parse(loginResp.Header.Get("Location"))
	state := authURL.Query().Get("state")

	cbResp, err := client.Get(ts + "/auth/oidc/callback?code=c&state=" + url.QueryEscape(state))
	if err != nil {
		t.Fatalf("callback GET: %v", err)
	}
	defer cbResp.Body.Close()
	loc := cbResp.Header.Get("Location")
	if i := strings.Index(loc, "#"); i >= 0 {
		vals, _ := url.ParseQuery(loc[i+1:])
		return cbResp, vals
	}
	return cbResp, nil
}

// TestOIDCRejectsUnverifiedEmail proves H3: an ID token whose email is not
// verified is rejected (401 sso_failed) and provisions no user.
func TestOIDCRejectsUnverifiedEmail(t *testing.T) {
	m := newMockOIDCProvider(t, "client-abc", "attacker@corp.test")
	m.emailVerified = false
	mem, ts := newOIDCGateway(t, m)

	resp, _ := ssoCallback(t, ts)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 for unverified email", resp.StatusCode)
	}
	users, _ := mem.Users(context.Background(), store.DefaultOrgID)
	if len(users) != 0 {
		t.Errorf("provisioned %d users for unverified email, want 0", len(users))
	}
}

// TestOIDCEmailMatchDifferentSubNoTakeover proves H3: a verified email that
// matches an existing user BOUND TO A DIFFERENT SUBJECT must not adopt that
// user's role — the login is refused, closing the account-takeover vector.
func TestOIDCEmailMatchDifferentSubNoTakeover(t *testing.T) {
	m := newMockOIDCProvider(t, "client-abc", "owner@corp.test")
	m.sub = "attacker-subject"
	mem, ts := newOIDCGateway(t, m)
	ctx := context.Background()

	// A privileged account already bound to a different IdP subject.
	victim, _, err := mem.CreateUserWithExternalID(ctx, store.DefaultOrgID, "owner@corp.test", "owner", "victim-subject")
	if err != nil {
		t.Fatal(err)
	}

	resp, vals := ssoCallback(t, ts)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (no takeover)", resp.StatusCode)
	}
	if vals.Get("role") == "owner" {
		t.Errorf("attacker adopted owner role via email match")
	}
	// The victim's record is untouched (role + bound subject preserved).
	got, err := mem.UserByExternalID(ctx, store.DefaultOrgID, "victim-subject")
	if err != nil || got.ID != victim.ID || got.Role != "owner" {
		t.Errorf("victim mutated: %+v (err %v)", got, err)
	}
}

// TestOIDCSameSubReturningKeepsRole proves the same-subject returning user is
// matched on sub and keeps their role, even if the email is unchanged.
func TestOIDCSameSubReturningKeepsRole(t *testing.T) {
	m := newMockOIDCProvider(t, "client-abc", "admin@corp.test")
	m.sub = "stable-sub-1"
	mem, ts := newOIDCGateway(t, m)
	ctx := context.Background()

	existing, _, err := mem.CreateUserWithExternalID(ctx, store.DefaultOrgID, "admin@corp.test", "admin", "stable-sub-1")
	if err != nil {
		t.Fatal(err)
	}

	resp, vals := ssoCallback(t, ts)
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want 302", resp.StatusCode)
	}
	if vals.Get("role") != "admin" {
		t.Errorf("role = %q, want admin (kept via sub match)", vals.Get("role"))
	}
	u, err := mem.AuthenticateUser(ctx, vals.Get("token"))
	if err != nil || u.ID != existing.ID {
		t.Errorf("token resolved to %+v (err %v), want reuse of %q", u, err, existing.ID)
	}
	users, _ := mem.Users(ctx, store.DefaultOrgID)
	if len(users) != 1 {
		t.Errorf("users = %d, want 1 (no duplicate)", len(users))
	}
}
