package store

import (
	"context"
	"sort"
	"sync"
)

type memoryKey struct {
	name      string
	budgetUSD float64
	spendUSD  float64
}

// Memory is an in-process Store used when AGENTOS_DATABASE_URL is empty.
type Memory struct {
	mu    sync.Mutex
	keys  map[string]*memoryKey // secret hash -> key
	usage map[string]*KeyUsage  // key name -> aggregate
	audit []Usage
}

// NewMemory returns an empty in-memory store.
func NewMemory() *Memory {
	return &Memory{
		keys:  make(map[string]*memoryKey),
		usage: make(map[string]*KeyUsage),
	}
}

func (m *Memory) CreateKey(_ context.Context, name string, budgetUSD float64) (string, error) {
	secret, err := newSecret()
	if err != nil {
		return "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.keys[hashSecret(secret)] = &memoryKey{name: name, budgetUSD: budgetUSD}
	return secret, nil
}

func (m *Memory) Authenticate(_ context.Context, secret string) (*Key, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k, ok := m.keys[hashSecret(secret)]
	if !ok {
		return nil, ErrInvalidKey
	}
	return &Key{Name: k.name, MonthlyBudgetUSD: k.budgetUSD, SpendUSD: k.spendUSD}, nil
}

func (m *Memory) RecordUsage(_ context.Context, u Usage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, k := range m.keys {
		if k.name == u.KeyName {
			k.spendUSD += u.CostUSD
		}
	}
	agg, ok := m.usage[u.KeyName]
	if !ok {
		agg = &KeyUsage{Name: u.KeyName}
		m.usage[u.KeyName] = agg
	}
	agg.Requests++
	agg.InputTokens += u.InputTokens
	agg.OutputTokens += u.OutputTokens
	agg.SpendUSD += u.CostUSD
	m.audit = append(m.audit, u)
	return nil
}

func (m *Memory) Usage(_ context.Context) ([]KeyUsage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	seen := make(map[string]bool)
	var out []KeyUsage
	for name, agg := range m.usage {
		seen[name] = true
		out = append(out, *agg)
	}
	for _, k := range m.keys {
		if !seen[k.name] {
			seen[k.name] = true
			out = append(out, KeyUsage{Name: k.name})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (m *Memory) Keys(_ context.Context) ([]KeyInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]KeyInfo, 0, len(m.keys))
	for _, k := range m.keys {
		out = append(out, KeyInfo{Name: k.name, MonthlyBudgetUSD: k.budgetUSD, SpendUSD: k.spendUSD})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (m *Memory) EnsureKey(_ context.Context, name, secret string, budgetUSD float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	h := hashSecret(secret)
	if k, ok := m.keys[h]; ok {
		k.name = name
		k.budgetUSD = budgetUSD
		return nil // spend preserved
	}
	m.keys[h] = &memoryKey{name: name, budgetUSD: budgetUSD}
	return nil
}

// Audit returns a copy of the recorded audit entries (memory store only).
func (m *Memory) Audit() []Usage {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Usage, len(m.audit))
	copy(out, m.audit)
	return out
}
