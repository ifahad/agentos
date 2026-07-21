"""Opt-in OpenTelemetry tracing (AGENTOS_OTEL_ENDPOINT).

When the endpoint is unset this module never imports the OTel SDK or
exporter, so the disabled path has zero import cost and is a strict no-op.
Spans are simple and manual: one per run, plus one child span per executed
tool step (emitted from the recorded steps once the run settles).
"""

from contextlib import contextmanager, nullcontext
from typing import Any

from agentos_runtime.config import Settings


def setup_tracing(settings: Settings) -> Any | None:
    """Return a tracer when AGENTOS_OTEL_ENDPOINT is set, else None."""
    if not settings.otel_endpoint:
        return None
    from opentelemetry import trace
    from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
    from opentelemetry.sdk.resources import Resource
    from opentelemetry.sdk.trace import TracerProvider
    from opentelemetry.sdk.trace.export import BatchSpanProcessor

    provider = TracerProvider(
        resource=Resource.create({"service.name": "agentos-runtime"})
    )
    exporter = OTLPSpanExporter(
        endpoint=settings.otel_endpoint.rstrip("/") + "/v1/traces"
    )
    provider.add_span_processor(BatchSpanProcessor(exporter))
    trace.set_tracer_provider(provider)
    return trace.get_tracer("agentos-runtime")


@contextmanager
def run_span(tracer: Any | None, thread_id: str):
    """Span around one /runs invocation; no-op when tracing is disabled."""
    if tracer is None:
        with nullcontext():
            yield None
        return
    with tracer.start_as_current_span("agent.run") as span:
        span.set_attribute("agentos.thread_id", thread_id)
        yield span


def record_tool_spans(tracer: Any | None, steps: list[Any]) -> None:
    """Emit one child span per executed tool step (within the run span)."""
    if tracer is None:
        return
    for step in steps:
        with tracer.start_as_current_span("agent.tool") as span:
            span.set_attribute("agentos.tool", step.tool)
            span.set_attribute("agentos.tool_input", str(step.input))
