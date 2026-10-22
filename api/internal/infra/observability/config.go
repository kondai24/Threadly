package observability

import (
	"fmt"
	"net/url"
)

const (
	envSDKDisabled     = "OTEL_SDK_DISABLED"
	envTracesExporter  = "OTEL_TRACES_EXPORTER"
	envMetricsExporter = "OTEL_METRICS_EXPORTER"
	envLogsExporter    = "OTEL_LOGS_EXPORTER"
	envTracesSampler   = "OTEL_TRACES_SAMPLER"
	envServiceName     = "OTEL_SERVICE_NAME"
	envOTLPEndpoint    = "OTEL_EXPORTER_OTLP_ENDPOINT"
	envOTLPProtocol    = "OTEL_EXPORTER_OTLP_PROTOCOL"
)

// traceConfigは、bootstrapが明示的に解釈したOTEL_*設定だけを保持する。
type traceConfig struct {
	enabled     bool
	serviceName string
	endpoint    *url.URL
}

// configErrorは、raw値・endpoint・元errorを含めず、設定項目名と分類だけを返す。
type configError struct {
	key    string
	reason string
}

func (e *configError) Error() string {
	return fmt.Sprintf("trace config %s: %s", e.key, e.reason)
}

// parseTraceConfigは、SDK無効化→Exporter→未対応signal→Sampler→送信先の順に検証する。
// 送信しない設定では、送信先などの有効化専用設定を検証しない。
func parseTraceConfig(getenv func(string) string) (traceConfig, error) {
	switch getenv(envSDKDisabled) {
	case "", "false":
	case "true":
		return traceConfig{}, nil
	default:
		return traceConfig{}, &configError{key: envSDKDisabled, reason: "invalid boolean"}
	}

	enabled := false
	switch getenv(envTracesExporter) {
	case "", "none":
	case "otlp":
		enabled = true
	default:
		return traceConfig{}, &configError{key: envTracesExporter, reason: "unsupported value"}
	}
	// Metrics/Logsは今回Providerを持たないため、送信を求める設定は黙って無視せず拒否する。
	for _, key := range []string{envMetricsExporter, envLogsExporter} {
		if value := getenv(key); value != "" && value != "none" {
			return traceConfig{}, &configError{key: key, reason: "unsupported value"}
		}
	}
	if sampler := getenv(envTracesSampler); sampler != "" && sampler != "parentbased_always_on" {
		return traceConfig{}, &configError{key: envTracesSampler, reason: "unsupported value"}
	}
	if !enabled {
		return traceConfig{}, nil
	}

	serviceName := getenv(envServiceName)
	if serviceName == "" {
		return traceConfig{}, &configError{key: envServiceName, reason: "required"}
	}
	switch getenv(envOTLPProtocol) {
	case "grpc":
	case "":
		return traceConfig{}, &configError{key: envOTLPProtocol, reason: "required"}
	default:
		return traceConfig{}, &configError{key: envOTLPProtocol, reason: "unsupported value"}
	}
	endpoint, err := parseOTLPEndpoint(getenv(envOTLPEndpoint))
	if err != nil {
		return traceConfig{}, err
	}
	return traceConfig{enabled: true, serviceName: serviceName, endpoint: endpoint}, nil
}

// parseOTLPEndpointは、gRPC送信先として解釈が一意なscheme://host:portだけを受け付ける。
func parseOTLPEndpoint(raw string) (*url.URL, error) {
	if raw == "" {
		return nil, &configError{key: envOTLPEndpoint, reason: "required"}
	}
	endpoint, err := url.Parse(raw)
	if err != nil {
		return nil, &configError{key: envOTLPEndpoint, reason: "invalid url"}
	}
	if endpoint.Scheme != "http" && endpoint.Scheme != "https" {
		return nil, &configError{key: envOTLPEndpoint, reason: "unsupported scheme"}
	}
	if endpoint.Hostname() == "" || endpoint.Port() == "" {
		return nil, &configError{key: envOTLPEndpoint, reason: "host and port required"}
	}
	if endpoint.User != nil || endpoint.RawQuery != "" || endpoint.ForceQuery || endpoint.Fragment != "" {
		return nil, &configError{key: envOTLPEndpoint, reason: "unsupported url component"}
	}
	if endpoint.Path != "" && endpoint.Path != "/" {
		return nil, &configError{key: envOTLPEndpoint, reason: "unsupported path"}
	}
	return endpoint, nil
}
