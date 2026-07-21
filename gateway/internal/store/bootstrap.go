package store

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// BootstrapKey is one parsed AGENTOS_BOOTSTRAP_KEYS entry.
type BootstrapKey struct {
	Name      string
	Secret    string
	BudgetUSD float64
}

// ParseBootstrapKeys parses the AGENTOS_BOOTSTRAP_KEYS env var: a
// comma-separated list of name:secret:monthly_budget_usd entries.
// An empty value yields no keys and no error.
func ParseBootstrapKeys(raw string) ([]BootstrapKey, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var out []BootstrapKey
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.Split(entry, ":")
		if len(parts) != 3 {
			return nil, fmt.Errorf("bootstrap key entry %q: want name:secret:monthly_budget_usd", entry)
		}
		name, secret := parts[0], parts[1]
		if name == "" || secret == "" {
			return nil, fmt.Errorf("bootstrap key entry %q: name and secret must be non-empty", entry)
		}
		budget, err := strconv.ParseFloat(parts[2], 64)
		if err != nil {
			return nil, fmt.Errorf("bootstrap key entry %q: invalid budget %q", entry, parts[2])
		}
		out = append(out, BootstrapKey{Name: name, Secret: secret, BudgetUSD: budget})
	}
	return out, nil
}

// ApplyBootstrapKeys idempotently upserts each bootstrap key into the store.
func ApplyBootstrapKeys(ctx context.Context, st Store, keys []BootstrapKey) error {
	for _, k := range keys {
		if err := st.EnsureKey(ctx, k.Name, k.Secret, k.BudgetUSD); err != nil {
			return fmt.Errorf("bootstrap key %q: %w", k.Name, err)
		}
	}
	return nil
}
