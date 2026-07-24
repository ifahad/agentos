package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/ifahad/agentos/gateway/internal/safehttp"
)

// DefaultMaxAttempts is the per-request upstream attempt cap (1 retry).
const DefaultMaxAttempts = 2

// Price is USD per one million tokens.
type Price struct {
	In  float64 `json:"in"`
	Out float64 `json:"out"`
}

// Entry is one operator-configured OpenAI-compatible provider. Name is the
// model prefix: name "moonshot" routes models written "moonshot/kimi-k3".
type Entry struct {
	Name           string           `json:"name"`
	BaseURL        string           `json:"base_url"`
	KeyName        string           `json:"key_name"`
	Enabled        bool             `json:"enabled"`
	ChatPath       string           `json:"chat_path"`
	EmbeddingsPath string           `json:"embeddings_path"`
	MaxAttempts    int              `json:"max_attempts"`
	Prices         map[string]Price `json:"prices"`
}

// Registry holds validated provider entries keyed by name.
type Registry struct {
	entries map[string]Entry
}

type registryFile struct {
	Providers []Entry `json:"providers"`
}

// LoadRegistry reads a JSON registry file. A missing path yields an empty
// registry and no errors: the built-in providers alone are a valid config.
// Returned errors are per-entry validation failures — the caller logs them and
// continues, so one bad entry never prevents startup.
func LoadRegistry(path string) (*Registry, []error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Registry{entries: map[string]Entry{}}, nil
		}
		return &Registry{entries: map[string]Entry{}}, []error{fmt.Errorf("read %s: %w", path, err)}
	}
	return ParseRegistry(data)
}

// ParseRegistry validates registry JSON. Invalid entries are dropped and
// reported; valid siblings are kept.
func ParseRegistry(data []byte) (*Registry, []error) {
	reg := &Registry{entries: map[string]Entry{}}
	var file registryFile
	if err := json.Unmarshal(data, &file); err != nil {
		return reg, []error{fmt.Errorf("parse providers file: %w", err)}
	}
	var errs []error
	for _, e := range file.Providers {
		if err := validateEntry(&e); err != nil {
			errs = append(errs, fmt.Errorf("provider %q: %w", e.Name, err))
			continue
		}
		if _, dup := reg.entries[e.Name]; dup {
			errs = append(errs, fmt.Errorf("provider %q: duplicate name", e.Name))
			continue
		}
		reg.entries[e.Name] = e
	}
	return reg, errs
}

// validateEntry normalizes and screens one entry. It rejects names that would
// break prefix routing and base URLs that could aim the gateway at internal
// infrastructure (SSRF).
func validateEntry(e *Entry) error {
	e.Name = strings.TrimSpace(e.Name)
	if e.Name == "" {
		return errors.New("name is required")
	}
	if strings.ContainsAny(e.Name, "/ \t") {
		return errors.New("name must not contain '/' or whitespace")
	}
	e.BaseURL = strings.TrimSpace(strings.TrimSuffix(e.BaseURL, "/"))
	if e.BaseURL == "" {
		return errors.New("base_url is required")
	}
	u, err := url.Parse(e.BaseURL)
	if err != nil {
		return fmt.Errorf("base_url is not a URL: %w", err)
	}
	host := u.Hostname()
	if host == "" {
		return errors.New("base_url has no host")
	}
	loopback := isLoopbackHost(host)
	switch u.Scheme {
	case "https":
	case "http":
		if !loopback {
			return errors.New("base_url must use https (http allowed only for loopback)")
		}
	default:
		return fmt.Errorf("base_url scheme %q must be https", u.Scheme)
	}
	// Loopback is explicitly permitted (a local Ollama-style server); every
	// other host must not resolve into private/link-local space.
	if !loopback && safehttp.IsDisallowedHost(host) {
		return errors.New("base_url host resolves to a private, loopback, or link-local address")
	}
	if e.MaxAttempts <= 0 {
		e.MaxAttempts = DefaultMaxAttempts
	}
	if e.ChatPath == "" {
		e.ChatPath = "/v1/chat/completions"
	}
	if e.EmbeddingsPath == "" {
		e.EmbeddingsPath = "/v1/embeddings"
	}
	return nil
}

// isLoopbackHost reports whether host is a literal loopback address or the
// name "localhost". Docker service names are NOT loopback and are screened.
func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// Lookup returns the entry registered under name.
func (r *Registry) Lookup(name string) (Entry, bool) {
	if r == nil {
		return Entry{}, false
	}
	e, ok := r.entries[name]
	return e, ok
}

// Names returns the registered provider names, sorted.
func (r *Registry) Names() []string {
	if r == nil {
		return nil
	}
	names := make([]string, 0, len(r.entries))
	for n := range r.entries {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
