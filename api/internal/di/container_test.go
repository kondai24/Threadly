package di

import (
	"testing"

	"Threadly/internal/tracing/tracingtest"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

func TestNewContainer_ProvidesGivenTracer(t *testing.T) {
	tracer, _ := tracingtest.NewRecorder()
	container, err := NewContainer(tracer)
	require.NoError(t, err)

	var resolved trace.Tracer
	require.NoError(t, container.Invoke(func(tracer trace.Tracer) { resolved = tracer }))

	require.Same(t, tracer, resolved)
}
