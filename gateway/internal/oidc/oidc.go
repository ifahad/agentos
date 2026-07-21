// Package oidc implements optional OpenID Connect single sign-on for the
// gateway (Phase 6). It is enabled only when AGENTOS_OIDC_ISSUER is set; with
// no OIDC env the gateway behaves exactly as Phase 5. The package wraps
// coreos/go-oidc for real ID-token verification (JWKS/RS256, issuer, audience,
// expiry) and golang.org/x/oauth2 for the authorization-code exchange. The
// login `state` is an HMAC over a nonce and expiry keyed by the admin key, so
// callbacks cannot be forged or replayed past their 10-minute window.
package oidc

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// stateTTL bounds how long a signed login state stays valid.
const stateTTL = 10 * time.Minute

// Config is the resolved OIDC configuration. DefaultOrg/DefaultRole seed
// SSO-provisioned users.
type Config struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	PostLoginURL string
	DefaultOrg   string
	DefaultRole  string
}

// Provider is an enabled OIDC integration. Build it with New; a nil *Provider
// represents "SSO disabled" and its Enabled method reports false.
type Provider struct {
	cfg      Config
	oauth    *oauth2.Config
	verifier *oidc.IDTokenVerifier
	adminKey []byte // HMAC secret for login state
}

// New performs OIDC discovery against cfg.Issuer and builds a Provider. It is
// called only when SSO is configured; discovery failure or a missing client
// id/secret is a fatal misconfiguration (returned as an error). adminKey is the
// HMAC secret used to sign and verify the login state.
func New(ctx context.Context, cfg Config, adminKey string) (*Provider, error) {
	if cfg.Issuer == "" {
		return nil, errors.New("oidc: issuer is required")
	}
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, errors.New("oidc: AGENTOS_OIDC_CLIENT_ID and AGENTOS_OIDC_CLIENT_SECRET are required")
	}
	if cfg.DefaultOrg == "" {
		cfg.DefaultOrg = "org_default"
	}
	if cfg.DefaultRole == "" {
		cfg.DefaultRole = "member"
	}
	provider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc: discovery for issuer %q failed: %w", cfg.Issuer, err)
	}
	return &Provider{
		cfg: cfg,
		oauth: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Endpoint:     provider.Endpoint(),
			RedirectURL:  cfg.RedirectURL,
			Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
		},
		verifier: provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
		adminKey: []byte(adminKey),
	}, nil
}

// Enabled reports whether SSO is active. A nil Provider is disabled.
func (p *Provider) Enabled() bool { return p != nil }

// Issuer returns the configured issuer URL.
func (p *Provider) Issuer() string { return p.cfg.Issuer }

// DefaultOrg is the org SSO users are provisioned into.
func (p *Provider) DefaultOrg() string { return p.cfg.DefaultOrg }

// DefaultRole is the role assigned to newly provisioned SSO users.
func (p *Provider) DefaultRole() string { return p.cfg.DefaultRole }

// PostLoginURL is the console URL the callback redirects back to.
func (p *Provider) PostLoginURL() string { return p.cfg.PostLoginURL }

// AuthCodeURL returns the provider authorization URL for the given signed
// state (response_type=code with the configured scopes and redirect URI).
func (p *Provider) AuthCodeURL(state string) string {
	return p.oauth.AuthCodeURL(state)
}

// NewState returns a signed, time-limited login state: base64url(payload) plus
// an HMAC-SHA256 tag over the payload, keyed by the admin key. The payload is a
// random nonce and an absolute expiry.
func (p *Provider) NewState(now time.Time) (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("oidc: generate state nonce: %w", err)
	}
	payload := b64.EncodeToString(nonce) + ":" + strconv.FormatInt(now.Add(stateTTL).Unix(), 10)
	return b64.EncodeToString([]byte(payload)) + "." + b64.EncodeToString(p.sign(payload)), nil
}

// VerifyState checks the HMAC tag and expiry of a login state.
func (p *Provider) VerifyState(state string, now time.Time) error {
	encPayload, encSig, ok := strings.Cut(state, ".")
	if !ok {
		return errors.New("oidc: malformed state")
	}
	payload, err := b64.DecodeString(encPayload)
	if err != nil {
		return errors.New("oidc: malformed state payload")
	}
	sig, err := b64.DecodeString(encSig)
	if err != nil {
		return errors.New("oidc: malformed state signature")
	}
	if !hmac.Equal(sig, p.sign(string(payload))) {
		return errors.New("oidc: state signature mismatch")
	}
	_, expStr, ok := strings.Cut(string(payload), ":")
	if !ok {
		return errors.New("oidc: malformed state payload fields")
	}
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return errors.New("oidc: malformed state expiry")
	}
	if now.Unix() > exp {
		return errors.New("oidc: state expired")
	}
	return nil
}

func (p *Provider) sign(payload string) []byte {
	mac := hmac.New(sha256.New, p.adminKey)
	mac.Write([]byte(payload))
	return mac.Sum(nil)
}

// ErrEmailNotVerified is returned by Exchange when the ID token's email is not
// asserted as verified by the IdP. Matching users by an unverified (and thus
// spoofable) email is the account-takeover vector closed in H3.
var ErrEmailNotVerified = errors.New("oidc: id token email is not verified")

// Identity is the verified subject of a successful SSO exchange. Sub is the
// stable OIDC subject identifier — the value SSO upserts match on, never the
// mutable email.
type Identity struct {
	Email string
	Sub   string
}

// Exchange trades an authorization code for tokens, verifies the ID token, and
// returns the caller's identity. It requires the email claim to be present and
// asserted as verified (email_verified == true); an unverified or absent email
// is rejected (H3) so a hostile IdP cannot present an arbitrary victim address.
func (p *Provider) Exchange(ctx context.Context, code string) (*Identity, error) {
	tok, err := p.oauth.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("oidc: code exchange failed: %w", err)
	}
	rawID, ok := tok.Extra("id_token").(string)
	if !ok || rawID == "" {
		return nil, errors.New("oidc: token response missing id_token")
	}
	idToken, err := p.verifier.Verify(ctx, rawID)
	if err != nil {
		return nil, fmt.Errorf("oidc: id token verification failed: %w", err)
	}
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Sub           string `json:"sub"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("oidc: parse id token claims: %w", err)
	}
	if claims.Sub == "" {
		return nil, errors.New("oidc: id token missing sub")
	}
	if claims.Email == "" {
		return nil, errors.New("oidc: id token missing email")
	}
	// Fail closed: a missing email_verified claim is treated as unverified.
	if !claims.EmailVerified {
		return nil, ErrEmailNotVerified
	}
	return &Identity{Email: claims.Email, Sub: claims.Sub}, nil
}

var b64 = base64.RawURLEncoding
