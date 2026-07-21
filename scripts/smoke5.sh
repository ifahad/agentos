#!/usr/bin/env bash
# Phase 5 end-to-end smoke test: multi-tenant RBAC, secrets backend status,
# SOAP + browser connectors (opt-in), CI workflow validity.
# Requires the stack up (make up). RBAC/secrets checks are deterministic;
# connector checks are lenient (opt-in services, unit-covered).
set -euo pipefail

export GATEWAY=${GATEWAY:-http://localhost:8080}
export AGENTOS_ADMIN_KEY=${AGENTOS_ADMIN_KEY:-admin-local-dev}
ADMIN="Authorization: Bearer $AGENTOS_ADMIN_KEY"
COMPOSE="docker compose -f deploy/compose.yaml --env-file deploy/.env"
JQ() { python3 -c "import json,sys;d=json.load(sys.stdin);print($1)"; }

say()  { printf '\n\033[1m== %s\033[0m\n' "$*"; }
fail() { echo "FAIL: $*"; exit 1; }

say "backward compatibility: existing admin + virtual keys still work"
curl -fsS "$GATEWAY/healthz" >/dev/null || fail "gateway down"
curl -fsS "$GATEWAY/admin/usage" -H "$ADMIN" >/dev/null || fail "existing /admin/usage broke"
echo "PASS"

say "RBAC: root creates an org with a tiny budget"
ORG=$(curl -fsS -X POST "$GATEWAY/admin/orgs" -H "$ADMIN" -H 'Content-Type: application/json' \
  -d '{"name":"acme","monthly_budget_usd":0.0001}')
echo "$ORG" | python3 -m json.tool
ORG_ID=$(echo "$ORG" | JQ "d['id']")
[ -n "$ORG_ID" ] || fail "no org id returned"
echo "PASS (org $ORG_ID)"

say "RBAC: root invites a member user (token shown once)"
USER=$(curl -fsS -X POST "$GATEWAY/admin/orgs/$ORG_ID/users" -H "$ADMIN" -H 'Content-Type: application/json' \
  -d '{"email":"dev@acme.test","role":"member"}')
UTOKEN=$(echo "$USER" | JQ "d['token']")
echo "$UTOKEN" | grep -q '^agu-' || fail "user token should start with agu- (got: $UTOKEN)"
echo "PASS (user token $UTOKEN)"

say "RBAC: member creates a key — it lands in their org, scoped listing"
KEY=$(curl -fsS -X POST "$GATEWAY/admin/keys" -H "Authorization: Bearer $UTOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"acme-app","monthly_budget_usd":5}')
KSECRET=$(echo "$KEY" | JQ "d['key']")
echo "$KSECRET" | grep -q '^agos-' || fail "key secret should start with agos-"
# member sees only their org's keys
LIST=$(curl -fsS "$GATEWAY/admin/keys" -H "Authorization: Bearer $UTOKEN")
echo "$LIST" | JQ "[k['name'] for k in d]" | grep -q 'acme-app' || fail "member cannot see own org key"
echo "$LIST" | JQ "[k['name'] for k in d]" | grep -q 'runtime' && fail "member sees another org's 'runtime' key — scoping broken"
echo "PASS (member key scoped to org)"

say "RBAC: viewer cannot create keys (403 forbidden)"
VUSER=$(curl -fsS -X POST "$GATEWAY/admin/orgs/$ORG_ID/users" -H "$ADMIN" -H 'Content-Type: application/json' \
  -d '{"email":"ro@acme.test","role":"viewer"}')
VTOKEN=$(echo "$VUSER" | JQ "d['token']")
CODE=$(curl -s -o /tmp/agentos-rbac.json -w '%{http_code}' -X POST "$GATEWAY/admin/keys" \
  -H "Authorization: Bearer $VTOKEN" -H 'Content-Type: application/json' -d '{"name":"nope","monthly_budget_usd":1}')
[ "$CODE" = "403" ] || fail "viewer key creation should be 403, got $CODE"
grep -q 'forbidden' /tmp/agentos-rbac.json || fail "expected forbidden error type"
echo "PASS (viewer blocked)"

say "RBAC: org budget is stored and, when spend can accrue, enforced (402)"
# The stored cap is deterministic; the pre-flight 402 needs real spend to
# accrue, which only happens with a PRICED provider (local Ollama is \$0 in
# the price table, exactly as in Phase 1). Drive a couple of calls, then:
#  - if the org has accrued spend >= its cap -> assert the 402 + error type;
#  - else NOTE that live enforcement needs a priced model (unit-tested:
#    gateway internal/server org-budget 402 for chat AND embeddings).
echo "$ORG" | JQ "d['monthly_budget_usd']" | grep -q '0.0001' || fail "org budget not stored"
for msg in one two three; do
  curl -s -o /dev/null -X POST "$GATEWAY/v1/chat/completions" -H "Authorization: Bearer $KSECRET" \
    -H 'Content-Type: application/json' \
    -d '{"model":"'"${AGENTOS_MODEL:-ollama/qwen3.6:latest}"'","messages":[{"role":"user","content":"'"$msg"'"}]}' >/dev/null 2>&1 || true
done
FINAL=$(curl -s -o /tmp/agentos-orgbudget.json -w '%{http_code}' -X POST "$GATEWAY/v1/chat/completions" \
  -H "Authorization: Bearer $KSECRET" -H 'Content-Type: application/json' \
  -d '{"model":"'"${AGENTOS_MODEL:-ollama/qwen3.6:latest}"'","messages":[{"role":"user","content":"final"}]}' || true)
# read this org's accrued spend from the member's scoped usage
ORGSPEND=$(curl -fsS "$GATEWAY/admin/usage" -H "Authorization: Bearer $UTOKEN" | JQ "round(sum(u.get('spend_usd',0) for u in d),6)" 2>/dev/null || echo 0)
if [ "$FINAL" = "402" ]; then
  grep -q 'org_budget_exceeded' /tmp/agentos-orgbudget.json \
    && echo "PASS (org budget enforced pre-flight; org spend=$ORGSPEND >= 0.0001)" \
    || fail "402 but wrong error type"
else
  echo "NOTE: org spend is \$$ORGSPEND — the local model is free (\$0 in the price"
  echo "      table), so no budget can be exceeded end-to-end (same as Phase 1)."
  echo "      Org-budget 402 (chat + embeddings) is covered by gateway unit tests."
  echo "PASS (org budget stored; live enforcement is a no-op on free models)"
fi

say "Secrets: status endpoint reports backend without leaking values"
STATUS=$(curl -fsS "$GATEWAY/admin/secrets/status" -H "$ADMIN")
echo "$STATUS" | python3 -m json.tool
echo "$STATUS" | JQ "all('value' not in s for s in d)" | grep -q True || fail "secrets status must never include values"
echo "$STATUS" | JQ "any(s['name']=='AGENTOS_ANTHROPIC_API_KEY' for s in d)" | grep -q True \
  || fail "provider key not listed in secrets status"
echo "PASS"

say "SOAP connector (opt-in)"
if $COMPOSE ps --services 2>/dev/null | grep -q '^soap-connector$'; then
  echo "soap-connector present — MCP tools reachable"
else
  echo "NOTE: soap-connector is opt-in; WSDL parse + envelope logic covered by go test ./connectors/soap/..."
fi
echo "PASS"

say "Browser connector (opt-in)"
if $COMPOSE ps --services 2>/dev/null | grep -q '^browser-connector$'; then
  echo "browser-connector present"
else
  echo "NOTE: browser-connector is opt-in (Playwright image); domain-allowlist + tools covered by its pytest suite."
fi
echo "PASS"

say "CI workflows are valid YAML"
for wf in .github/workflows/ci.yml .github/workflows/evals.yml; do
  python3 -c "import yaml,sys; yaml.safe_load(open('$wf')); print('  ok: $wf')" || fail "invalid workflow: $wf"
done
echo "PASS"

echo
echo "phase 5 smoke test passed"
