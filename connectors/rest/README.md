# agentos-rest connector

MCP server (`agentos-rest`, StreamableHTTP `/mcp` on `:8091`) that turns an
OpenAPI 3 JSON spec into tools: `list_operations` plus one tool per spec
operation (name = `operationId` snake_cased), proxied to the upstream API.

## Environment

| Variable | Default | Meaning |
| --- | --- | --- |
| `AGENTOS_REST_SPEC_URL` | — (required) | OpenAPI 3 JSON spec, `http(s)://` URL or file path. Fatal if unset or unfetchable. |
| `AGENTOS_REST_BASE_URL` | first spec `servers[].url` | Upstream base URL override. |
| `AGENTOS_REST_ALLOW_MUTATIONS` | `false` | When false, only `GET` operations are registered. |
| `AGENTOS_REST_AUTH_HEADER` | — | Raw header `Name: value` attached to every upstream request. |
| `AGENTOS_REST_MAX_BODY_BYTES` | `65536` | Upstream response body cap; larger bodies are truncated (`"truncated": true`). |

Tool results are `{"status": N, "body": ...}` — `body` is parsed JSON when the
upstream `Content-Type` is JSON, otherwise a string (truncated at the cap; a
JSON body over the cap is returned as the raw truncated string).

## Limitation

**No request-body support.** Tools accept path and query parameters only;
operations that need a JSON request body cannot be driven yet. Mutations are
gated off by default anyway (`AGENTOS_REST_ALLOW_MUTATIONS=false`), and
operations without an `operationId` are skipped (logged at startup).
