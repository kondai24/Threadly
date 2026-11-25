package di

import (
	"strings"
	"testing"

	"Threadly/internal/tracing/tracingtest"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

func TestNewContainer_ProvidesGivenTracer(t *testing.T) {
	tracer, _ := tracingtest.NewRecorder()
	container, err := NewContainer(func(string) string { return "" }, tracer)
	require.NoError(t, err)

	var resolved trace.Tracer
	require.NoError(t, container.Invoke(func(tracer trace.Tracer) { resolved = tracer }))

	require.Same(t, tracer, resolved)
}

// os.Getenvではなく引数のgetenvが使われることを、DB接続前の設定検証で確認する。
func TestResolveRouter_UsesInjectedGetenv(t *testing.T) {
	env := map[string]string{"JWT_SECRET": strings.Repeat("s", 32)}
	t.Setenv("DB_USER", "from-process-env")
	t.Setenv("JWT_SECRET", "")
	tracer, _ := tracingtest.NewRecorder()

	container, err := NewContainer(func(key string) string { return env[key] }, tracer)
	require.NoError(t, err)

	_, err = ResolveRouter(container)
	require.ErrorContains(t, err, "database config DB_USER: required")
}
