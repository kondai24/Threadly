package observability

import (
	"bytes"
	"context"
	"log"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func envOf(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func enabledEnv(overrides map[string]string) map[string]string {
	env := map[string]string{
		"OTEL_TRACES_EXPORTER":        "otlp",
		"OTEL_SERVICE_NAME":           "threadly-api",
		"OTEL_EXPORTER_OTLP_ENDPOINT": "http://localhost:4317",
		"OTEL_EXPORTER_OTLP_PROTOCOL": "grpc",
	}
	for key, value := range overrides {
		env[key] = value
	}
	return env
}

func TestNewTraceRuntime_RejectsInvalidSettingsWithoutRawValues(t *testing.T) {
	const secret = "dummy-secret-value"
	cases := []struct {
		name string
		env  map[string]string
		key  string
	}{
		{name: "SDK無効化はtrue/falseだけを許可する", env: map[string]string{"OTEL_SDK_DISABLED": secret}, key: "OTEL_SDK_DISABLED"},
		{name: "未対応のExporterを拒否する", env: map[string]string{"OTEL_TRACES_EXPORTER": secret}, key: "OTEL_TRACES_EXPORTER"},
		{name: "複数Exporterを拒否する", env: map[string]string{"OTEL_TRACES_EXPORTER": "otlp,console"}, key: "OTEL_TRACES_EXPORTER"},
		{name: "Metrics送信を拒否する", env: map[string]string{"OTEL_METRICS_EXPORTER": "otlp"}, key: "OTEL_METRICS_EXPORTER"},
		{name: "Logs送信を拒否する", env: map[string]string{"OTEL_LOGS_EXPORTER": "otlp"}, key: "OTEL_LOGS_EXPORTER"},
		{name: "未対応のSamplerを拒否する", env: enabledEnv(map[string]string{"OTEL_TRACES_SAMPLER": secret}), key: "OTEL_TRACES_SAMPLER"},
		{name: "送信時はservice名が必須", env: enabledEnv(map[string]string{"OTEL_SERVICE_NAME": ""}), key: "OTEL_SERVICE_NAME"},
		{name: "送信時はprotocolが必須", env: enabledEnv(map[string]string{"OTEL_EXPORTER_OTLP_PROTOCOL": ""}), key: "OTEL_EXPORTER_OTLP_PROTOCOL"},
		{name: "gRPC以外のprotocolを拒否する", env: enabledEnv(map[string]string{"OTEL_EXPORTER_OTLP_PROTOCOL": "http/protobuf"}), key: "OTEL_EXPORTER_OTLP_PROTOCOL"},
		{name: "送信時はendpointが必須", env: enabledEnv(map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": ""}), key: "OTEL_EXPORTER_OTLP_ENDPOINT"},
		{name: "schemeのないendpointを拒否する", env: enabledEnv(map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": "localhost:4317"}), key: "OTEL_EXPORTER_OTLP_ENDPOINT"},
		{name: "http/https以外を拒否する", env: enabledEnv(map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": "ftp://localhost:4317"}), key: "OTEL_EXPORTER_OTLP_ENDPOINT"},
		{name: "portのないendpointを拒否する", env: enabledEnv(map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": "http://localhost"}), key: "OTEL_EXPORTER_OTLP_ENDPOINT"},
		{name: "userinfoを拒否する", env: enabledEnv(map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": "http://user:" + secret + "@localhost:4317"}), key: "OTEL_EXPORTER_OTLP_ENDPOINT"},
		{name: "queryを拒否する", env: enabledEnv(map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": "http://localhost:4317?token=" + secret}), key: "OTEL_EXPORTER_OTLP_ENDPOINT"},
		{name: "fragmentを拒否する", env: enabledEnv(map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": "http://localhost:4317#" + secret}), key: "OTEL_EXPORTER_OTLP_ENDPOINT"},
		{name: "root以外のpathを拒否する", env: enabledEnv(map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": "http://localhost:4317/" + secret}), key: "OTEL_EXPORTER_OTLP_ENDPOINT"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			runtime, err := NewTraceRuntime(context.Background(), envOf(tt.env))
			require.Error(t, err)
			require.Nil(t, runtime)
			require.Contains(t, err.Error(), tt.key)
			require.NotContains(t, err.Error(), secret)
		})
	}
}

func TestNewTraceRuntime_DisabledSettingsUseNoopTracer(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
	}{
		{name: "未設定は送信しない", env: map[string]string{}},
		{name: "Exporterがnoneなら送信しない", env: map[string]string{"OTEL_TRACES_EXPORTER": "none"}},
		{
			name: "SDK無効化を最優先にする",
			env: map[string]string{
				"OTEL_SDK_DISABLED":    "true",
				"OTEL_TRACES_EXPORTER": "otlp",
			},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			runtime, err := NewTraceRuntime(context.Background(), envOf(tt.env))
			require.NoError(t, err)

			_, span := runtime.Tracer().Start(context.Background(), "span")
			span.End()
			require.False(t, span.SpanContext().IsValid())
			require.NoError(t, runtime.Shutdown(context.Background()))
		})
	}
}

func TestTraceRuntime_ExportsWithResourceEnvironmentAndExplicitTransportSettings(t *testing.T) {
	const secret = "dummy-secret-value"
	// Resource属性の環境設定は許容し、送信先と認証headerは検証済み設定に固定する。
	t.Setenv("OTEL_EXPORTER_OTLP_HEADERS", "authorization="+secret)
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_HEADERS", "authorization="+secret)
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "http://127.0.0.1:1")
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "service.name=environment-api,service.version=1.2.3")
	t.Setenv("OTEL_SERVICE_NAME", "environment-service")
	collector := startFakeCollector(t)
	ctx := context.Background()

	runtime, err := NewTraceRuntime(ctx, envOf(enabledEnv(map[string]string{
		"OTEL_EXPORTER_OTLP_ENDPOINT": "http://" + collector.addr,
	})))
	require.NoError(t, err)
	_, span := runtime.Tracer().Start(ctx, "PostUsecase.ListAllPosts")
	span.End()
	require.NoError(t, runtime.Shutdown(ctx))

	requests, metadata := collector.received()
	require.Len(t, requests, 1)
	resourceSpans := requests[0].GetResourceSpans()
	require.Len(t, resourceSpans, 1)
	resourceAttributes := make(map[string]string)
	for _, attr := range resourceSpans[0].GetResource().GetAttributes() {
		resourceAttributes[attr.GetKey()] = attr.GetValue().GetStringValue()
	}
	require.Equal(t, map[string]string{
		"service.name":    "threadly-api",
		"service.version": "1.2.3",
	}, resourceAttributes)
	scopeSpans := resourceSpans[0].GetScopeSpans()
	require.Len(t, scopeSpans, 1)
	require.Equal(t, "Threadly/api", scopeSpans[0].GetScope().GetName())
	require.Len(t, scopeSpans[0].GetSpans(), 1)
	require.Equal(t, "PostUsecase.ListAllPosts", scopeSpans[0].GetSpans()[0].GetName())
	require.NotContains(t, metadata[0].Get("authorization"), secret)
}

func newExportingRuntime(t *testing.T, collector *fakeCollector) *TraceRuntime {
	t.Helper()
	runtime, err := NewTraceRuntime(context.Background(), envOf(enabledEnv(map[string]string{
		"OTEL_EXPORTER_OTLP_ENDPOINT": "http://" + collector.addr,
	})))
	require.NoError(t, err)
	return runtime
}

func exportedSpans(t *testing.T, collector *fakeCollector) []*tracepb.Span {
	t.Helper()
	requests, _ := collector.received()
	var spans []*tracepb.Span
	for _, request := range requests {
		for _, resourceSpans := range request.GetResourceSpans() {
			for _, scopeSpans := range resourceSpans.GetScopeSpans() {
				spans = append(spans, scopeSpans.GetSpans()...)
			}
		}
	}
	return spans
}

func TestTraceRuntime_RespectsParentSamplingDecision(t *testing.T) {
	collector := startFakeCollector(t)
	runtime := newExportingRuntime(t, collector)
	unsampledParent := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{1},
		SpanID:  trace.SpanID{1},
		Remote:  true,
	})

	_, root := runtime.Tracer().Start(context.Background(), "root")
	root.End()
	_, child := runtime.Tracer().Start(
		trace.ContextWithRemoteSpanContext(context.Background(), unsampledParent),
		"unsampled-child",
	)
	child.End()
	require.NoError(t, runtime.Shutdown(context.Background()))

	spans := exportedSpans(t, collector)
	require.Len(t, spans, 1)
	require.Equal(t, "root", spans[0].GetName())
}

func TestTraceRuntime_DoesNotRecordPanicValues(t *testing.T) {
	const secret = "dummy-secret-value"
	collector := startFakeCollector(t)
	runtime := newExportingRuntime(t, collector)

	require.PanicsWithValue(t, secret, func() {
		_, span := runtime.Tracer().Start(context.Background(), "panicking")
		defer span.End()
		panic(secret)
	})
	require.NoError(t, runtime.Shutdown(context.Background()))

	spans := exportedSpans(t, collector)
	require.Len(t, spans, 1)
	require.Empty(t, spans[0].GetEvents())
}

// captureLogは、標準logへ出る報告内容を確認する。global出力を差し替えるため並列実行しない。
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var output bytes.Buffer
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &output
}

func TestTraceRuntime_ReportsExportFailureWithoutRawError(t *testing.T) {
	const secret = "dummy-secret-value"
	output := captureLog(t)
	collector := startFakeCollectorReturning(t, status.Error(codes.InvalidArgument, secret))
	runtime := newExportingRuntime(t, collector)

	_, span := runtime.Tracer().Start(context.Background(), "span")
	span.End()
	require.NoError(t, runtime.Shutdown(context.Background()))

	require.Contains(t, output.String(), "export_failed")
	require.NotContains(t, output.String(), secret)
}

func TestTraceRuntime_StartsAndShutsDownWithinDeadlineWhenCollectorIsUnreachable(t *testing.T) {
	captureLog(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	unreachable := listener.Addr().String()
	require.NoError(t, listener.Close())

	runtime, err := NewTraceRuntime(context.Background(), envOf(enabledEnv(map[string]string{
		"OTEL_EXPORTER_OTLP_ENDPOINT": "http://" + unreachable,
	})))
	require.NoError(t, err)
	_, span := runtime.Tracer().Start(context.Background(), "span")
	span.End()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	started := time.Now()
	err = runtime.Shutdown(ctx)
	require.Less(t, time.Since(started), 2*time.Second)
	require.EqualError(t, err, "trace shutdown_failed")
}

func TestTraceRuntime_IgnoresSpanLimitEnvironment(t *testing.T) {
	// SDKが環境変数から読むspan limitも表外の設定として扱い、既定値で固定する。
	t.Setenv("OTEL_SPAN_ATTRIBUTE_COUNT_LIMIT", "1")
	t.Setenv("OTEL_ATTRIBUTE_COUNT_LIMIT", "1")
	collector := startFakeCollector(t)
	runtime := newExportingRuntime(t, collector)

	_, span := runtime.Tracer().Start(context.Background(), "span", trace.WithAttributes(
		attribute.String("threadly.layer", "usecase"),
		attribute.String("threadly.result", "success"),
	))
	span.End()
	require.NoError(t, runtime.Shutdown(context.Background()))

	spans := exportedSpans(t, collector)
	require.Len(t, spans, 1)
	require.Len(t, spans[0].GetAttributes(), 2)
}
