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
	BackendEnv  = "env"
	BackendFile = "file"
	BackendAge  = "age"
)

// Source resolves named secrets. Get reports the value and whether it is set;
// Backend names the active backend for the /admin/secrets/status report.
type Source interface {
	Get(name string) (value string, ok bool)
	Backend() string
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

// staticSource serves a fixed map (used by the age backend after in-memory
// decryption at startup).
type staticSource struct {
	values  map[string]string
	backend string
}

func (s *staticSource) Get(name string) (string, bool) {
	v, ok := s.values[name]
	return v, ok
}

func (s *staticSource) Backend() string { return s.backend }

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
	default:
		return nil, fmt.Errorf("AGENTOS_SECRETS_BACKEND must be env, file, or age (got %q)", backend)
	}
}
