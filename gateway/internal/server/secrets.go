package server

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/ifahad/agentos/gateway/internal/secret"
	"github.com/ifahad/agentos/gateway/internal/store"
)

// ReloadSecrets forces the active secret source to re-fetch when it implements
// secret.Reloadable (file re-reads, age re-decrypts, vault re-GETs); the env
// backend is a no-op. It returns how many secret values changed, whether the
// source was reloadable, and any reload error. Shared by the admin endpoint and
// the background refresh loop.
func (s *Server) ReloadSecrets() (changed int, reloadable bool, err error) {
	r, ok := s.secrets.(secret.Reloadable)
	if !ok {
		return 0, false, nil
	}
	changed, err = r.Reload()
	return changed, true, err
}

// handleSecretsReload forces a secret reload (root only) and returns the fresh
// /admin/secrets/status array. It audits a store.KindSecretReload event.
func (s *Server) handleSecretsReload(w http.ResponseWriter, r *http.Request, c *caller) {
	if !c.root {
		s.forbid(w, r, c, "only the root admin key may reload secrets")
		return
	}
	if _, _, err := s.ReloadSecrets(); err != nil {
		writeError(w, http.StatusBadGateway, errProviderError, "failed to reload secrets: "+err.Error())
		return
	}
	// Audit the reload against the root actor. Kind marks it as a secret event.
	s.recordAudit(r, store.Usage{KeyName: store.RootCreator, Model: s.secrets.Backend(), Status: http.StatusOK, Kind: store.KindSecretReload})
	writeJSON(w, http.StatusOK, s.secretsStatus())
}

// StartSecretsRefresh launches a background goroutine that reloads the secret
// source every interval when it is Reloadable, logging only when values change.
// A non-positive interval or a non-Reloadable source is a no-op. The goroutine
// exits when ctx is cancelled.
func (s *Server) StartSecretsRefresh(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		return
	}
	if _, ok := s.secrets.(secret.Reloadable); !ok {
		log.Printf("secrets refresh requested but backend %q is not reloadable; ignoring", s.secrets.Backend())
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				changed, _, err := s.ReloadSecrets()
				if err != nil {
					log.Printf("secrets refresh: reload failed: %v", err)
					continue
				}
				if changed > 0 {
					log.Printf("secrets refresh: %d secret value(s) changed", changed)
				}
			}
		}
	}()
}
