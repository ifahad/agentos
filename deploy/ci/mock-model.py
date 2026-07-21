#!/usr/bin/env python3
"""Deterministic, offline, OpenAI-compatible mock model server for CI evals.

This is a *stdlib-only* stub (``http.server`` — no third-party deps, nothing to
install) that stands in for a real LLM provider so the runtime eval suite
(``runtime/evals/default.yaml``) can run fully offline and produce the *same*
score on every run. It speaks just enough of the OpenAI HTTP API for the
AgentOS gateway to forward to it:

    POST /v1/chat/completions   -> canned assistant turns (tool-calls + answers)
    POST /v1/embeddings         -> a fixed 1024-dim vector (matches bge-m3)
    GET  /healthz               -> "ok"

Wiring (see .github/workflows/evals.yml)
----------------------------------------
The runtime always calls models through the gateway on the ``ollama/`` prefix,
which the gateway routes to ``AGENTOS_OLLAMA_BASE_URL`` with **no** auth header
and **zero** cost. Point that env var at this server and set the runtime's
three models to ``ollama/...`` names:

    AGENTOS_OLLAMA_BASE_URL = http://<this-host>:8100
    AGENTOS_MODEL           = ollama/mock-agent
    AGENTOS_JUDGE_MODEL     = ollama/mock-judge
    AGENTOS_EMBED_MODEL     = ollama/mock-embed

Determinism
-----------
There is no randomness, no clock-dependent branching, and no state carried
between requests. Every response is a pure function of the request body:

  * The server keys off **substrings in the conversation**, never off model
    names or message ordering, so it is robust to prompt/temperature changes.
  * A turn that already contains a tool result (role ``tool``) is the
    "produce the final answer" turn; otherwise it is the first turn and the
    server either emits a tool-call or answers directly.
  * The LLM-judge prompt is recognised by its template header and always
    scored 1.0 with a fixed justification.
  * The prompt-injection guardrail classifier prompt (only hit when
    AGENTOS_GUARDRAILS_MODE=model) always returns ``{"injection": false}``.

Given identical eval inputs the suite therefore scores 1.0 (5/5) every run,
comfortably above the 0.8 gate. Token counts in the ``usage`` block are fixed
constants (the ``ollama`` route prices at $0 regardless).

Run locally
-----------
    python3 deploy/ci/mock-model.py --port 8100
    curl -s localhost:8100/v1/chat/completions -H 'content-type: application/json' \
      -d '{"model":"mock-agent","messages":[{"role":"user","content":"Which customer has the highest total order value?"}]}'
"""

from __future__ import annotations

import argparse
import json
import os
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any

# --- fixed constants (determinism) ------------------------------------------

EMBED_DIM = 1024  # must match runtime/src/agentos_runtime/context.py EMBED_DIM (bge-m3)
EMBED_VALUE = 0.001  # non-zero so the vector has a defined norm for cosine distance
USAGE = {"prompt_tokens": 32, "completion_tokens": 16, "total_tokens": 48}

# Markers that identify special (non-agent) turns by their prompt text.
JUDGE_MARKER = "you are grading an ai agent's answer"  # evals.JUDGE_TEMPLATE header
CLASSIFIER_MARKER = "prompt-injection classifier"  # guardrail classifierSystemPrompt


# --- canned agent cases -----------------------------------------------------
#
# Each case is matched by substrings that appear in the user question. On the
# first turn the server emits `tool_calls` (when `tool` is set) so the ReAct
# loop runs a real tool; on the turn that already has a tool result it emits
# `answer` (which always contains the suite's expected substring). Cases
# without a tool answer directly on the first turn.

CASES: list[dict[str, Any]] = [
    {
        # judged case — must come before the plain top-customer case because it
        # shares the "revenue"/"customer" vocabulary but is matched by its own
        # distinctive phrasing.
        "match": ["how do you know", "most revenue", "name the data"],
        "tool": ("query", {"sql": "SELECT c.name, SUM(o.total) AS revenue "
                                   "FROM customers c JOIN orders o ON o.customer_id = c.id "
                                   "GROUP BY c.name ORDER BY revenue DESC LIMIT 1"}),
        "answer": (
            "Al-Faisal Trading Co. generates the most revenue for us. I computed "
            "this by summing order totals per customer using the orders and "
            "customers tables of the ERP database (a JOIN of orders to customers, "
            "grouped by customer and ordered by total order value), and Al-Faisal "
            "Trading Co. had the highest total. Source: the orders and customers "
            "tables."
        ),
    },
    {
        "match": ["highest total order value", "top customer", "highest total"],
        "tool": ("query", {"sql": "SELECT c.name, SUM(o.total) AS total "
                                   "FROM customers c JOIN orders o ON o.customer_id = c.id "
                                   "GROUP BY c.name ORDER BY total DESC LIMIT 1"}),
        "answer": (
            "The customer with the highest total order value is Al-Faisal Trading "
            "Co. (source: the orders and customers tables)."
        ),
    },
    {
        "match": ["pending status", "pending", "how many orders"],
        "tool": None,  # no expect_tool on this case; answer directly
        "answer": "There are 4 orders currently in pending status.",
    },
    {
        "match": ["procurement policy", "signs off", "purchase orders above", "250000"],
        "tool": ("search_knowledge", {"query": "who signs off on purchase orders above 250000 SAR"}),
        "answer": (
            "According to our procurement policy, purchase orders above 250,000 SAR "
            "are signed off by Al-Harbi (the procurement director)."
        ),
    },
    {
        "match": ["6 times 7", "6 times", "times 7", "what is 6"],
        "tool": None,
        "answer": "42",
    },
]

DEFAULT_ANSWER = "42"  # harmless deterministic fallback; contains a common expected token


def _text_of(content: Any) -> str:
    """Flatten OpenAI string-or-parts message content to plain text."""
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        out = []
        for part in content:
            if isinstance(part, dict) and isinstance(part.get("text"), str):
                out.append(part["text"])
        return "\n".join(out)
    return ""


def _conversation_text(messages: list[dict[str, Any]]) -> str:
    return "\n".join(_text_of(m.get("content")) for m in messages).lower()


def _user_text(messages: list[dict[str, Any]]) -> str:
    for m in reversed(messages):
        if m.get("role") == "user":
            return _text_of(m.get("content"))
    return _conversation_text(messages)


def _has_tool_result(messages: list[dict[str, Any]]) -> bool:
    return any(m.get("role") == "tool" for m in messages)


def _select_case(user_text: str) -> dict[str, Any]:
    low = user_text.lower()
    for case in CASES:
        if any(needle in low for needle in case["match"]):
            return case
    return {"match": [], "tool": None, "answer": DEFAULT_ANSWER}


def _completion(model: str, message: dict[str, Any], finish_reason: str) -> dict[str, Any]:
    return {
        "id": "chatcmpl-mock-deterministic",
        "object": "chat.completion",
        "created": 0,  # fixed for determinism
        "model": model,
        "choices": [
            {"index": 0, "message": message, "finish_reason": finish_reason}
        ],
        "usage": USAGE,
    }


def build_chat_response(body: dict[str, Any]) -> dict[str, Any]:
    """Pure function: request body -> OpenAI chat.completion response dict."""
    model = body.get("model", "mock")
    messages = body.get("messages") or []
    convo = _conversation_text(messages)

    # 1) LLM judge turn -> strict JSON verdict, always a passing score.
    if JUDGE_MARKER in convo:
        verdict = json.dumps(
            {"score": 1.0, "justification": "Mock judge: criteria met deterministically."}
        )
        return _completion(model, {"role": "assistant", "content": verdict}, "stop")

    # 2) Guardrail classifier turn -> never flag (evals are benign).
    if CLASSIFIER_MARKER in convo:
        verdict = json.dumps({"injection": False, "reason": "mock: benign eval prompt"})
        return _completion(model, {"role": "assistant", "content": verdict}, "stop")

    # 3) Agent turn.
    case = _select_case(_user_text(messages))

    # First turn with a tool to call -> emit tool_calls (ReAct will run it).
    if case["tool"] is not None and not _has_tool_result(messages):
        name, args = case["tool"]
        message = {
            "role": "assistant",
            "content": None,
            "tool_calls": [
                {
                    "id": "call_mock_0",
                    "type": "function",
                    "function": {"name": name, "arguments": json.dumps(args)},
                }
            ],
        }
        return _completion(model, message, "tool_calls")

    # Otherwise (tool result already present, or a no-tool case) -> final answer.
    return _completion(model, {"role": "assistant", "content": case["answer"]}, "stop")


def build_embeddings_response(body: dict[str, Any]) -> dict[str, Any]:
    """Pure function: fixed-dim embedding vector per input item."""
    model = body.get("model", "mock-embed")
    inputs = body.get("input", "")
    if isinstance(inputs, str):
        inputs = [inputs]
    if not isinstance(inputs, list) or not inputs:
        inputs = [""]
    vector = [EMBED_VALUE] * EMBED_DIM
    data = [
        {"object": "embedding", "index": i, "embedding": vector}
        for i in range(len(inputs))
    ]
    return {
        "object": "list",
        "data": data,
        "model": model,
        "usage": {"prompt_tokens": 1, "total_tokens": 1},
    }


def _sse_chat(body: dict[str, Any]) -> bytes:
    """Render a chat response as a minimal OpenAI SSE stream.

    Only used when the caller sets ``stream: true`` (the langgraph ReAct agent
    normally invokes non-streaming). One content/tool_calls delta chunk, a
    finish chunk carrying usage, then ``[DONE]``.
    """
    resp = build_chat_response(body)
    choice = resp["choices"][0]
    msg = choice["message"]
    delta: dict[str, Any] = {"role": "assistant"}
    if msg.get("tool_calls"):
        delta["tool_calls"] = [
            {
                "index": 0,
                "id": tc["id"],
                "type": "function",
                "function": tc["function"],
            }
            for tc in msg["tool_calls"]
        ]
    else:
        delta["content"] = msg.get("content") or ""
    base = {"id": resp["id"], "object": "chat.completion.chunk",
            "created": 0, "model": resp["model"]}
    chunk1 = {**base, "choices": [{"index": 0, "delta": delta, "finish_reason": None}]}
    chunk2 = {**base,
              "choices": [{"index": 0, "delta": {}, "finish_reason": choice["finish_reason"]}],
              "usage": resp["usage"]}
    lines = [
        f"data: {json.dumps(chunk1)}\n\n",
        f"data: {json.dumps(chunk2)}\n\n",
        "data: [DONE]\n\n",
    ]
    return "".join(lines).encode("utf-8")


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, fmt: str, *args: Any) -> None:  # quieter CI logs
        sys.stderr.write("mock-model: " + (fmt % args) + "\n")

    def _send_json(self, obj: dict[str, Any], status: int = 200) -> None:
        payload = json.dumps(obj).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def _read_body(self) -> dict[str, Any]:
        length = int(self.headers.get("Content-Length", 0) or 0)
        raw = self.rfile.read(length) if length else b""
        if not raw:
            return {}
        try:
            return json.loads(raw)
        except json.JSONDecodeError:
            return {}

    def do_GET(self) -> None:
        if self.path.rstrip("/") in ("/healthz", "/health", ""):
            body = b"ok"
            self.send_response(200)
            self.send_header("Content-Type", "text/plain; charset=utf-8")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return
        self._send_json({"error": {"type": "not_found", "message": self.path}}, 404)

    def do_POST(self) -> None:
        path = self.path.split("?", 1)[0].rstrip("/")
        body = self._read_body()
        if path.endswith("/v1/chat/completions") or path.endswith("/chat/completions"):
            if body.get("stream") is True:
                payload = _sse_chat(body)
                self.send_response(200)
                self.send_header("Content-Type", "text/event-stream")
                self.send_header("Content-Length", str(len(payload)))
                self.end_headers()
                self.wfile.write(payload)
                return
            self._send_json(build_chat_response(body))
            return
        if path.endswith("/v1/embeddings") or path.endswith("/embeddings"):
            self._send_json(build_embeddings_response(body))
            return
        self._send_json({"error": {"type": "not_found", "message": self.path}}, 404)


def main() -> None:
    parser = argparse.ArgumentParser(description="Deterministic OpenAI-compatible mock model")
    parser.add_argument(
        "--port",
        type=int,
        default=int(os.environ.get("MOCK_MODEL_PORT", "8100")),
        help="port to listen on (default 8100 or $MOCK_MODEL_PORT)",
    )
    parser.add_argument("--host", default=os.environ.get("MOCK_MODEL_HOST", "0.0.0.0"))
    args = parser.parse_args()
    server = ThreadingHTTPServer((args.host, args.port), Handler)
    sys.stderr.write(f"mock-model: listening on http://{args.host}:{args.port}\n")
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()


if __name__ == "__main__":
    main()
