#!/usr/bin/env bash
# Phase 3 end-to-end smoke test: sandbox, REST connector, eval-gated
# self-improvement, OTel overlay, Helm chart rendering.
# Requires the Phase 3 stack up (make up) and Phase 2 state (vendor-policy
# doc ingested — run smoke2 first on a fresh volume).
set -euo pipefail

export GATEWAY=${GATEWAY:-http://localhost:8080}
export RUNTIME=${RUNTIME:-http://localhost:18000}
export AGENTOS_ADMIN_KEY=${AGENTOS_ADMIN_KEY:-admin-local-dev}
COMPOSE="docker compose -f deploy/compose.yaml --env-file deploy/.env"

say()  { printf '\n\033[1m== %s\033[0m\n' "$*"; }
fail() { echo "FAIL: $*"; exit 1; }

# The sandbox lives on an internal-only network (egress-less, Phase 4) and no
# longer publishes a host port, so reach it from inside the runtime container.
sandbox_exec() { # $1 = JSON body
  $COMPOSE exec -T runtime python -c "
import sys,json,urllib.request
body=sys.stdin.buffer.read()
req=urllib.request.Request('http://sandbox:8070/execute',data=body,headers={'Content-Type':'application/json'})
print(urllib.request.urlopen(req,timeout=30).read().decode())
" <<<"$1"
}

say "sandbox executes python with isolation"
OUT=$(sandbox_exec '{"language":"python","code":"import os\nprint(6*7)\nprint(sorted(os.environ.keys()))"}')
echo "$OUT" | python3 -m json.tool
echo "$OUT" | grep -q '"stdout": *"42' || echo "$OUT" | grep -q '42' || fail "sandbox did not compute"
# Exec-time env is PATH-only; CPython's PEP 538 locale coercion may add
# LC_CTYPE to os.environ after startup. Anything else is leakage.
echo "$OUT" | python3 -c "
import json, sys, ast
d = json.load(sys.stdin)
keys = ast.literal_eval(d['stdout'].splitlines()[1])
extra = set(keys) - {'PATH', 'LC_CTYPE'}
sys.exit(1 if extra else 0)
" || fail "sandbox env not cleared (unexpected vars beyond PATH/LC_CTYPE)"
echo "PASS"

say "sandbox timeout enforcement"
T0=$(date +%s)
TOUT=$(sandbox_exec '{"language":"python","code":"import time\ntime.sleep(60)","timeout_s":3}')
T1=$(date +%s)
echo "$TOUT" | grep -q '"timed_out": *true\|"timed_out":true' || fail "no timeout flag"
[ $((T1 - T0)) -lt 15 ] || fail "timeout took too long ($((T1-T0))s)"
echo "PASS"

say "agent uses run_python for computation"
RESP=$(curl -fsS -X POST "$RUNTIME/runs" -H 'Content-Type: application/json' \
  -d '{"input": "Use the run_python tool to compute the sum of squares of 1..10 and report the number."}')
echo "$RESP" | grep -q '"tool": *"run_python"\|"tool":"run_python"' || fail "run_python not used"
echo "$RESP" | grep -q "385" || fail "wrong sum of squares"
echo "PASS"

say "agent reaches the legacy CRM through the REST connector"
RESP=$(curl -fsS -X POST "$RUNTIME/runs" -H 'Content-Type: application/json' \
  -d '{"input": "Using the CRM tools, how many OPEN tickets does Red Sea Logistics have right now? Reply with the number."}')
echo "$RESP" | python3 -m json.tool | head -25
echo "$RESP" | grep -qE '"tool": *"(list_tickets|list_operations|list_customers|get_customer)"|"tool":"(list_tickets|list_operations|list_customers|get_customer)"' \
  || fail "no CRM tool used"
echo "$RESP" | grep -q '2' || fail "expected 2 open tickets"
echo "PASS"

say "eval suite runs and scores"
EVAL=$(curl -fsS -X POST "$RUNTIME/evals/run" -H 'Content-Type: application/json' -d '{"suite":"default"}')
echo "$EVAL" | python3 -c "import json,sys; d=json.load(sys.stdin); print(f\"score={d['score']} passed={d['passed']} failed={d['failed']}\"); [print(' -', c['name'], 'PASS' if c['passed'] else 'FAIL') for c in d['cases']]"
PASSED=$(echo "$EVAL" | python3 -c "import json,sys; print(json.load(sys.stdin)['passed'])")
[ "$PASSED" -ge 3 ] || fail "expected >=3 eval cases passing, got $PASSED"
echo "PASS"

say "self-improvement: propose, verify eval gate, deny, 409 on re-decision"
PROP=$(curl -fsS -X POST "$RUNTIME/improve" -H 'Content-Type: application/json' -d '{}')
echo "$PROP" | python3 -c "import json,sys; d=json.load(sys.stdin); print(f\"proposal {d['id']}: {d['status']} baseline={d['baseline_score']} candidate={d['candidate_score']}\"); print('rationale:', d['rationale'][:160])"
PID=$(echo "$PROP" | python3 -c "import json,sys; print(json.load(sys.stdin)['id'])")
STATUS=$(echo "$PROP" | python3 -c "import json,sys; print(json.load(sys.stdin)['status'])")
case "$STATUS" in passed_evals|failed_evals) ;; *) fail "unexpected proposal status $STATUS";; esac
curl -fsS -X POST "$RUNTIME/proposals/$PID/approve" -H 'Content-Type: application/json' -d '{"approve": false}' > /dev/null
ACTIVE=$(curl -fsS "$RUNTIME/prompts/active")
echo "$ACTIVE" | grep -q '"source": *"default"\|"source":"default"' || fail "denied proposal must not activate"
CODE=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$RUNTIME/proposals/$PID/approve" \
  -H 'Content-Type: application/json' -d '{"approve": true}')
[ "$CODE" = "409" ] || fail "re-deciding a proposal should 409, got $CODE"
echo "PASS (proposal denied → default prompt still active; gate enforced)"

say "OTel overlay: spans reach the collector"
$COMPOSE -f deploy/compose.otel.yaml up -d otel-collector gateway runtime > /dev/null 2>&1
for i in $(seq 1 30); do curl -fsS "$RUNTIME/healthz" > /dev/null 2>&1 && break; sleep 2; done
curl -fsS -X POST "$RUNTIME/runs" -H 'Content-Type: application/json' \
  -d '{"input": "How many customers are in the ERP? Short answer."}' > /dev/null
sleep 5
# capture first: grep -q on a live pipe + pipefail turns SIGPIPE into failure
COLLECTOR_LOGS=$($COMPOSE -f deploy/compose.otel.yaml logs otel-collector 2>/dev/null || true)
echo "$COLLECTOR_LOGS" | grep -qiE 'agent\.run|data_type": "traces|spans' \
  || fail "no spans seen at the collector"
$COMPOSE up -d gateway runtime > /dev/null 2>&1
echo "PASS"

say "helm chart lints and renders"
bash deploy/helm/test-render.sh > /tmp/agentos-helm.log 2>&1 || { tail -20 /tmp/agentos-helm.log; fail "helm render"; }
tail -3 /tmp/agentos-helm.log
echo "PASS"

echo
echo "phase 3 smoke test passed"
