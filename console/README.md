# console/ — Admin UI (Phase 2)

TypeScript admin console for AgentOS: virtual keys and budgets, usage and
audit trails, agent runs (with streaming and human-in-the-loop tool
approvals), and knowledge-base documents.

Vite + React + TypeScript SPA, hand-rolled CSS (no component library),
served by nginx in production.

## Pages

- **Overview** — per-key requests/tokens/spend from `GET /admin/usage`.
- **Keys** — list keys and budgets; create a key (secret shown exactly once).
- **Audit** — latest gateway events from `GET /admin/audit?limit=100` with
  kind badges (`chat`, `embeddings`, `guardrail_flag`, `guardrail_block`).
- **Playground** — `POST /runs` (or `POST /runs/stream` with the streaming
  toggle); tool calls render as a timeline; HTTP 202 `pending_approval`
  surfaces Approve/Deny wired to `POST /runs/{thread_id}/approve`; the
  thread id is kept so the conversation continues.
- **Documents** — list and ingest documents for the context engine.

The gateway admin key is entered via the Settings modal (sidebar), stored in
`localStorage`, and sent as `Authorization: Bearer` on `/admin/*` calls only.

## API access

The SPA is same-origin only: it calls `/api/gateway/...` and
`/api/runtime/...`.

- **Production** — nginx (see `nginx.conf`) strips the prefix and proxies to
  `http://gateway:8080/` and `http://runtime:8000/` (compose service names),
  with SPA fallback (`try_files ... /index.html`) for client-side routes.
- **Development** — the vite dev server proxies the same paths to
  `http://localhost:8080` (gateway) and `http://localhost:18000` (runtime);
  see `vite.config.ts`.

## Development

```sh
npm install
npm run dev        # http://localhost:5173, APIs proxied as above
```

## Build & test

```sh
npm run build      # tsc typecheck + vite build → dist/
npm test           # vitest, pure logic only (api client, SSE parser, formatting)
npm run preview    # serve the production build locally
```

## Docker

Multi-stage image: `node:22-alpine` build → `nginx:alpine` serving `dist/`
on port 80 with the reverse proxy above. Built by the compose stack as the
`console` service (published on `${AGENTOS_CONSOLE_PORT:-3000}`).

```sh
docker build -t agentos-console .
```
