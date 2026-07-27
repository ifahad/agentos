> **Canonical source:** [`docs/deployment.md`](../docs/deployment.md) — observability and Langfuse setup. This file is a snippet kept for reference.

<!-- README-ready section. Intended to slot into README.md directly after the
     existing "## Observability" section content (before "## Roadmap"). -->

### Bundled Langfuse

```bash
docker compose -f deploy/compose.yaml -f deploy/compose.otel.yaml \
  -f deploy/compose.langfuse.yaml up -d
```

brings up Langfuse OSS (`langfuse/langfuse:2` with its own dedicated Postgres,
separate from the AgentOS database) and swaps the collector config for
`deploy/otel-collector.langfuse.yaml`, which exports traces to both the debug
log and Langfuse. First run:

1. Open http://localhost:3001 (override with `AGENTOS_LANGFUSE_PORT`), sign up,
   and create an organization + project.
2. In the project settings, copy the API key pair (`pk-lf-...` / `sk-lf-...`).
3. Base64-encode them into `.env` (see `deploy/langfuse.env.example` for all
   `LANGFUSE_*` variables):

   ```bash
   echo "LANGFUSE_OTLP_BASIC_AUTH=$(echo -n 'pk-lf-xxx:sk-lf-xxx' | base64 -w0)" >> .env
   ```

4. Restart the collector so it picks up the credentials:

   ```bash
   docker compose -f deploy/compose.yaml -f deploy/compose.otel.yaml \
     -f deploy/compose.langfuse.yaml up -d --force-recreate otel-collector
   ```

Gateway and runtime need `AGENTOS_OTEL_ENDPOINT` set for their spans to reach
the collector (and thus Langfuse) — `compose.otel.yaml` already does that, which
is why the Langfuse overlay is always stacked on top of it.

> **OTLP note.** Native OTLP ingestion (`/api/public/otel`) landed in Langfuse
> **v3**. The bundled profile pins the lightweight, Postgres-only `langfuse:2`
> image, whose collector export path is therefore best-effort — the collector's
> `debug` exporter always shows your traces regardless. To ingest spans into the
> Langfuse UI, set `LANGFUSE_IMAGE=langfuse/langfuse:3` in `.env` and add the v3
> dependencies (ClickHouse/Redis/MinIO), or point `LANGFUSE_OTLP_ENDPOINT` at a
> Langfuse Cloud project. The image tag is env-overridable; no file edits needed.
