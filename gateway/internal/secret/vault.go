package secret

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// NewVault reads a HashiCorp Vault KV v2 secret at construction and serves it
// from a static in-memory map. It performs a single GET {addr}/v1/{kvPath} with
// the X-Vault-Token header and parses the KV v2 envelope
// {"data":{"data":{NAME:VALUE,…}}}. Any misconfiguration — empty inputs, an
// unreachable server, a non-200 status (e.g. 403 on a bad token), or malformed
// JSON — is a fatal error the caller surfaces at startup. No live reads happen
// after construction (rotation is future work).
func NewVault(addr, token, kvPath string) (Source, error) {
	return newVaultWithClient(addr, token, kvPath, &http.Client{Timeout: 10 * time.Second})
}

func newVaultWithClient(addr, token, kvPath string, client *http.Client) (Source, error) {
	if addr == "" {
		return nil, fmt.Errorf("vault secrets backend requires AGENTOS_VAULT_ADDR")
	}
	if token == "" {
		return nil, fmt.Errorf("vault secrets backend requires AGENTOS_VAULT_TOKEN")
	}
	if kvPath == "" {
		return nil, fmt.Errorf("vault secrets backend requires AGENTOS_VAULT_KV_PATH")
	}
	url := strings.TrimRight(addr, "/") + "/v1/" + strings.TrimLeft(kvPath, "/")

	// fetchVault performs one KV v2 GET; used at startup and on Reload (Phase 7
	// rotation) so a re-GET picks up a rotated secret without a restart.
	fetchVault := func() (map[string]string, error) {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("build vault request: %w", err)
		}
		req.Header.Set("X-Vault-Token", token)

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("reach vault at %q: %w", url, err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read vault response: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("vault returned status %d for %q: %s", resp.StatusCode, url, strings.TrimSpace(string(body)))
		}

		var env struct {
			Data struct {
				Data map[string]string `json:"data"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &env); err != nil {
			return nil, fmt.Errorf("parse vault KV v2 response from %q: %w", url, err)
		}
		if env.Data.Data == nil {
			return nil, fmt.Errorf("vault response from %q missing data.data object", url)
		}
		return env.Data.Data, nil
	}
	values, err := fetchVault()
	if err != nil {
		return nil, err
	}
	return &staticSource{values: values, backend: BackendVault, refetch: fetchVault}, nil
}
