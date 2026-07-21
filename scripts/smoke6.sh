#!/usr/bin/env bash
# Phase 6 end-to-end smoke test: whoami, per-tenant rate limits, OIDC SSO
# (against the mock provider), and the Vault secrets backend (against mock
# Vault). Requires the stack up (make up). Brings the mocks up itself.
set -euo pipefail

export GATEWAY=${GATEWAY:-http://localhost:8080}
export AGENTOS_ADMIN_KEY=${AGENTOS_ADMIN_KEY:-admin-local-dev}
ADMIN="Authorization: Bearer $AGENTOS_ADMIN_KEY"
MODEL=${AGENTOS_MODEL:-ollama/qwen3.6:latest}
COMPOSE="docker compose -f deploy/compose.yaml --env-file deploy/.env"
MOCKS="$COMPOSE -f deploy/compose.auth-mocks.yaml"
JQ() { python3 -c "import json,sys;d=json.load(sys.stdin);print($1)"; }

say()  { printf '\n\033[1m== %s\033[0m\n' "$*"; }
fail() { echo "FAIL: $*"; exit 1; }

say "whoami: root admin key identifies as root"
curl -fsS "$GATEWAY/admin/whoami" -H "$ADMIN" | JQ "d['root']" | grep -qi true \
  || fail "root whoami should report root:true"
echo "PASS"

say "whoami: a user token resolves its org/email/role"
ORG=$(curl -fsS -X POST "$GATEWAY/admin/orgs" -H "$ADMIN" -H 'Content-Type: application/json' \
  -d '{"name":"phase6","monthly_budget_usd":0,"rate_limit_rpm":2}')
ORG_ID=$(echo "$ORG" | JQ "d['id']")
USER=$(curl -fsS -X POST "$GATEWAY/admin/orgs/$ORG_ID/users" -H "$ADMIN" -H 'Content-Type: application/json' \
  -d '{"email":"p6@acme.test","role":"admin"}')
UTOKEN=$(echo "$USER" | JQ "d['token']")
WHO=$(curl -fsS "$GATEWAY/admin/whoami" -H "Authorization: Bearer $UTOKEN")
echo "$WHO" | python3 -m json.tool
echo "$WHO" | JQ "d['org_id']" | grep -q "$ORG_ID" || fail "whoami org mismatch"
echo "$WHO" | JQ "d['role']" | grep -q admin || fail "whoami role mismatch"
echo "$WHO" | JQ "d['email']" | grep -q 'p6@acme.test' || fail "whoami email mismatch"
echo "PASS"

say "rate limit: org capped at 2 rpm returns 429 with Retry-After"
KEY=$(curl -fsS -X POST "$GATEWAY/admin/keys" -H "Authorization: Bearer $UTOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"p6-key","monthly_budget_usd":5}' | JQ "d['key']")
codes=""
for i in 1 2 3 4; do
  c=$(curl -s -o /tmp/agentos-rl.json -D /tmp/agentos-rl.hdr -w '%{http_code}' -X POST "$GATEWAY/v1/chat/completions" \
    -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' \
    -d '{"model":"'"$MODEL"'","messages":[{"role":"user","content":"ping '"$i"'"}]}' || true)
  codes="$codes $c"
done
echo "response codes:$codes"
echo "$codes" | grep -q 429 || fail "expected a 429 within the burst (got:$codes)"
grep -qi '^Retry-After:' /tmp/agentos-rl.hdr || fail "429 missing Retry-After header"
grep -q 'rate_limited' /tmp/agentos-rl.json || fail "wrong error type on 429"
echo "PASS (rate limited with Retry-After: $(grep -i '^Retry-After:' /tmp/agentos-rl.hdr | tr -d '\r'))"

say "bring up mock OIDC + mock Vault"
$MOCKS up -d mock-oidc mock-vault >/dev/null 2>&1
for i in $(seq 1 30); do
  curl -fsS "http://localhost:${AGENTOS_MOCK_OIDC_PORT:-9000}/.well-known/openid-configuration" >/dev/null 2>&1 && break
  sleep 1
done
curl -fsS "http://localhost:${AGENTOS_MOCK_OIDC_PORT:-9000}/.well-known/openid-configuration" >/dev/null \
  || fail "mock-oidc not serving discovery"
echo "PASS"

say "OIDC SSO: full login flow against the mock provider mints an agu- token"
AGENTOS_OIDC_ISSUER=http://mock-oidc:9000 \
  AGENTOS_OIDC_CLIENT_ID=agentos-console \
  AGENTOS_OIDC_CLIENT_SECRET=mock-secret \
  AGENTOS_OIDC_REDIRECT_URL=http://gateway:8080/auth/oidc/callback \
  AGENTOS_OIDC_POST_LOGIN_URL=http://console/ \
  $COMPOSE up -d gateway >/dev/null 2>&1
for i in $(seq 1 30); do curl -fsS "$GATEWAY/healthz" >/dev/null 2>&1 && break; sleep 2; done
curl -fsS "$GATEWAY/auth/oidc/status" | JQ "d['enabled']" | grep -qi true || fail "SSO not enabled"
# Drive the browser flow from inside the runtime container (it can reach both
# gateway:8080 and mock-oidc:9000 on the compose network). Manual redirect
# following so we can read the #token fragment from the final Location.
TOKEN=$($COMPOSE exec -T runtime python - <<'PY'
import urllib.request, urllib.error, urllib.parse
class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise urllib.error.HTTPError(req.full_url, code, msg, headers, fp)
op = urllib.request.build_opener(NoRedirect)
def loc(url):
    try:
        op.open(url); return None
    except urllib.error.HTTPError as e:
        if e.code in (301,302,303,307,308): return e.headers.get("Location")
        raise
url = "http://gateway:8080/auth/oidc/login"
for _ in range(6):
    nxt = loc(url)
    if nxt is None: break
    if nxt.startswith("/"): nxt = "http://gateway:8080" + nxt
    frag = urllib.parse.urlparse(nxt).fragment
    if "token=" in (frag or ""):
        print(urllib.parse.parse_qs(frag)["token"][0]); break
    url = nxt
PY
)
echo "minted token: ${TOKEN:0:16}..."
echo "$TOKEN" | grep -q '^agu-' || fail "SSO did not mint an agu- token"
# the minted token must resolve via whoami to the mock user's email
curl -fsS "$GATEWAY/admin/whoami" -H "Authorization: Bearer $TOKEN" | JQ "d['email']" \
  | grep -q 'alice@example.com' || fail "SSO user email mismatch"
echo "PASS (SSO login → agu- token → whoami=alice@example.com)"
$COMPOSE up -d gateway >/dev/null 2>&1   # restore SSO-disabled gateway

say "Vault secrets backend: gateway loads provider keys from mock Vault"
AGENTOS_SECRETS_BACKEND=vault \
  AGENTOS_VAULT_ADDR=http://mock-vault:8200 \
  AGENTOS_VAULT_TOKEN=test-token \
  AGENTOS_VAULT_KV_PATH=secret/data/agentos \
  $COMPOSE up -d gateway >/dev/null 2>&1
for i in $(seq 1 30); do curl -fsS "$GATEWAY/healthz" >/dev/null 2>&1 && break; sleep 2; done
$COMPOSE logs gateway 2>/dev/null | grep -qi 'secrets backend: vault' || fail "vault backend not active"
STATUS=$(curl -fsS "$GATEWAY/admin/secrets/status" -H "$ADMIN")
echo "$STATUS" | python3 -m json.tool
echo "$STATUS" | JQ "all(s['source']=='vault' for s in d)" | grep -q True || fail "secrets source should be vault"
echo "$STATUS" | JQ "any(s['name']=='AGENTOS_ANTHROPIC_API_KEY' and s['present'] for s in d)" | grep -q True \
  || fail "vault-provided key not present"
echo "PASS (provider keys sourced from Vault)"
$COMPOSE up -d gateway >/dev/null 2>&1   # restore env backend

say "backward compatibility: default gateway still healthy, whoami root works"
for i in $(seq 1 20); do curl -fsS "$GATEWAY/healthz" >/dev/null 2>&1 && break; sleep 2; done
curl -fsS "$GATEWAY/admin/whoami" -H "$ADMIN" | JQ "d['root']" | grep -qi true || fail "post-restore whoami broke"
$MOCKS stop mock-oidc mock-vault >/dev/null 2>&1 || true
echo "PASS"

echo
echo "phase 6 smoke test passed"
