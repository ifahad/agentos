#!/usr/bin/env python3
"""Deterministic, offline, minimal OIDC provider for CI/smoke (stdlib only).

This is a *stdlib-only* mock (``http.server`` + ``hashlib``/``hmac``/``base64``/
``json`` — no third-party deps, nothing to install) that stands in for a real
identity provider so the AgentOS gateway's OIDC single-sign-on flow can be
exercised end-to-end in CI/smoke against nothing but ``python3``.

It implements the four endpoints the gateway's ``coreos/go-oidc`` +
``golang.org/x/oauth2`` client touches:

    GET  /.well-known/openid-configuration  -> discovery document
    GET  /authorize                         -> 302 auto-approve back to redirect_uri
    POST /token                             -> {access_token, id_token(RS256 JWT), ...}
    GET  /jwks                              -> JWKS public key (kid "mock-key-1")

The ID token is a real RS256 JWT (RSASSA-PKCS1-v1_5 over SHA-256), signed with a
FIXED, hardcoded 2048-bit RSA private key embedded below. RS256 is hand-rolled
on Python big-int modular exponentiation (``pow(m, d, n)``) so no ``PyJWT`` /
``cryptography`` is required, yet the signature is a standards-compliant PKCS#1
v1.5 signature that ``go-oidc`` (or any conformant JWKS verifier) accepts.

Env / argv
----------
  argv[1] (optional)          port to bind (default 9000 or $AGENTOS_MOCK_OIDC_PORT)
  $AGENTOS_MOCK_OIDC_ISSUER   PUBLIC base URL the gateway uses to reach this
                              server, e.g. http://mock-oidc:9000. This value is
                              echoed VERBATIM as ``issuer`` and is the base for
                              authorization_endpoint / token_endpoint / jwks_uri
                              in discovery, so it MUST match how the gateway
                              dials us (else go-oidc's issuer check fails).
                              Default: http://localhost:<port>.
  --selftest                  run an in-process end-to-end + signature-verify
                              self-check and exit (see bottom of file).

How smoke6 uses it
------------------
smoke6 starts this on a network alias (e.g. ``mock-oidc:9000``), sets the
gateway's AGENTOS_OIDC_ISSUER to that same URL, AGENTOS_OIDC_CLIENT_ID /
_CLIENT_SECRET to anything (this mock accepts any client), then drives
``/auth/oidc/login`` -> ``/authorize`` (auto-approve) -> gateway
``/auth/oidc/callback`` -> ``/token`` -> ID-token verify via ``/jwks``, proving
the real SSO verification path (not a bypass). The mock always authenticates the
same fixed user: sub ``mock-user-1``, email ``alice@example.com``.
"""

from __future__ import annotations

import argparse
import base64
import hashlib
import hmac
import json
import os
import sys
import time
import urllib.parse
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any

# --- fixed identity + protocol constants (determinism) ----------------------

FIXED_CODE = "mock-auth-code"        # the single authorization code we hand out
KID = "mock-key-1"                   # JWKS key id; echoed in the JWT header
SUB = "mock-user-1"
EMAIL = "alice@example.com"
NAME = "Alice Example"

# --- FIXED RSA private key (2048-bit) ---------------------------------------
#
# Hardcoded so startup needs no (expensive, nondeterministic) key generation and
# every run signs/verifies with the same key. Generated once with OpenSSL; the
# public half (N, E) is published via /jwks. This key is a TEST FIXTURE ONLY --
# it is public in the repo and MUST NOT be used for anything real.

RSA_N = 26629518733758770484551305652643937071856576659006907401759711713948912311727697698262113352799305614765787244052710516305033709305605568559386697238800617014235467333911219269930592808488232661317289778326316053577392096796624691717739811205023614815800034108623146946963556423884809845657900538337823264199638386393233553290057771756487990085110611137221477359745462744704441482570510319026789647093600122559325214868273577344817151597626005685076428750640415603528122703647164457093769586409069250762034710386530165740593433749997169383420725468640246032609235481252805205831084252934217238357039045576995244374517
RSA_E = 65537
RSA_D = 1838160191928260136162717470868146705783000595733261332290573403959380011487721690607159677574071522706226414507130123808097029087085798727963931629341097770769166755279084651020863737534512563198740651471970532275028237696138018745885560053959229079108646835147469057126589590647281672034353099043551827873725081854749011917657778788305708571866222002731911423143268557643303774856118312667162783808664603837288990467629795346844510386477182477112631503057818418406518510090264527784213139541459496233414014181528501230735609016850059412693207421331879804529565801913732717737705117825331534143161250486360535965953
RSA_P = 166595815896396217873967896348555838204415937438415031984612742682815379202490712906978596652064378658823629631727075659853396724302678624817985277288237568905027139785376130212591044581948745797717860952222745684294500830561617929404126561635489379387403867684135937505573463023018698463330116333884315970421
RSA_Q = 159845063277695543051906476093922105787691410288619059469122898399506331917827838680431314420804326956668869263281815177323034864800663831610792701683553559706983801171076241099803632761365576330948191419754853613668507070273186852533770603907661988051256372802290948528293023212141084238764625417164545410177

RSA_KEY_BYTES = (RSA_N.bit_length() + 7) // 8   # 256 for a 2048-bit modulus

# SHA-256 DigestInfo prefix (DER) for EMSA-PKCS1-v1_5 (RFC 8017 sec 9.2).
SHA256_DIGEST_INFO_PREFIX = bytes.fromhex("3031300d060960864801650304020105000420")


# --- base64url helpers ------------------------------------------------------

def b64url(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode("ascii")


def b64url_uint(value: int) -> str:
    """Big-endian, minimal-length base64url of an unsigned integer (JWK n/e)."""
    length = (value.bit_length() + 7) // 8 or 1
    return b64url(value.to_bytes(length, "big"))


def b64url_decode(s: str) -> bytes:
    pad = "=" * (-len(s) % 4)
    return base64.urlsafe_b64decode(s + pad)


# --- RS256: RSASSA-PKCS1-v1_5 sign/verify over SHA-256 ----------------------

def _emsa_pkcs1_v15(message: bytes, em_len: int) -> bytes:
    """EMSA-PKCS1-v1_5 encode `message` for an `em_len`-byte modulus (RFC 8017)."""
    digest = hashlib.sha256(message).digest()
    t = SHA256_DIGEST_INFO_PREFIX + digest          # DigestInfo
    # EM = 0x00 || 0x01 || PS (0xff...) || 0x00 || T ; PS >= 8 bytes.
    ps_len = em_len - len(t) - 3
    if ps_len < 8:
        raise ValueError("modulus too short for RS256")
    return b"\x00\x01" + (b"\xff" * ps_len) + b"\x00" + t


def rs256_sign(signing_input: bytes) -> bytes:
    """Produce the raw RS256 signature bytes for `signing_input`."""
    em = _emsa_pkcs1_v15(signing_input, RSA_KEY_BYTES)
    m = int.from_bytes(em, "big")
    s = pow(m, RSA_D, RSA_N)                          # s = m^d mod n
    return s.to_bytes(RSA_KEY_BYTES, "big")


def rs256_verify(signing_input: bytes, signature: bytes) -> bool:
    """Verify an RS256 signature with the public key (recompute EM)."""
    if len(signature) != RSA_KEY_BYTES:
        return False
    s = int.from_bytes(signature, "big")
    if s >= RSA_N:
        return False
    m = pow(s, RSA_E, RSA_N)                           # m = s^e mod n
    em = m.to_bytes(RSA_KEY_BYTES, "big")
    expected = _emsa_pkcs1_v15(signing_input, RSA_KEY_BYTES)
    return hmac.compare_digest(em, expected)


def make_jwt(claims: dict[str, Any]) -> str:
    """Serialize + RS256-sign a JWT with header {alg:RS256, typ:JWT, kid}."""
    header = {"alg": "RS256", "typ": "JWT", "kid": KID}
    seg_h = b64url(json.dumps(header, separators=(",", ":")).encode("utf-8"))
    seg_p = b64url(json.dumps(claims, separators=(",", ":")).encode("utf-8"))
    signing_input = (seg_h + "." + seg_p).encode("ascii")
    seg_s = b64url(rs256_sign(signing_input))
    return seg_h + "." + seg_p + "." + seg_s


# --- server -----------------------------------------------------------------

class OIDCState:
    """Holds the public issuer base URL (shared with request handlers)."""

    def __init__(self, issuer: str) -> None:
        self.issuer = issuer.rstrip("/")

    def discovery(self) -> dict[str, Any]:
        return {
            "issuer": self.issuer,
            "authorization_endpoint": self.issuer + "/authorize",
            "token_endpoint": self.issuer + "/token",
            "jwks_uri": self.issuer + "/jwks",
            "response_types_supported": ["code"],
            "subject_types_supported": ["public"],
            "id_token_signing_alg_values_supported": ["RS256"],
            "scopes_supported": ["openid", "email", "profile"],
            "token_endpoint_auth_methods_supported": [
                "client_secret_post", "client_secret_basic",
            ],
            "claims_supported": ["sub", "email", "name", "iss", "aud", "iat", "exp"],
        }

    def jwks(self) -> dict[str, Any]:
        return {
            "keys": [
                {
                    "kty": "RSA",
                    "alg": "RS256",
                    "use": "sig",
                    "kid": KID,
                    "n": b64url_uint(RSA_N),
                    "e": b64url_uint(RSA_E),
                }
            ]
        }

    def id_token(self, aud: str) -> str:
        now = int(time.time())
        claims = {
            "iss": self.issuer,
            "aud": aud,
            "sub": SUB,
            "email": EMAIL,
            "email_verified": True,
            "name": NAME,
            "iat": now,
            "exp": now + 3600,
        }
        return make_jwt(claims)


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"
    state: OIDCState  # set on the class before serving

    def log_message(self, fmt: str, *args: Any) -> None:
        sys.stderr.write("mock-oidc: " + (fmt % args) + "\n")

    def _send_json(self, obj: dict[str, Any], status: int = 200) -> None:
        payload = json.dumps(obj).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Cache-Control", "no-store")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def _redirect(self, location: str) -> None:
        self.send_response(302)
        self.send_header("Location", location)
        self.send_header("Content-Length", "0")
        self.end_headers()

    def _read_form(self) -> dict[str, str]:
        length = int(self.headers.get("Content-Length", 0) or 0)
        raw = self.rfile.read(length) if length else b""
        ctype = (self.headers.get("Content-Type") or "").split(";")[0].strip()
        if ctype == "application/json":
            try:
                obj = json.loads(raw or b"{}")
                return {k: str(v) for k, v in obj.items()}
            except json.JSONDecodeError:
                return {}
        # default: application/x-www-form-urlencoded
        parsed = urllib.parse.parse_qs(raw.decode("utf-8"))
        return {k: v[0] for k, v in parsed.items()}

    def do_GET(self) -> None:
        parsed = urllib.parse.urlparse(self.path)
        path = parsed.path.rstrip("/") or "/"
        if path == "/.well-known/openid-configuration":
            self._send_json(self.state.discovery())
            return
        if path == "/jwks":
            self._send_json(self.state.jwks())
            return
        if path == "/authorize":
            q = urllib.parse.parse_qs(parsed.query)
            redirect_uri = q.get("redirect_uri", [""])[0]
            stateparam = q.get("state", [""])[0]
            if not redirect_uri:
                self._send_json({"error": "invalid_request",
                                 "error_description": "missing redirect_uri"}, 400)
                return
            sep = "&" if urllib.parse.urlparse(redirect_uri).query else "?"
            loc = (redirect_uri + sep + "code=" + urllib.parse.quote(FIXED_CODE)
                   + "&state=" + urllib.parse.quote(stateparam))
            self._redirect(loc)
            return
        if path in ("/healthz", "/health"):
            body = b"ok"
            self.send_response(200)
            self.send_header("Content-Type", "text/plain; charset=utf-8")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return
        self._send_json({"error": "not_found", "path": self.path}, 404)

    def do_POST(self) -> None:
        parsed = urllib.parse.urlparse(self.path)
        path = parsed.path.rstrip("/") or "/"
        if path == "/token":
            form = self._read_form()
            grant = form.get("grant_type", "")
            code = form.get("code", "")
            if grant != "authorization_code" or code != FIXED_CODE:
                self._send_json({"error": "invalid_grant"}, 400)
                return
            # client_id may arrive in the body (client_secret_post) or Basic auth.
            aud = form.get("client_id") or self._basic_client_id() or "mock-client"
            id_token = self.state.id_token(aud)
            self._send_json({
                "access_token": "mock-access-token",
                "token_type": "Bearer",
                "expires_in": 3600,
                "scope": "openid email profile",
                "id_token": id_token,
            })
            return
        self._send_json({"error": "not_found", "path": self.path}, 404)

    def _basic_client_id(self) -> str:
        auth = self.headers.get("Authorization", "")
        if auth.startswith("Basic "):
            try:
                dec = base64.b64decode(auth[6:]).decode("utf-8")
                return urllib.parse.unquote(dec.split(":", 1)[0])
            except Exception:
                return ""
        return ""


def build_state(port: int) -> OIDCState:
    issuer = os.environ.get("AGENTOS_MOCK_OIDC_ISSUER") or f"http://localhost:{port}"
    return OIDCState(issuer)


def serve(port: int, issuer: str | None = None) -> ThreadingHTTPServer:
    Handler.state = OIDCState(issuer) if issuer else build_state(port)
    server = ThreadingHTTPServer(("0.0.0.0", port), Handler)
    return server


def main() -> None:
    parser = argparse.ArgumentParser(description="Minimal stdlib OIDC provider mock")
    parser.add_argument("port", nargs="?",
                        type=int,
                        default=int(os.environ.get("AGENTOS_MOCK_OIDC_PORT", "9000")),
                        help="port to bind (default 9000 or $AGENTOS_MOCK_OIDC_PORT)")
    parser.add_argument("--selftest", action="store_true",
                        help="run an in-process end-to-end + RS256 verify self-check")
    args = parser.parse_args()
    if args.selftest:
        sys.exit(0 if selftest() else 1)
    server = serve(args.port)
    sys.stderr.write(
        f"mock-oidc: listening on http://0.0.0.0:{args.port} "
        f"(issuer={Handler.state.issuer})\n"
    )
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()


# --- self-test --------------------------------------------------------------

def selftest() -> bool:
    """Start the server, drive the full flow, and cryptographically verify the
    RS256 ID-token signature with the public key. Prints PASS/FAIL."""
    import http.client
    import threading
    import urllib.request

    ok = True

    def check(cond: bool, label: str) -> None:
        nonlocal ok
        status = "PASS" if cond else "FAIL"
        if not cond:
            ok = False
        print(f"  [{status}] {label}")

    port = 19000
    issuer = f"http://127.0.0.1:{port}"
    server = serve(port, issuer=issuer)
    t = threading.Thread(target=server.serve_forever, daemon=True)
    t.start()
    try:
        # 1) discovery
        with urllib.request.urlopen(issuer + "/.well-known/openid-configuration") as r:
            disc = json.loads(r.read())
        check(disc["issuer"] == issuer, "discovery issuer matches")
        check(disc["token_endpoint"] == issuer + "/token", "discovery token_endpoint")
        check(disc["jwks_uri"] == issuer + "/jwks", "discovery jwks_uri")
        check("RS256" in disc["id_token_signing_alg_values_supported"],
              "discovery advertises RS256")

        # 2) authorize -> 302 with code + state. Use http.client so the redirect
        #    is NOT auto-followed and we can assert the 302 + Location directly.
        redirect_uri = "http://gateway/auth/oidc/callback"
        auth_path = ("/authorize?response_type=code&client_id=cid"
                     "&scope=" + urllib.parse.quote("openid email profile")
                     + "&redirect_uri=" + urllib.parse.quote(redirect_uri)
                     + "&state=xyz-state")
        conn = http.client.HTTPConnection("127.0.0.1", port)
        conn.request("GET", auth_path)
        resp = conn.getresponse()
        loc = resp.getheader("Location", "")
        resp.read()
        conn.close()
        check(resp.status == 302, "authorize returns 302")
        loc_q = urllib.parse.parse_qs(urllib.parse.urlparse(loc).query)
        check(loc_q.get("code", [""])[0] == FIXED_CODE, "authorize returns fixed code")
        check(loc_q.get("state", [""])[0] == "xyz-state", "authorize echoes state")

        # 3) token exchange
        body = urllib.parse.urlencode({
            "grant_type": "authorization_code",
            "code": FIXED_CODE,
            "redirect_uri": redirect_uri,
            "client_id": "cid",
            "client_secret": "secret",
        }).encode()
        treq = urllib.request.Request(issuer + "/token", data=body,
                                      headers={"Content-Type":
                                               "application/x-www-form-urlencoded"})
        with urllib.request.urlopen(treq) as r:
            tok = json.loads(r.read())
        check(tok.get("token_type") == "Bearer", "token_type Bearer")
        check(bool(tok.get("id_token")), "id_token present")
        jwt = tok["id_token"]

        # 4) decode header + payload
        seg_h, seg_p, seg_s = jwt.split(".")
        header = json.loads(b64url_decode(seg_h))
        payload = json.loads(b64url_decode(seg_p))
        check(header.get("alg") == "RS256", "JWT header alg RS256")
        check(header.get("kid") == KID, "JWT header kid mock-key-1")
        check(payload.get("iss") == issuer, "claim iss == issuer")
        check(payload.get("aud") == "cid", "claim aud == client_id")
        check(payload.get("sub") == SUB, "claim sub == mock-user-1")
        check(payload.get("email") == EMAIL, "claim email == alice@example.com")
        check(payload.get("exp", 0) > payload.get("iat", 0), "claim exp > iat")

        # 5) CRYPTOGRAPHICALLY verify the RS256 signature with the public key.
        signing_input = (seg_h + "." + seg_p).encode("ascii")
        signature = b64url_decode(seg_s)
        check(rs256_verify(signing_input, signature),
              "RS256 signature verifies against public key (real PKCS#1 v1.5)")
        # negative control: a tampered signing input must NOT verify.
        bad_input = (seg_h + "." + seg_p + "x").encode("ascii")
        check(not rs256_verify(bad_input, signature),
              "tampered signing input rejected")

        # 6) jwks exposes the same public key
        with urllib.request.urlopen(issuer + "/jwks") as r:
            jwks = json.loads(r.read())
        k = jwks["keys"][0]
        check(k["kid"] == KID and k["alg"] == "RS256", "jwks key metadata")
        check(k["n"] == b64url_uint(RSA_N) and k["e"] == b64url_uint(RSA_E),
              "jwks n/e match signing key")
    finally:
        server.shutdown()
        server.server_close()

    print("mock-oidc selftest:", "PASS" if ok else "FAIL")
    return ok


if __name__ == "__main__":
    main()
