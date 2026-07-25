#!/usr/bin/env bash
# Operators (native autonomy) end-to-end smoke: create operators, fire them
# manually and by webhook, confirm runs go through the governed agent, and check
# the skills surface. Requires the stack up (make up).
set -euo pipefail

export RUNTIME=${RUNTIME:-http://localhost:18000}
RUNTIME_AUTH_TOKEN=${AGENTOS_RUNTIME_AUTH_TOKEN:-runtime-local-dev}
RA=(-H "Authorization: Bearer $RUNTIME_AUTH_TOKEN")

say()  { printf '\n\033[1m== %s\033[0m\n' "$*"; }
pass() { printf '\033[32mPASS\033[0m %s\n' "$*"; }
fail() { printf '\033[31mFAIL\033[0m %s\n' "$*"; exit 1; }
py()   { python3 -c "$1"; }

# ---------------------------------------------------------------------------
say "1. create an interval operator"
OP=$(curl -fsS "${RA[@]}" -H 'Content-Type: application/json' -X POST "$RUNTIME/operators" \
  -d '{"name":"smoke8-'"$$"'","goal":"How many orders are pending? Answer with just the number.","trigger":{"type":"interval","interval_s":300}}')
OID=$(printf '%s' "$OP" | py "import json,sys; print(json.load(sys.stdin)['id'])")
[ -n "$OID" ] || fail "no operator id returned"
pass "operator $OID created (interval)"

# ---------------------------------------------------------------------------
say "2. run it now — the governed agent answers"
RUN=$(curl -fsS -m 300 "${RA[@]}" -X POST "$RUNTIME/operators/$OID/run")
STATUS=$(printf '%s' "$RUN" | py "import json,sys; print(json.load(sys.stdin)['run']['status'])")
OUTPUT=$(printf '%s' "$RUN" | py "import json,sys; print(json.load(sys.stdin)['run']['output'].strip()[:40])")
CYCLES=$(printf '%s' "$RUN" | py "import json,sys; print(json.load(sys.stdin)['run']['cycles'])")
[ "$STATUS" = "completed" ] || fail "run status = $STATUS (want completed)"
[ -n "$OUTPUT" ] || fail "run produced no output"
pass "manual run completed: '$OUTPUT' ($CYCLES cycles)"

# ---------------------------------------------------------------------------
say "3. run history is recorded"
RUNS=$(curl -fsS "${RA[@]}" "$RUNTIME/operators/$OID/runs")
N=$(printf '%s' "$RUNS" | py "import json,sys; print(len(json.load(sys.stdin)['runs']))")
[ "${N:-0}" -ge 1 ] || fail "no runs recorded"
pass "$N run(s) recorded for the operator"

# ---------------------------------------------------------------------------
say "4. webhook operator fires by token with NO bearer"
WOP=$(curl -fsS "${RA[@]}" -H 'Content-Type: application/json' -X POST "$RUNTIME/operators" \
  -d '{"name":"smoke8-hook-'"$$"'","goal":"Acknowledge the order in the payload with one sentence.","trigger":{"type":"webhook"}}')
TOK=$(printf '%s' "$WOP" | py "import json,sys; print(json.load(sys.stdin)['trigger']['webhook_token'])")
WOID=$(printf '%s' "$WOP" | py "import json,sys; print(json.load(sys.stdin)['id'])")
[ -n "$TOK" ] || fail "webhook token not returned at creation"
# Fire WITHOUT the runtime bearer — the token is the credential.
WRUN=$(curl -fsS -m 300 -H 'Content-Type: application/json' \
  -X POST "$RUNTIME/operators/webhooks/$TOK" -d '{"order_id":4242}')
WSRC=$(printf '%s' "$WRUN" | py "import json,sys; print(json.load(sys.stdin)['run']['trigger_source'])")
[ "$WSRC" = "webhook" ] || fail "webhook run source = $WSRC (want webhook)"
pass "webhook fired by token with no bearer (source=webhook)"

# ---------------------------------------------------------------------------
say "5. an unknown webhook token is a 404, never a hint"
CODE=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$RUNTIME/operators/webhooks/whk-nonexistent" -d '{}')
[ "$CODE" = "404" ] || fail "unknown webhook returned $CODE (want 404)"
pass "unknown webhook token -> 404"

# ---------------------------------------------------------------------------
say "6. the listing never leaks a webhook token"
LEAK=$(curl -fsS "${RA[@]}" "$RUNTIME/operators" \
  | py "import json,sys; d=json.load(sys.stdin); print(any((o['trigger'].get('webhook_token')) for o in d['operators']))")
[ "$LEAK" = "False" ] || fail "a webhook token leaked in the operators listing"
pass "no webhook token exposed in the listing"

# ---------------------------------------------------------------------------
say "7. skills are loaded from the in-repo directory"
SKILLS=$(curl -fsS "${RA[@]}" "$RUNTIME/operators/skills")
HAS_ERP=$(printf '%s' "$SKILLS" | py "import json,sys; d=json.load(sys.stdin); print(any(s['name']=='erp-analysis' for s in d['skills']))")
HAS_HASH=$(printf '%s' "$SKILLS" | py "import json,sys; d=json.load(sys.stdin); print(all(len(s['sha256'])==64 for s in d['skills']) and len(d['skills'])>0)")
[ "$HAS_ERP" = "True" ] || fail "the shipped erp-analysis skill is not loaded"
[ "$HAS_HASH" = "True" ] || fail "skills are missing their provenance sha256"
pass "skills loaded with provenance hashes (incl. erp-analysis)"

# ---------------------------------------------------------------------------
say "8. pause halts, delete cleans up"
curl -fsS "${RA[@]}" -H 'Content-Type: application/json' \
  -X PATCH "$RUNTIME/operators/$OID" -d '{"enabled":false}' >/dev/null
ENABLED=$(curl -fsS "${RA[@]}" "$RUNTIME/operators/$OID" \
  | py "import json,sys; print(json.load(sys.stdin)['operator']['enabled'])")
[ "$ENABLED" = "False" ] || fail "pause did not disable the operator"
for id in "$OID" "$WOID"; do
  DC=$(curl -s -o /dev/null -w '%{http_code}' "${RA[@]}" -X DELETE "$RUNTIME/operators/$id")
  [ "$DC" = "204" ] || fail "delete of $id returned $DC (want 204)"
done
pass "operator paused, and both smoke operators deleted"

printf '\n\033[1;32mAll operator smoke checks passed.\033[0m\n'
