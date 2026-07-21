# Sandbox egress isolation — verification record (Phase 4, Task 5)

Date: 2026-07-21. Topology change: `sandbox` moved to a dedicated
`internal: true` Docker network (`sandbox-net`); `runtime` attached to both
`default` and `sandbox-net`; the sandbox `ports:` host publish was **removed**
(Docker forbids publishing ports from an internal network — and publishing
would be pointless anyway once the goal is no route beyond the runtime).

## Topology proof

```console
$ docker inspect agentos-sandbox-1 --format '{{range $k,$v := .NetworkSettings.Networks}}{{$k}} {{end}}'
agentos_sandbox-net
$ docker inspect agentos-runtime-1 --format '{{range $k,$v := .NetworkSettings.Networks}}{{$k}} {{end}}'
agentos_default agentos_sandbox-net
$ docker network inspect agentos_sandbox-net --format 'internal={{.Internal}}'
internal=true
```

## 1. runtime → sandbox still works (internal network route)

```console
$ docker compose -f compose.yaml --env-file .env exec -T runtime \
    python -c "import urllib.request; print('healthz:', urllib.request.urlopen('http://sandbox:8070/healthz', timeout=5).read().decode())"
healthz: ok
```

## 2. sandbox → internet FAILS (egress-less)

Executed via the sandbox's own `/execute` endpoint (called from the runtime,
payload `{"language":"python","code":...,"timeout_s":15}`), i.e. this is
untrusted-code-eye-view:

```python
import socket
socket.create_connection(("1.1.1.1", 53), timeout=3)   # raw TCP egress
urllib.request.urlopen("http://example.com", timeout=3) # HTTP + DNS egress
```

Result:

```console
exit_code: 0
OUTBOUND-FAILED: OSError [Errno 101] Network is unreachable
HTTP-FAILED: URLError <urlopen error [Errno -3] Temporary failure in name resolution>
```

## 3. Collateral checks

```console
$ curl -s -m 3 http://localhost:8070/healthz || echo unreachable
host->sandbox:8070 unreachable (expected — host publish removed)
$ docker compose ... exec -T runtime python -c "...urlopen('http://gateway:8080/healthz')..."
runtime->gateway healthz: 200          # runtime kept its default-network reach
$ curl -s http://localhost:18000/healthz
{"status":"ok"}                        # runtime API unaffected
```

## Note for the orchestrator (smoke3 / smoke4 impact)

`scripts/smoke3.sh` (owned by the orchestrator — deliberately **not** edited
here) can no longer reach the sandbox at `host:${AGENTOS_SANDBOX_PORT:-8070}`:
the host port publish is gone and cannot come back while `sandbox-net` is
`internal: true`. Any sandbox check must instead go through the runtime
container, e.g.:

```bash
docker compose -f deploy/compose.yaml --env-file deploy/.env exec -T runtime \
  python -c "import urllib.request; print(urllib.request.urlopen('http://sandbox:8070/healthz', timeout=5).read().decode())"
```

`AGENTOS_SANDBOX_PORT` in `.env`/`.env.example` is now unused and can be
retired. smoke4's "sandbox egress denied" assertion can reuse the section-2
`/execute` payload above verbatim.
