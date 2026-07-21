package secret

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"filippo.io/age"
)

// NewAge decrypts an age-encrypted JSON secrets file in-memory at startup using
// the X25519 identity in ageKey (an "AGE-SECRET-KEY-1..." string). No plaintext
// ever touches disk. A missing file or bad key is a fatal misconfiguration.
func NewAge(path, ageKey string) (Source, error) {
	if path == "" {
		return nil, fmt.Errorf("secrets age backend requires AGENTOS_SECRETS_FILE")
	}
	if ageKey == "" {
		return nil, fmt.Errorf("secrets age backend requires AGENTOS_SECRETS_AGE_KEY")
	}
	identity, err := age.ParseX25519Identity(ageKey)
	if err != nil {
		return nil, fmt.Errorf("parse AGENTOS_SECRETS_AGE_KEY: %w", err)
	}
	ciphertext, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read age secrets file %q: %w", path, err)
	}
	r, err := age.Decrypt(bytes.NewReader(ciphertext), identity)
	if err != nil {
		return nil, fmt.Errorf("decrypt age secrets file %q: %w", path, err)
	}
	plaintext, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read decrypted secrets: %w", err)
	}
	values, err := parseSecretsJSON(plaintext)
	if err != nil {
		return nil, fmt.Errorf("parse decrypted secrets: %w", err)
	}
	return &staticSource{values: values, backend: BackendAge}, nil
}
