# deploy/openclaw — governed OpenClaw worker (template)

An **illustrative** overlay for running an OpenClaw worker as a budgeted, audited,
contained client of the AgentOS gateway. Read `docs/interop/openclaw.md` first.

```bash
# 1) create a scoped virtual key (see docs/interop/openclaw.md, Step 1)
export AGENTOS_OPENCLAW_KEY=agos-...     # a key inside your openclaw-fleet org

# 2) start the governed worker (adapt OPENCLAW_IMAGE to your version)
docker compose -f deploy/compose.yaml -f deploy/openclaw/compose.openclaw.yaml up -d openclaw
```

What the overlay enforces: model traffic only through `gateway:8080/v1`;
non-root, read-only rootfs, all caps dropped, no-new-privileges; skills mounted
read-only from a vetted dir (no ClawHub auto-fetch); the control plane (:18789)
not published. It is a **template** — pin the image to a digest and restrict
egress to the gateway with a firewall/NetworkPolicy in production.
