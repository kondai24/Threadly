package observability

import (
	"context"
	"crypto/tls"
	"errors"
	"log"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	scopeName = "Threadly/api"

	batchQueueSize     = 2048
	batchMaxSize       = 512
	batchInterval      = 5 * time.Second
	exportTimeout      = 5 * time.Second
	serviceNameAttrKey = "service.name"
)

var errShutdownFailed = errors.New("trace shutdown_failed")

// TraceRuntimeは、API全体で共有するTracerProviderと終了処理を保持する。
type TraceRuntime struct {
	provider trace.TracerProvider
	shutdown func(context.Context) error
}

// NewTraceRuntimeは、明示的に解釈するOTEL_*設定だけからTrace送信を構成する。
// Collectorへの接続確認は起動条件にしない。
func NewTraceRuntime(ctx context.Context, getenv func(string) string) (*TraceRuntime, error) {
	config, err := parseTraceConfig(getenv)
	if err != nil {
		return nil, err
	}
	if !config.enabled {
		return newNoopTraceRuntime(), nil
	}

	// 配送失敗のerrorはendpointやCollector応答を含み得るため、固定の分類だけを報告する。
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(error) {
		log.Print("trace export_failed")
	}))
	exporter, err := otlptracegrpc.New(ctx, exporterOptions(config)...)
	if err != nil {
		return nil, &configError{key: envOTLPEndpoint, reason: "exporter setup failed"}
	}
	serviceResource := resource.NewSchemaless(attribute.String(serviceNameAttrKey, config.serviceName))
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(serviceResource),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
		// defer span.End()がpanic値とstackをexception eventとして自動保存する経路を塞ぐ。
		sdktrace.WithoutPanicRecording(),
		// queue満杯時はSpanを落とし、HTTP処理を送信待ちで止めない（WithBlockingは使わない）。
		sdktrace.WithBatcher(
			fixedResourceExporter{SpanExporter: exporter, resource: serviceResource},
			sdktrace.WithMaxQueueSize(batchQueueSize),
			sdktrace.WithMaxExportBatchSize(batchMaxSize),
			sdktrace.WithBatchTimeout(batchInterval),
			sdktrace.WithExportTimeout(exportTimeout),
		),
	)
	return &TraceRuntime{provider: provider, shutdown: provider.Shutdown}, nil
}

// exporterOptionsは、Exporterが環境変数から読む設定を検証済みの値で上書きする。
// headerやTLS設定が環境から暗黙に混入しないよう、空のheaderと資格情報も明示する。
func exporterOptions(config traceConfig) []otlptracegrpc.Option {
	transport := insecure.NewCredentials()
	if config.endpoint.Scheme == "https" {
		transport = credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})
	}
	return []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(config.endpoint.Host),
		otlptracegrpc.WithTLSCredentials(transport),
		otlptracegrpc.WithHeaders(map[string]string{}),
		otlptracegrpc.WithTimeout(exportTimeout),
	}
}

func newNoopTraceRuntime() *TraceRuntime {
	return &TraceRuntime{
		provider: noop.NewTracerProvider(),
		shutdown: func(context.Context) error { return nil },
	}
}

func (r *TraceRuntime) Tracer() trace.Tracer {
	return r.provider.Tracer(scopeName)
}

// Shutdownは、未送信のSpanをctxの期限内で送信してからProviderを終了する。
func (r *TraceRuntime) Shutdown(ctx context.Context) error {
	if err := r.shutdown(ctx); err != nil {
		return errShutdownFailed
	}
	return nil
}
