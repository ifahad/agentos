package oidc

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockOIDC is an httptest-backed OpenID provider that serves discovery, a JWKS,
// and a token endpoint returning an RS256-signed ID token. It exercises the
// real go-oidc verification path (no bypass).
type mockOIDC struct {
	server        *httptest.Server
	key           *rsa.PrivateKey
	clientID      string
	email         string // the email claim the token endpoint issues
	emailVerified bool   // the email_verified claim (default true)
	sub           string // the subject claim (default "subject-123")
	kid           string
}

func newMockOIDC(t *testing.T, clientID, email string) *mockOIDC {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	m := &mockOIDC{key: key, clientID: clientID, email: email, emailVerified: true, sub: "subject-123", kid: "test-key-1"}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"issuer":                 m.server.URL,
			"authorization_endpoint": m.server.URL + "/authorize",
			"token_endpoint":         m.server.URL + "/token",
			"jwks_uri":               m.server.URL + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, m.jwks())
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		idToken := m.signIDToken(t, time.Now().Add(time.Hour))
		writeJSON(w, map[string]any{
			"access_token": "mock-access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
			"id_token":     idToken,
		})
	})
	m.server = httptest.NewServer(mux)
	t.Cleanup(m.server.Close)
	return m
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func b64url(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// jwks returns the public key as a JWKS document.
func (m *mockOIDC) jwks() map[string]any {
	pub := m.key.Public().(*rsa.PublicKey)
	eb := big.NewInt(int64(pub.E)).Bytes()
	return map[string]any{
		"keys": []map[string]any{{
			"kty": "RSA",
			"kid": m.kid,
			"alg": "RS256",
			"use": "sig",
			"n":   b64url(pub.N.Bytes()),
			"e":   b64url(eb),
		}},
	}
}

// signIDToken builds and RS256-signs an ID token with the mock's claims.
func (m *mockOIDC) signIDToken(t *testing.T, expiry time.Time) string {
	t.Helper()
	header := map[string]any{"alg": "RS256", "typ": "JWT", "kid": m.kid}
	claims := map[string]any{
		"iss":            m.server.URL,
		"aud":            m.clientID,
		"sub":            m.sub,
		"email":          m.email,
		"email_verified": m.emailVerified,
		"exp":            expiry.Unix(),
		"iat":            time.Now().Unix(),
	}
	hb, _ := json.Marshal(header)
	cb, _ := json.Marshal(claims)
	signingInput := b64url(hb) + "." + b64url(cb)
	sum := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, m.key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatalf("sign id token: %v", err)
	}
	return signingInput + "." + b64url(sig)
}
