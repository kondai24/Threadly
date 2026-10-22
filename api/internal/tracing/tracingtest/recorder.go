// Package tracingtestは、各テスト専用のSDK ProviderでSpanを記録する補助を提供する。
// global Providerを使わないため、並列テスト間で記録が混ざらない。
package tracingtest

import (
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// NewRecorderは、実行時と同じくpanic値の自動記録を無効にしたTracerとRecorderを返す。
func NewRecorder() (trace.Tracer, *tracetest.SpanRecorder) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(recorder),
		sdktrace.WithoutPanicRecording(),
	)
	return provider.Tracer("test"), recorder
}

// Attributeは、Spanの文字列属性を返す。属性がなければ空文字を返す。
func Attribute(span sdktrace.ReadOnlySpan, key string) string {
	for _, attr := range span.Attributes() {
		if string(attr.Key) == key {
			return attr.Value.String()
		}
	}
	return ""
}

// Resultsは、終了済みSpanを「Span名 結果分類」の形で終了順に返す。
func Results(spans []sdktrace.ReadOnlySpan) []string {
	results := make([]string, 0, len(spans))
	for _, span := range spans {
		results = append(results, span.Name()+" "+Attribute(span, "threadly.result"))
	}
	return results
}
