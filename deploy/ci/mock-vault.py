#!/usr/bin/env python3
"""Deterministic, offline, minimal HashiCorp Vault KV v2 mock for CI/smoke.

This is a *stdlib-only* mock (``http.server`` + ``json`` — no third-party deps,
nothing to install) that stands in for a real Vault server so the AgentOS
gateway's ``AGENTOS_SECRETS_BACKEND=vault`` code path can be exercised in
CI/smoke against nothing but ``python3``.

The gateway's Vault backend does exactly one thing at startup: it issues
``GET {AGENTOS_VAULT_ADDR}/v1/{AGENTOS_VAULT_KV_PATH}`` with an
``X-Vault-Token`` header and parses the KV v2 response shape

    {"data": {"data": {NAME: VALUE, ...}, "metadata": {...}}}

into its secret map. This mock replies with a canned KV v2 payload:

    GET /v1/<any path>   with correct X-Vault-Token  -> 200 canned secrets
    GET /v1/<any path>   with wrong/missing token    -> 403 {"errors":["permission denied"]}

Path handling (documented deviation-free choice)
------------------------------------------------
Any path under ``/v1/`` returns the SAME canned data as long as the token is
valid — the mock does not model multiple secret paths. This keeps it tiny and
means smoke6 works with whatever ``AGENTOS_VAULT_KV_PATH`` the gateway is
configured with (e.g. ``secret/data/agentos``). A request that is not under
``/v1/`` returns 404. Token check happens BEFORE path check, matching Vault's
precedence (a bad token is always 403).

Env / argv
----------
  argv[1] (optional)            port to bind (default 8200 or $AGENTOS_MOCK_VAULT_PORT)
  $AGENTOS_MOCK_VAULT_TOKEN     the token that must be presented in X-Vault-Token
                                (default "test-token"). Set the gateway's
                                AGENTOS_VAULT_TOKEN to the same value.
  --selftest                    run an in-process self-check (right token -> data,
                                wrong token -> 403) and exit.

How smoke6 uses it
------------------
smoke6 starts this on a network alias (e.g. ``vault:8200``) and configures the
gateway with AGENTOS_SECRETS_BACKEND=vault, AGENTOS_VAULT_ADDR=http://vault:8200,
AGENTOS_VAULT_TOKEN=<this token>, AGENTOS_VAULT_KV_PATH=secret/data/agentos. The
gateway then serves provider API keys from the canned secrets below, proving the
Vault backend end-to-end.
"""

from __future__ import annotations

import argparse
import json
import os
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any

# --- canned KV v2 secret payload (determinism) ------------------------------
#
# These are the secret NAME->VALUE pairs the gateway will load. The values are
# obvious fakes; nothing real is stored here.
CANNED_SECRETS = {
    "AGENTOS_ANTHROPIC_API_KEY": "sk-vault-demo",
    "AGENTOS_OPENAI_API_KEY": "sk-vault-demo-openai",
}

CANNED_METADATA = {
    "created_time": "2026-07-21T00:00:00Z",
    "custom_metadata": None,
    "deletion_time": "",
    "destroyed": False,
    "version": 1,
}


def kv_v2_body() -> dict[str, Any]:
    """The KV v2 read response shape the gateway parses."""
    return {
        "request_id": "mock-vault-request",
        "lease_id": "",
        "renewable": False,
        "lease_duration": 0,
        "data": {
            "data": dict(CANNED_SECRETS),
            "metadata": dict(CANNED_METADATA),
        },
        "wrap_info": None,
        "warnings": None,
        "auth": None,
    }


class VaultState:
    def __init__(self, token: str) -> None:
        self.token = token


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"
    state: VaultState  # set on the class before serving

    def log_message(self, fmt: str, *args: Any) -> None:
        sys.stderr.write("mock-vault: " + (fmt % args) + "\n")

    def _send_json(self, obj: dict[str, Any], status: int = 200) -> None:
        payload = json.dumps(obj).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Cache-Control", "no-store")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def do_GET(self) -> None:
        path = self.path.split("?", 1)[0]

        # Health endpoints (unauthenticated) — handy for smoke readiness probes.
        if path.rstrip("/") in ("/v1/sys/health", "/healthz", "/health"):
            self._send_json({"initialized": True, "sealed": False, "standby": False})
            return

        # Token check first (Vault precedence: a bad token is always 403).
        token = self.headers.get("X-Vault-Token", "")
        if token != self.state.token:
            self._send_json({"errors": ["permission denied"]}, 403)
            return

        # Any path under /v1/ returns the canned secrets; everything else 404.
        if path.startswith("/v1/"):
            self._send_json(kv_v2_body())
            return
        self._send_json({"errors": ["not found"]}, 404)


def build_state() -> VaultState:
    token = os.environ.get("AGENTOS_MOCK_VAULT_TOKEN", "test-token")
    return VaultState(token)


def serve(port: int, token: str | None = None) -> ThreadingHTTPServer:
    Handler.state = VaultState(token) if token is not None else build_state()
    return ThreadingHTTPServer(("0.0.0.0", port), Handler)


def main() -> None:
    parser = argparse.ArgumentParser(description="Minimal stdlib Vault KV v2 mock")
    parser.add_argument("port", nargs="?",
                        type=int,
                        default=int(os.environ.get("AGENTOS_MOCK_VAULT_PORT", "8200")),
                        help="port to bind (default 8200 or $AGENTOS_MOCK_VAULT_PORT)")
    parser.add_argument("--selftest", action="store_true",
                        help="run an in-process self-check and exit")
    args = parser.parse_args()
    if args.selftest:
        sys.exit(0 if selftest() else 1)
    server = serve(args.port)
    sys.stderr.write(
        f"mock-vault: listening on http://0.0.0.0:{args.port} "
        f"(token={Handler.state.token})\n"
    )
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()


# --- self-test --------------------------------------------------------------

def selftest() -> bool:
    """Start the server; assert right token -> canned data, wrong token -> 403."""
    import http.client
    import threading

    ok = True

    def check(cond: bool, label: str) -> None:
        nonlocal ok
        status = "PASS" if cond else "FAIL"
        if not cond:
            ok = False
        print(f"  [{status}] {label}")

    port = 18200
    token = "test-token"
    server = serve(port, token=token)
    t = threading.Thread(target=server.serve_forever, daemon=True)
    t.start()
    try:
        # 1) correct token -> 200 with the KV v2 nested data shape
        conn = http.client.HTTPConnection("127.0.0.1", port)
        conn.request("GET", "/v1/secret/data/agentos", headers={"X-Vault-Token": token})
        resp = conn.getresponse()
        raw = resp.read()
        conn.close()
        check(resp.status == 200, "correct token -> 200")
        body = json.loads(raw)
        data = body.get("data", {}).get("data", {})
        check(data.get("AGENTOS_ANTHROPIC_API_KEY") == "sk-vault-demo",
              "canned AGENTOS_ANTHROPIC_API_KEY present")
        check(data.get("AGENTOS_OPENAI_API_KEY") == "sk-vault-demo-openai",
              "canned AGENTOS_OPENAI_API_KEY present")
        check("metadata" in body.get("data", {}), "KV v2 metadata present")

        # 2) wrong token -> 403 permission denied
        conn = http.client.HTTPConnection("127.0.0.1", port)
        conn.request("GET", "/v1/secret/data/agentos", headers={"X-Vault-Token": "nope"})
        resp = conn.getresponse()
        raw = resp.read()
        conn.close()
        check(resp.status == 403, "wrong token -> 403")
        check(json.loads(raw).get("errors") == ["permission denied"],
              "403 body is permission denied")

        # 3) missing token -> 403
        conn = http.client.HTTPConnection("127.0.0.1", port)
        conn.request("GET", "/v1/secret/data/agentos")
        resp = conn.getresponse()
        resp.read()
        conn.close()
        check(resp.status == 403, "missing token -> 403")
    finally:
        server.shutdown()
        server.server_close()

    print("mock-vault selftest:", "PASS" if ok else "FAIL")
    return ok


if __name__ == "__main__":
    main()
