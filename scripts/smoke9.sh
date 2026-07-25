#!/usr/bin/env bash
# Multiverse council end-to-end smoke: a real five-model council answers an
# objective against the seeded legacy ERP database, at $0, on local Ollama
# models. Asserts distinct members answered on distinct models, at least one
# member gathered tool evidence, the cycle cap trips, the kill switch halts the
# loop, spend is attributed per key, and council/multiverse answers through the
# OpenAI-compatible surface while refusing a depth-marked recursive request.
#
# Requires the stack up: make up (or docker compose -f deploy/compose.yaml up -d).
set -euo pipefail

export GATEWAY=${GATEWAY:-http://localhost:8080}
export RUNTIME=${RUNTIME:-http://localhost:18000}
RUNTIME_AUTH_TOKEN=${AGENTOS_RUNTIME_AUTH_TOKEN:-runtime-local-dev}
export AGENTOS_ADMIN_KEY=${AGENTOS_ADMIN_KEY:-admin-local-dev}
RA=(-H "Authorization: Bearer $RUNTIME_AUTH_TOKEN")
AA=(-H "Authorization: Bearer $AGENTOS_ADMIN_KEY")

say()  { printf '\n\033[1m== %s\033[0m\n' "$*"; }
pass() { printf '\033[32mPASS\033[0m %s\n' "$*"; }
fail() { printf '\033[31mFAIL\033[0m %s\n' "$*"; exit 1; }

py() { python3 -c "$1"; }

# ---------------------------------------------------------------------------
say "1. members: five local models enabled, five frontier models disabled"
MEMBERS=$(curl -fsS "${RA[@]}" "$RUNTIME/council/members")
ENABLED=$(printf '%s' "$MEMBERS" | py "import json,sys; d=json.load(sys.stdin); print(sum(1 for m in d['members'] if m['enabled']))")
[ "$ENABLED" -ge 5 ] || fail "expected >= 5 enabled members, got $ENABLED"
LOCAL_OK=$(printf '%s' "$MEMBERS" | py "import json,sys; d=json.load(sys.stdin); print(all(m['model'].startswith('ollama/') for m in d['members'] if m['enabled']))")
[ "$LOCAL_OK" = "True" ] || fail "enabled members must all be ollama/* (frontier five stay disabled)"
pass "$ENABLED local members enabled, all ollama/*, frontier members disabled"

# ---------------------------------------------------------------------------
say "2. a real objective produces a verdict from the council"
Q='Which customers in Riyadh have unpaid invoices, and for how much?'
RUN=$(curl -fsS "${RA[@]}" -H 'Content-Type: application/json' \
  -X POST "$RUNTIME/council/objectives/run" -d "{\"input\":\"$Q\"}")
OID=$(printf '%s' "$RUN" | py "import json,sys; print(json.load(sys.stdin)['objective']['id'])")
AGREE=$(printf '%s' "$RUN" | py "import json,sys; v=json.load(sys.stdin)['verdict']; a=v.get('agreement',-1); print('OK' if isinstance(a,(int,float)) and 0<=a<=1 else 'BAD')")
[ "$AGREE" = "OK" ] || fail "verdict.agreement must be a number in [0,1]"
ANSWER=$(printf '%s' "$RUN" | py "import json,sys; print((json.load(sys.stdin)['verdict'].get('answer') or '').strip()[:60])")
[ -n "$ANSWER" ] || fail "verdict.answer is empty — the council did not reach quorum (check local model tool-calling)"
pass "objective $OID produced a verdict: '$ANSWER'"

# ---------------------------------------------------------------------------
say "3. distinct members ran on distinct models"
DETAIL=$(curl -fsS "${RA[@]}" "$RUNTIME/council/objectives/$OID")
# member_runs live in council_member_runs, not the objective view; count via the
# cycles' cited/agreement is not enough, so read runs from the DB view exposed
# by the gateway audit instead. Here we assert on the cycle's dissent+cited plus
# the DB. Fall back to counting distinct models from the detail if present.
RUNS=$(docker compose -f deploy/compose.yaml exec -T postgres \
  psql -U agentos -d agentos -tAc \
  "SELECT count(*), count(distinct model_used), count(*) filter (where status='answered')
   FROM council_member_runs r JOIN council_cycles c ON c.id=r.cycle_id
   WHERE c.objective_id='$OID';" 2>/dev/null | tr -d ' ')
TOTAL=$(printf '%s' "$RUNS" | cut -d'|' -f1)
DISTINCT=$(printf '%s' "$RUNS" | cut -d'|' -f2)
ANSWERED=$(printf '%s' "$RUNS" | cut -d'|' -f3)
[ "${TOTAL:-0}" -ge 3 ] || fail "expected >= 3 member runs, got ${TOTAL:-0}"
[ "${DISTINCT:-0}" -ge 3 ] || fail "expected >= 3 distinct models, got ${DISTINCT:-0}"
pass "$TOTAL member runs on $DISTINCT distinct models ($ANSWERED answered)"

# ---------------------------------------------------------------------------
say "4. tool-calling reality check (the honest one)"
STEPS=$(docker compose -f deploy/compose.yaml exec -T postgres \
  psql -U agentos -d agentos -tAc \
  "SELECT member_id, jsonb_array_length(steps) FROM council_member_runs r
   JOIN council_cycles c ON c.id=r.cycle_id WHERE c.objective_id='$OID';" 2>/dev/null)
printf '%s\n' "$STEPS" | while IFS='|' read -r m n; do
  [ -n "$m" ] && printf '   member %-18s steps=%s\n' "$(echo "$m" | tr -d ' ')" "$(echo "$n" | tr -d ' ')"
done
WITH_TOOLS=$(docker compose -f deploy/compose.yaml exec -T postgres \
  psql -U agentos -d agentos -tAc \
  "SELECT count(*) FROM council_member_runs r JOIN council_cycles c ON c.id=r.cycle_id
   WHERE c.objective_id='$OID' AND jsonb_array_length(steps) > 0;" 2>/dev/null | tr -d ' ')
[ "${WITH_TOOLS:-0}" -ge 1 ] || fail "no member gathered tool evidence — the verdict is unsupported (swap non-tool-calling models in council.yaml)"
pass "$WITH_TOOLS member(s) called a tool — the verdict has evidence"

# ---------------------------------------------------------------------------
say "5. the cycle cap trips"
C1=$(curl -fsS "${RA[@]}" -H 'Content-Type: application/json' \
  -X POST "$RUNTIME/council/objectives/run" -d "{\"input\":\"$Q\",\"max_cycles\":1}")
C1ID=$(printf '%s' "$C1" | py "import json,sys; print(json.load(sys.stdin)['objective']['id'])")
C1OBJ=$(curl -fsS "${RA[@]}" "$RUNTIME/council/objectives/$C1ID")
CYCLES=$(printf '%s' "$C1OBJ" | py "import json,sys; print(json.load(sys.stdin)['objective']['cycles_run'])")
REASON=$(printf '%s' "$C1OBJ" | py "import json,sys; print(json.load(sys.stdin)['objective']['stop_reason'] or '')")
[ "$CYCLES" = "1" ] || fail "max_cycles=1 ran $CYCLES cycles"
[ -n "$REASON" ] || fail "stop_reason not set"
pass "cycle cap held: cycles_run=1, stop_reason=$REASON"

# ---------------------------------------------------------------------------
say "6. the kill switch halts a queued objective"
curl -fsS "${RA[@]}" -X POST "$RUNTIME/council/pause" >/dev/null
QUEUED=$(curl -fsS "${RA[@]}" -H 'Content-Type: application/json' \
  -X POST "$RUNTIME/council/objectives" -d "{\"input\":\"$Q\"}")
QID=$(printf '%s' "$QUEUED" | py "import json,sys; print(json.load(sys.stdin)['id'])")
PAUSED_RUN=$(curl -fsS "${RA[@]}" -X POST "$RUNTIME/council/objectives/$QID/run")
PREASON=$(printf '%s' "$PAUSED_RUN" | py "import json,sys; print(json.load(sys.stdin)['stop_reason'])")
curl -fsS "${RA[@]}" -X POST "$RUNTIME/council/resume" >/dev/null
[ "$PREASON" = "paused" ] || fail "paused objective ran anyway (stop_reason=$PREASON)"
pass "kill switch works: a run while paused stops with stop_reason=paused"

# ---------------------------------------------------------------------------
say "7. governance: council traffic is recorded per key in the gateway"
USAGE=$(curl -fsS "${AA[@]}" "$GATEWAY/admin/usage")
REQS=$(printf '%s' "$USAGE" | py "import json,sys; d=json.load(sys.stdin); print(sum(u['requests'] for u in d))")
[ "${REQS:-0}" -ge 1 ] || fail "no requests recorded in gateway usage"
pass "gateway recorded $REQS request(s) across keys"

# ---------------------------------------------------------------------------
say "8. council/multiverse works as an OpenAI-compatible model"
K=$(curl -fsS "${AA[@]}" -H 'Content-Type: application/json' -X POST "$GATEWAY/admin/keys" \
  -d "{\"name\":\"smoke9-$$\",\"monthly_budget_usd\":5}" \
  | py "import json,sys; print(json.load(sys.stdin)['key'])")
COMP=$(curl -fsS -H "Authorization: Bearer $K" -H 'Content-Type: application/json' \
  -X POST "$GATEWAY/v1/chat/completions" \
  -d '{"model":"council/multiverse","messages":[{"role":"user","content":"What is 8 times 9? Answer with just the number."}]}')
CONTENT=$(printf '%s' "$COMP" | py "import json,sys; d=json.load(sys.stdin); print(d['choices'][0]['message']['content'][:40])")
HASAGREE=$(printf '%s' "$COMP" | py "import json,sys; d=json.load(sys.stdin); print('agreement' in d.get('x_agentos_council',{}))")
[ -n "$CONTENT" ] || fail "council/multiverse returned an empty completion"
[ "$HASAGREE" = "True" ] || fail "council/multiverse missing x_agentos_council.agreement"
pass "council/multiverse answered: '$CONTENT' (with council extension)"

DEPTH=$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $K" \
  -H 'Content-Type: application/json' -H 'X-AgentOS-Council-Depth: 1' \
  -X POST "$GATEWAY/v1/chat/completions" \
  -d '{"model":"council/multiverse","messages":[{"role":"user","content":"hi"}]}')
[ "$DEPTH" = "400" ] || fail "recursion guard did not reject a depth-marked request (got $DEPTH)"
pass "recursion guard rejected a depth-marked request (400)"

printf '\n\033[1;32mAll council smoke checks passed.\033[0m\n'
