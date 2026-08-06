#!/usr/bin/env bash
# Run a Go command across every Go module in this repo.
#
#   scripts/go.sh                 # go vet ./...   (the default)
#   scripts/go.sh test ./...
#   scripts/go.sh build ./...
#
# Uses the local toolchain when `go` is on PATH — which is how CI runs it, via
# actions/setup-go. Otherwise it falls back to the pinned toolchain image, so a
# contributor without Go installed still gets the result CI gets. The gateway
# is where authz, rate limiting and the guardrail screener live; needing a
# host-wide Go install to test it is how that plane ends up only ever being
# checked after a push.
#
# GO_VERSION must stay in step with .github/workflows/ci.yml.
set -euo pipefail

GO_VERSION="${GO_VERSION:-1.25}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Every Go module in the repo. A module runs only when it actually has a
# go.mod, so a not-yet-landed connector is skipped rather than failing.
MODULES="gateway connectors/sql connectors/rest connectors/ssh connectors/soap deploy/demo-crm"

[ "$#" -eq 0 ] && set -- vet ./...

if command -v go >/dev/null 2>&1; then
  run_in() { local mod="$1"; shift; (cd "$REPO_ROOT/$mod" && go "$@"); }
  echo "toolchain: $(go version)"
else
  command -v docker >/dev/null 2>&1 || {
    echo "need either go or docker on PATH" >&2
    exit 127
  }
  # A host directory, not a named volume: it is created by the invoking user,
  # so the container (run with that same uid) can write to it. A named volume
  # would be root-owned and unwritable for a non-root container user.
  CACHE="${GOCACHE_DIR:-$HOME/.cache/agentos-go}"
  mkdir -p "$CACHE"
  run_in() {
    local mod="$1"
    shift
    docker run --rm \
      -u "$(id -u):$(id -g)" \
      -v "$REPO_ROOT:/src" \
      -v "$CACHE:/gocache" \
      -w "/src/$mod" \
      `# Share the host network so a DSN pointing at localhost means the same` \
      `# thing here as it does on the native path. Without this the DB-backed` \
      `# suites silently t.Skip in the fallback and report ok.` \
      --network host \
      -e AGENTOS_TEST_DATABASE_URL \
      -e HOME=/gocache \
      -e GOMODCACHE=/gocache/mod \
      -e GOCACHE=/gocache/build \
      -e GOFLAGS=-buildvcs=false \
      "golang:${GO_VERSION}" go "$@"
  }
  # Debian-based rather than -alpine: the race detector needs cgo and a C
  # toolchain, and alpine's image ships neither. CI runs `test -race`, so an
  # alpine fallback would make the same command pass there and fail here.
  echo "toolchain: golang:${GO_VERSION} (no local go; cache: $CACHE)"
fi

failed=0
for d in $MODULES; do
  if [ -f "$REPO_ROOT/$d/go.mod" ]; then
    echo "==> $d: go $*"
    run_in "$d" "$@" || failed=1
  else
    echo "==> $d: skipped (no go.mod)"
  fi
done
exit "$failed"
