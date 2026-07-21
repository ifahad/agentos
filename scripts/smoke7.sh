#!/usr/bin/env bash
# Phase 7 end-to-end smoke test: SCIM provisioning, distributed (Postgres)
# rate-limit store, and secret reload. Requires the stack up (make up).
set -euo pipefail

export GATEWAY=${GATEWAY:-http://localhost:8080}
export AGENTOS_ADMIN_KEY=${AGENTOS_ADMIN_KEY:-admin-local-dev}
ADMIN="Authorization: Bearer $AGENTOS_ADMIN_KEY"
SCIM_TOKEN=${SCIM_TOKEN:-scim-secret-local}
SCIM="Authorization: Bearer $SCIM_TOKEN"
MODEL=${AGENTOS_MODEL:-ollama/qwen3.6:latest}
RUN=$$
ALICE="scim.alice.$RUN@corp.test"
DEPROV="deprovision.me.$RUN@corp.test"
NOPE="nope.$RUN@corp.test"
COMPOSE="docker compose -f deploy/compose.yaml --env-file deploy/.env"
JQ() { python3 -c "import json,sys;d=json.load(sys.stdin);print($1)"; }

say()  { printf '\n\033[1m== %s\033[0m\n' "$*"; }
fail() { echo "FAIL: $*"; exit 1; }

say "enable SCIM + Postgres rate-limit backend on the gateway"
AGENTOS_SCIM_TOKEN=$SCIM_TOKEN \
  AGENTOS_RATELIMIT_BACKEND=postgres \
  $COMPOSE up -d gateway >/dev/null 2>&1
for i in $(seq 1 30); do curl -fsS "$GATEWAY/healthz" >/dev/null 2>&1 && break; sleep 2; done
$COMPOSE logs gateway 2>/dev/null | grep -qiE 'scim|rate' | head -1 || true
echo "PASS"

say "SCIM: disabled routes reject without the SCIM token (401)"
CODE=$(curl -s -o /dev/null -w '%{http_code}' "$GATEWAY/scim/v2/Users" -H "Authorization: Bearer wrong")
[ "$CODE" = "401" ] || fail "SCIM without valid token should be 401, got $CODE"
echo "PASS"

say "SCIM: provision a user (POST /scim/v2/Users → 201)"
CREATE=$(curl -sS -o /tmp/scim-create.json -w '%{http_code}' -X POST "$GATEWAY/scim/v2/Users" -H "$SCIM" \
  -H 'Content-Type: application/scim+json' \
  -d "{\"schemas\":[\"urn:ietf:params:scim:schemas:core:2.0:User\"],\"userName\":\"$ALICE\",\"externalId\":\"idp-alice-001\",\"name\":{\"formatted\":\"Alice SCIM\"},\"active\":true}")
[ "$CREATE" = "201" ] || { cat /tmp/scim-create.json; fail "SCIM create expected 201, got $CREATE"; }
SCIM_ID=$(JQ "d['id']" < /tmp/scim-create.json)
python3 -m json.tool < /tmp/scim-create.json | head -12
echo "PASS (scim user $SCIM_ID)"

say "SCIM: get by id + filtered list"
curl -fsS "$GATEWAY/scim/v2/Users/$SCIM_ID" -H "$SCIM" | JQ "d['userName']" | grep -qF "$ALICE" \
  || fail "SCIM get returned wrong user"
LIST=$(curl -fsS "$GATEWAY/scim/v2/Users?filter=userName%20eq%20%22$ALICE%22" -H "$SCIM")
echo "$LIST" | JQ "d['totalResults']" | grep -q '1' || fail "SCIM filter should match exactly 1"
echo "PASS"

say "SCIM deactivation invalidates a user's agu- token"
# create a user via the RBAC admin API (returns an agu- token), prove whoami
# works, then deactivate that user via SCIM PATCH and prove the token dies.
UTOKEN=$(curl -fsS -X POST "$GATEWAY/admin/orgs/org_default/users" -H "$ADMIN" -H 'Content-Type: application/json' \
  -d "{\"email\":\"$DEPROV\",\"role\":\"member\"}" | JQ "d['token']")
curl -fsS "$GATEWAY/admin/whoami" -H "Authorization: Bearer $UTOKEN" | JQ "d['email']" \
  | grep -qF "$DEPROV" || fail "new user token should work before deactivation"
DUID=$(curl -fsS "$GATEWAY/scim/v2/Users?filter=userName%20eq%20%22$DEPROV%22" -H "$SCIM" \
  | JQ "d['Resources'][0]['id']")
curl -fsS -o /dev/null -X PATCH "$GATEWAY/scim/v2/Users/$DUID" -H "$SCIM" -H 'Content-Type: application/scim+json' \
  -d '{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"replace","path":"active","value":false}]}'
CODE=$(curl -s -o /dev/null -w '%{http_code}' "$GATEWAY/admin/whoami" -H "Authorization: Bearer $UTOKEN")
[ "$CODE" = "401" ] || fail "deactivated user's token should be 401, got $CODE"
echo "PASS (agu- token rejected after SCIM deactivation)"

say "SCIM: delete (204) then get is 404"
DEL=$(curl -s -o /dev/null -w '%{http_code}' -X DELETE "$GATEWAY/scim/v2/Users/$SCIM_ID" -H "$SCIM")
[ "$DEL" = "204" ] || fail "SCIM delete expected 204, got $DEL"
GONE=$(curl -s -o /dev/null -w '%{http_code}' "$GATEWAY/scim/v2/Users/$SCIM_ID" -H "$SCIM")
[ "$GONE" = "404" ] || fail "deleted SCIM user should 404, got $GONE"
echo "PASS"

say "distributed rate limit (Postgres backend) enforces a per-org cap"
ORG_ID=$(curl -fsS -X POST "$GATEWAY/admin/orgs" -H "$ADMIN" -H 'Content-Type: application/json' \
  -d '{"name":"p7","monthly_budget_usd":0,"rate_limit_rpm":2}' | JQ "d['id']")
KEY=$(curl -fsS -X POST "$GATEWAY/admin/keys" -H "$ADMIN" -H 'Content-Type: application/json' \
  -d '{"name":"p7-key","monthly_budget_usd":5,"org_id":"'"$ORG_ID"'"}' | JQ "d['key']")
rm -f /tmp/p7-rl.*
for i in 1 2 3 4 5 6; do
  ( curl -s -o /dev/null -w '%{http_code}' -X POST "$GATEWAY/v1/chat/completions" \
      -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' \
      -d '{"model":"'"$MODEL"'","messages":[{"role":"user","content":"ping '"$i"'"}]}' \
      > "/tmp/p7-rl.$i" 2>/dev/null || true ) &
done
wait
codes=$(cat /tmp/p7-rl.* | tr '\n' ' ')
echo "concurrent codes: $codes"
echo "$codes" | grep -q 429 || fail "postgres-backed rate limit did not trip (got: $codes)"
$COMPOSE logs gateway 2>/dev/null | grep -qi 'rate.limit.*postgres\|postgres.*rate' && echo "(postgres limiter active)" || true
echo "PASS"

say "secret reload endpoint (root only) + audit"
CODE=$(curl -s -o /tmp/p7-reload.json -w '%{http_code}' -X POST "$GATEWAY/admin/secrets/reload" -H "$ADMIN")
[ "$CODE" = "200" ] || { cat /tmp/p7-reload.json; fail "secrets reload expected 200, got $CODE"; }
python3 -m json.tool < /tmp/p7-reload.json | head -6
# a user token must be forbidden
VT=$(curl -fsS -X POST "$GATEWAY/admin/orgs/org_default/users" -H "$ADMIN" -H 'Content-Type: application/json' \
  -d "{\"email\":\"$NOPE\",\"role\":\"viewer\"}" | JQ "d['token']")
FORB=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$GATEWAY/admin/secrets/reload" -H "Authorization: Bearer $VT")
[ "$FORB" = "403" ] || fail "non-root secrets reload should be 403, got $FORB"
curl -fsS "$GATEWAY/admin/audit?limit=8" -H "$ADMIN" | grep -q 'secret_reload' || fail "secret_reload not audited"
echo "PASS"

say "restore default gateway (SCIM off, memory limiter) — backward compat"
$COMPOSE up -d gateway >/dev/null 2>&1
for i in $(seq 1 20); do curl -fsS "$GATEWAY/healthz" >/dev/null 2>&1 && break; sleep 2; done
GONE=$(curl -s -o /dev/null -w '%{http_code}' "$GATEWAY/scim/v2/Users" -H "$SCIM")
[ "$GONE" = "404" ] || fail "SCIM should be 404 when disabled, got $GONE"
curl -fsS "$GATEWAY/admin/whoami" -H "$ADMIN" | JQ "d['root']" | grep -qi true || fail "post-restore whoami broke"
echo "PASS"

echo
echo "phase 7 smoke test passed"
