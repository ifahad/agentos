package oidc

import (
	"context"
	"os"
)

// FromEnv builds a Provider from the AGENTOS_OIDC_* environment. SSO is enabled
// only when AGENTOS_OIDC_ISSUER is set; when it is empty the returned provider
// is nil and enabled is false, reproducing Phase 5 behavior exactly. A set
// issuer with failed discovery or a missing client id/secret returns an error
// so the caller can fail fast at startup.
func FromEnv(ctx context.Context, adminKey string) (provider *Provider, enabled bool, err error) {
	issuer := os.Getenv("AGENTOS_OIDC_ISSUER")
	if issuer == "" {
		return nil, false, nil
	}
	cfg := Config{
		Issuer:       issuer,
		ClientID:     os.Getenv("AGENTOS_OIDC_CLIENT_ID"),
		ClientSecret: os.Getenv("AGENTOS_OIDC_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("AGENTOS_OIDC_REDIRECT_URL"),
		PostLoginURL: os.Getenv("AGENTOS_OIDC_POST_LOGIN_URL"),
		DefaultOrg:   os.Getenv("AGENTOS_OIDC_DEFAULT_ORG"),
		DefaultRole:  os.Getenv("AGENTOS_OIDC_DEFAULT_ROLE"),
	}
	p, err := New(ctx, cfg, adminKey)
	if err != nil {
		return nil, true, err
	}
	return p, true, nil
}
