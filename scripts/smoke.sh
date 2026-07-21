#!/usr/bin/env bash
# End-to-end smoke test for the Phase 1 core loop.
# Requires the compose stack up (make up) and a provider key in deploy/.env.
set -euo pipefail

GATEWAY=${GATEWAY:-http://localhost:8080}
RUNTIME=${RUNTIME:-http://localhost:8000}
ADMIN_KEY=${AGENTOS_ADMIN_KEY:-admin-local-dev}

say() { printf '\n\033[1m== %s\033[0m\n' "$*"; }

say "gateway health"
curl -fsS "$GATEWAY/healthz"

say "runtime health"
curl -fsS "$RUNTIME/healthz"

say "asking the agent a question that requires the legacy ERP"
RESP=$(curl -fsS -X POST "$RUNTIME/runs" \
  -H 'Content-Type: application/json' \
  -d '{"input": "Which customer has the highest total order value across all their orders? Give the name and the total."}')
echo "$RESP" | python3 -m json.tool

echo "$RESP" | grep -qi "al-faisal" \
  && echo "PASS: agent found the top customer via the SQL connector" \
  || { echo "FAIL: expected answer to mention Al-Faisal Trading Co."; exit 1; }

say "usage accounting on the gateway"
curl -fsS "$GATEWAY/admin/usage" -H "Authorization: Bearer $ADMIN_KEY" | python3 -m json.tool

echo
echo "smoke test passed"
