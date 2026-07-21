// Package store persists virtual keys, usage accounting, and the audit log.
package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/ifahad/agentos/gateway/internal/rbac"
)

// ErrInvalidKey is returned by Authenticate when the secret is unknown.
var ErrInvalidKey = errors.New("invalid key")

// ErrInvalidToken is returned by AuthenticateUser when a user token is unknown.
var ErrInvalidToken = errors.New("invalid user token")

// ErrOrgNotFound is returned when an org id does not exist.
var ErrOrgNotFound = errors.New("org not found")

// ErrUserNotFound is returned when a user id does not exist in the given org.
var ErrUserNotFound = errors.New("user not found")

// ErrUserInactive is returned by AuthenticateUser when a user exists but has
// been deactivated (SCIM active=false). Its agu- token stops working while the
// account is retained and can be reactivated.
var ErrUserInactive = errors.New("user inactive")

// Multi-tenant defaults from the frozen contract. Pre-existing keys belong to
// the bootstrapped default org and are attributed to the root superuser.
const (
	DefaultOrgID = "org_default"
	RootCreator  = "root"
)

// Audit entry kinds. Records persisted before kinds existed count as chat.
const (
	KindChat           = "chat"
	KindEmbeddings     = "embeddings"
	KindGuardrailFlag  = "guardrail_flag"
	KindGuardrailBlock = "guardrail_block"
	// KindGuardrailError marks a failed model-classifier call: the request
	// was allowed (fail open) but the blind spot is audited.
	KindGuardrailError = "guardrail_error"
	// KindRateLimited marks a request rejected by the per-tenant rate limiter
	// (Phase 6). No spend is recorded; only the rejection is audited.
	KindRateLimited = "rate_limited"
	// KindSecretReload marks a forced secret-source reload via
	// POST /admin/secrets/reload (Phase 7). Audited for the root actor.
	KindSecretReload = "secret_reload"
)

// Key is an authenticated virtual key.
type Key struct {
	Name             string
	MonthlyBudgetUSD float64
	SpendUSD         float64
	OrgID            string // owning org; DefaultOrgID for pre-existing keys
}

// Org is a tenant. A MonthlyBudgetUSD of 0 means unlimited (the org budget cap
// is skipped) — this keeps the bootstrapped default org behaving as Phase 1–4.
type Org struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	MonthlyBudgetUSD float64 `json:"monthly_budget_usd"`
	// RateLimitRPM is the org's requests-per-minute cap (Phase 6). 0 means
	// unlimited (the default), preserving Phase 1–5 behavior.
	RateLimitRPM int       `json:"rate_limit_rpm"`
	CreatedAt    time.Time `json:"created_at"`
}

// User is a member of an org, authenticated by an agu- token.
type User struct {
	ID    string `json:"id"`
	OrgID string `json:"org_id"`
	Email string `json:"email"`
	Role  string `json:"role"`
	// Active is false when the user has been deactivated (SCIM active=false).
	// Deactivated users fail AuthenticateUser but are retained. Pre-Phase-7
	// users default to active=true. (Phase 7)
	Active bool `json:"active"`
	// ExternalID is the IdP-assigned SCIM external id, empty when the user was
	// not provisioned via SCIM. (Phase 7)
	ExternalID string    `json:"external_id"`
	CreatedAt  time.Time `json:"created_at"`
}

// Usage records one proxied request for accounting and audit.
type Usage struct {
	KeyName      string
	Model        string
	InputTokens  int64
	OutputTokens int64
	CostUSD      float64
	LatencyMS    int64
	Status       int
	Kind         string // KindChat when empty
}

// AuditEntry is one row of GET /admin/audit, newest first.
type AuditEntry struct {
	TS           time.Time `json:"ts"`
	KeyName      string    `json:"key_name"`
	Model        string    `json:"model"`
	InputTokens  int64     `json:"input_tokens"`
	OutputTokens int64     `json:"output_tokens"`
	CostUSD      float64   `json:"cost_usd"`
	LatencyMS    int64     `json:"latency_ms"`
	Status       int       `json:"status"`
	Kind         string    `json:"kind"`
}

// KeyUsage is the per-key aggregate reported by GET /admin/usage.
type KeyUsage struct {
	Name         string  `json:"name"`
	Requests     int64   `json:"requests"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	SpendUSD     float64 `json:"spend_usd"`
}

// KeyInfo is the secret-free key listing reported by GET /admin/keys.
type KeyInfo struct {
	Name             string  `json:"name"`
	MonthlyBudgetUSD float64 `json:"monthly_budget_usd"`
	SpendUSD         float64 `json:"spend_usd"`
	OrgID            string  `json:"org_id"`
}

// Store is the persistence interface shared by the memory and Postgres
// implementations.
type Store interface {
	// CreateKey is the Phase 1–4 back-compat wrapper: it creates a key in the
	// default org attributed to the root superuser by delegating to
	// CreateKeyIn. Existing callers and tests keep compiling unchanged.
	CreateKey(ctx context.Context, name string, budgetUSD float64) (secret string, err error)
	// CreateKeyIn creates a key in orgID attributed to createdBy (a user id or
	// RootCreator). Phase 5 callers use this to scope keys to a tenant.
	CreateKeyIn(ctx context.Context, name string, budgetUSD float64, orgID, createdBy string) (secret string, err error)
	Authenticate(ctx context.Context, secret string) (*Key, error) // ErrInvalidKey
	RecordUsage(ctx context.Context, u Usage) error                // updates spend
	RecordAudit(ctx context.Context, u Usage) error                // audit log only, no spend
	Usage(ctx context.Context) ([]KeyUsage, error)
	Keys(ctx context.Context) ([]KeyInfo, error)
	AuditList(ctx context.Context, limit int) ([]AuditEntry, error) // newest first
	EnsureKey(ctx context.Context, name, secret string, budgetUSD float64) error

	// Multi-tenant RBAC additions (Phase 5).
	CreateOrg(ctx context.Context, name string, monthlyBudgetUSD float64) (*Org, error)
	EnsureOrg(ctx context.Context, id, name string, monthlyBudgetUSD float64) error // idempotent bootstrap
	Org(ctx context.Context, id string) (*Org, error)                               // ErrOrgNotFound
	Orgs(ctx context.Context) ([]Org, error)
	OrgSpend(ctx context.Context, orgID string) (float64, error) // sum of the org's keys' spend
	// UpdateOrg patches an org's monthly budget and/or rate limit (Phase 6). A
	// nil field is left unchanged. Returns the updated org, or ErrOrgNotFound.
	UpdateOrg(ctx context.Context, id string, monthlyBudgetUSD *float64, rateLimitRPM *int) (*Org, error)
	// CreateUser is the Phase 5/6 back-compat wrapper: it provisions a user with
	// no external id by delegating to CreateUserWithExternalID. Existing callers
	// keep compiling unchanged. New users are active.
	CreateUser(ctx context.Context, orgID, email, role string) (user *User, token string, err error)
	// CreateUserWithExternalID provisions a user carrying a SCIM external id
	// (empty means none). Used by the SCIM upsert path. (Phase 7)
	CreateUserWithExternalID(ctx context.Context, orgID, email, role, externalID string) (user *User, token string, err error)
	AuthenticateUser(ctx context.Context, token string) (*User, error) // ErrInvalidToken, ErrUserInactive
	// UserByEmail finds a user by email within an org (Phase 6 SSO upsert).
	// Returns ErrUserNotFound when no such user exists.
	UserByEmail(ctx context.Context, orgID, email string) (*User, error)
	// IssueUserToken rotates and returns a fresh agu- token for an existing
	// user (Phase 6 SSO login mints a token for a returning user). The prior
	// token is invalidated. Returns ErrUserNotFound for an unknown user id.
	IssueUserToken(ctx context.Context, userID string) (token string, err error)
	Users(ctx context.Context, orgID string) ([]User, error)
	DeleteUser(ctx context.Context, orgID, userID string) error // ErrUserNotFound

	// SCIM provisioning additions (Phase 7).
	// SetUserActive activates or deactivates a user by id. A deactivated user's
	// agu- token stops authenticating. Returns ErrUserNotFound for an unknown id.
	SetUserActive(ctx context.Context, userID string, active bool) error
	// SetUserExternalID sets (or clears, with "") a user's SCIM external id.
	// Returns ErrUserNotFound for an unknown id.
	SetUserExternalID(ctx context.Context, userID, externalID string) error
	// UserByExternalID finds a user by SCIM external id within an org. Returns
	// ErrUserNotFound when no such user exists.
	UserByExternalID(ctx context.Context, orgID, externalID string) (*User, error)
}

// kindOrChat maps an unset kind to KindChat.
func kindOrChat(kind string) string {
	if kind == "" {
		return KindChat
	}
	return kind
}

// newSecret generates a fresh virtual key secret of the form agos-<random>.
func newSecret() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate secret: %w", err)
	}
	return "agos-" + hex.EncodeToString(buf), nil
}

// hashSecret returns the hex SHA-256 of a secret; only hashes are stored.
func hashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// newUserToken generates a fresh user token of the form agu-<random>. The agu-
// prefix is distinct from the agos- service-key prefix.
func newUserToken() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate user token: %w", err)
	}
	return "agu-" + hex.EncodeToString(buf), nil
}

// newID generates a prefixed random id like org_<hex> or usr_<hex>.
func newID(prefix string) (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return prefix + hex.EncodeToString(buf), nil
}

// validateRole guards CreateUser against unknown roles.
func validateRole(role string) error {
	if !rbac.ValidRole(role) {
		return fmt.Errorf("invalid role %q", role)
	}
	return nil
}
