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
)

// ErrInvalidKey is returned by Authenticate when the secret is unknown.
var ErrInvalidKey = errors.New("invalid key")

// Audit entry kinds. Records persisted before kinds existed count as chat.
const (
	KindChat           = "chat"
	KindEmbeddings     = "embeddings"
	KindGuardrailFlag  = "guardrail_flag"
	KindGuardrailBlock = "guardrail_block"
	// KindGuardrailError marks a failed model-classifier call: the request
	// was allowed (fail open) but the blind spot is audited.
	KindGuardrailError = "guardrail_error"
)

// Key is an authenticated virtual key.
type Key struct {
	Name             string
	MonthlyBudgetUSD float64
	SpendUSD         float64
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
}

// Store is the persistence interface shared by the memory and Postgres
// implementations.
type Store interface {
	CreateKey(ctx context.Context, name string, budgetUSD float64) (secret string, err error)
	Authenticate(ctx context.Context, secret string) (*Key, error) // ErrInvalidKey
	RecordUsage(ctx context.Context, u Usage) error                // updates spend
	RecordAudit(ctx context.Context, u Usage) error                // audit log only, no spend
	Usage(ctx context.Context) ([]KeyUsage, error)
	Keys(ctx context.Context) ([]KeyInfo, error)
	AuditList(ctx context.Context, limit int) ([]AuditEntry, error) // newest first
	EnsureKey(ctx context.Context, name, secret string, budgetUSD float64) error
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
