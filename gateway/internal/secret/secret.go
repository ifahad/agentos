// Package secret resolves provider keys (and future secrets) through a
// pluggable backend. The default env backend reproduces the Phase 1–4 behavior
// exactly (os.Getenv); the file and age backends are opt-in.
package secret

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// Backend identifiers selected by AGENTOS_SECRETS_BACKEND.
const (
	BackendEnv   = "env"
	BackendFile  = "file"
	BackendAge   = "age"
	BackendVault = "vault"
)

// Source resolves named secrets. Get reports the value and whether it is set;
// Backend names the active backend for the /admin/secrets/status report.
type Source interface {
	Get(name string) (value string, ok bool)
	Backend() string
}

// Reloadable is an optional interface a Source may implement to re-fetch its
// secrets on demand (Phase 7 rotation): file re-reads, age re-decrypts, vault
// re-GETs. The env backend is inherently live and does not implement it. Reload
// must be safe for concurrent use with Get. It returns the number of secret
// names whose value changed so callers can log rotation activity.
type Reloadable interface {
	Reload() (changed int, err error)
}

// envSource reads os.Getenv — the default, byte-for-byte Phase 1–4 behavior.
type envSource struct{}

// NewEnv returns the environment-variable backend.
func NewEnv() Source { return envSource{} }

func (envSource) Get(name string) (string, bool) { return os.LookupEnv(name) }
func (envSource) Backend() string                { return BackendEnv }

// fileSource reads a JSON object {"NAME":"value",...} from disk and reloads it
// whenever the file's mtime changes (hot rotation).
type fileSource struct {
	path    string
	mu      sync.Mutex
	mtime   int64
	values  map[string]string
	backend string
}

// NewFile loads a JSON secrets file at path and returns the file backend. A
// missing or malformed file is a fatal misconfiguration (returns an error).
func NewFile(path string) (Source, error) {
	if path == "" {
		return nil, fmt.Errorf("secrets file backend requires AGENTOS_SECRETS_FILE")
	}
	s := &fileSource{path: path, backend: BackendFile, values: map[string]string{}}
	if err := s.reload(); err != nil {
		return nil, err
	}
	return s, nil
}

// reload reads and parses the file, recording its mtime. Callers may hold s.mu.
func (s *fileSource) reload() error {
	info, err := os.Stat(s.path)
	if err != nil {
		return fmt.Errorf("stat secrets file %q: %w", s.path, err)
	}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return fmt.Errorf("read secrets file %q: %w", s.path, err)
	}
	values, err := parseSecretsJSON(raw)
	if err != nil {
		return fmt.Errorf("parse secrets file %q: %w", s.path, err)
	}
	s.values = values
	s.mtime = info.ModTime().UnixNano()
	return nil
}

func (s *fileSource) Get(name string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if info, err := os.Stat(s.path); err == nil && info.ModTime().UnixNano() != s.mtime {
		// mtime changed: hot-reload. On a transient error keep the last-good map.
		_ = s.reload()
	}
	v, ok := s.values[name]
	return v, ok
}

func (s *fileSource) Backend() string { return s.backend }

// Reload re-reads the file unconditionally (regardless of mtime) and reports
// how many secret values changed. On a read/parse error the last-good map is
// kept and the error is returned.
func (s *fileSource) Reload() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	before := s.values
	if err := s.reload(); err != nil {
		return 0, err
	}
	return countChanged(before, s.values), nil
}

// countChanged reports how many keys differ (added, removed, or altered value)
// between two secret maps.
func countChanged(before, after map[string]string) int {
	changed := 0
	for k, v := range after {
		if ov, ok := before[k]; !ok || ov != v {
			changed++
		}
	}
	for k := range before {
		if _, ok := after[k]; !ok {
			changed++
		}
	}
	return changed
}

// parseSecretsJSON decodes a flat JSON object of string values.
func parseSecretsJSON(raw []byte) (map[string]string, error) {
	values := map[string]string{}
	if len(raw) == 0 {
		return values, nil
	}
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	return values, nil
}

// staticSource serves an in-memory map (used by the age and vault backends,
// which materialize their secrets at startup). An optional refetch closure lets
// it re-fetch on Reload: age re-decrypts its file, vault re-GETs. When refetch
// is nil the source is not Reloadable.
type staticSource struct {
	mu      sync.Mutex
	values  map[string]string
	backend string
	refetch func() (map[string]string, error)
}

func (s *staticSource) Get(name string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.values[name]
	return v, ok
}

func (s *staticSource) Backend() string { return s.backend }

// Reload re-fetches the secrets via the backend's refetch closure and reports
// how many values changed. Sources built without a refetch closure are not
// Reloadable and this method is never surfaced (the type assertion in the
// server checks for it), but if called it is a no-op returning 0.
func (s *staticSource) Reload() (int, error) {
	if s.refetch == nil {
		return 0, nil
	}
	next, err := s.refetch()
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := countChanged(s.values, next)
	s.values = next
	return changed, nil
}

// FromEnv builds the Source selected by AGENTOS_SECRETS_BACKEND (default env),
// reading AGENTOS_SECRETS_FILE and AGENTOS_SECRETS_AGE_KEY as needed. It
// returns an error on misconfiguration so the caller can fail fast.
func FromEnv() (Source, error) {
	backend := os.Getenv("AGENTOS_SECRETS_BACKEND")
	if backend == "" {
		backend = BackendEnv
	}
	switch backend {
	case BackendEnv:
		return NewEnv(), nil
	case BackendFile:
		return NewFile(os.Getenv("AGENTOS_SECRETS_FILE"))
	case BackendAge:
		return NewAge(os.Getenv("AGENTOS_SECRETS_FILE"), os.Getenv("AGENTOS_SECRETS_AGE_KEY"))
	case BackendVault:
		return NewVault(os.Getenv("AGENTOS_VAULT_ADDR"), os.Getenv("AGENTOS_VAULT_TOKEN"), os.Getenv("AGENTOS_VAULT_KV_PATH"))
	default:
		return nil, fmt.Errorf("AGENTOS_SECRETS_BACKEND must be env, file, age, or vault (got %q)", backend)
	}
}
