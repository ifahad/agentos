// Package telemetry wires the optional OpenTelemetry OTLP/HTTP trace
// exporter. It is only invoked when AGENTOS_OTEL_ENDPOINT is set; otherwise
// no exporter, provider, or background goroutine is started.
package telemetry

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// Setup starts an OTLP/HTTP span exporter against endpoint (a base URL such
// as http://collector:4318; the standard /v1/traces path is appended when the
// URL has no path) and returns a tracer plus a shutdown function.
func Setup(ctx context.Context, endpoint string) (trace.Tracer, func(context.Context) error, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		return nil, nil, fmt.Errorf("invalid OTLP endpoint %q: %v", endpoint, err)
	}
	opts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(u.Host)}
	if u.Scheme != "https" {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	if path := strings.TrimSuffix(u.Path, "/"); path != "" {
		opts = append(opts, otlptracehttp.WithURLPath(path))
	}
	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("create OTLP exporter: %w", err)
	}

	res := sdkresource.NewSchemaless(attribute.String("service.name", "agentos-gateway"))
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	return tp.Tracer("github.com/ifahad/agentos/gateway"), tp.Shutdown, nil
}
