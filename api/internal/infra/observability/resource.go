package observability

import (
	"context"

	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// fixedResourceExporterは、送信直前のSpanのResourceを明示設定だけで作った値へ置き換える。
// SDK v1.45.0のWithResourceはOTEL_RESOURCE_ATTRIBUTESを必ずmergeするため、
// Provider設定だけでは環境由来の属性を除外できない。
type fixedResourceExporter struct {
	sdktrace.SpanExporter
	resource *resource.Resource
}

type fixedResourceSpan struct {
	sdktrace.ReadOnlySpan
	resource *resource.Resource
}

func (s fixedResourceSpan) Resource() *resource.Resource {
	return s.resource
}

func (e fixedResourceExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	fixed := make([]sdktrace.ReadOnlySpan, len(spans))
	for i, span := range spans {
		fixed[i] = fixedResourceSpan{ReadOnlySpan: span, resource: e.resource}
	}
	return e.SpanExporter.ExportSpans(ctx, fixed)
}
