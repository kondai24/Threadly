package tracing

import (
	"context"
	"testing"

	"Threadly/internal/tracing/tracingtest"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func TestEnd_RecordsExpectedResultsWithoutError(t *testing.T) {
	tracer, recorder := tracingtest.NewRecorder()

	_, span := Start(context.Background(), tracer, LayerUsecase, "PostUsecase.GetPostByID")
	End(span, ResultNotFound, nil)

	spans := recorder.Ended()
	require.Len(t, spans, 1)
	require.Equal(t, "PostUsecase.GetPostByID", spans[0].Name())
	require.Equal(t, trace.SpanKindInternal, spans[0].SpanKind())
	require.Equal(t, "usecase", tracingtest.Attribute(spans[0], "threadly.layer"))
	require.Equal(t, "not_found", tracingtest.Attribute(spans[0], "threadly.result"))
	require.Equal(t, codes.Unset, spans[0].Status().Code)
	require.Empty(t, spans[0].Events())
	require.Empty(t, tracingtest.Attribute(spans[0], "error.type"))
}

func TestEnd_RecordsInternalErrorAsFixedSafeError(t *testing.T) {
	tracer, recorder := tracingtest.NewRecorder()

	_, span := Start(context.Background(), tracer, LayerRepository, "PostRepository.GetByID")
	End(span, ResultInternalError, nil)

	spans := recorder.Ended()
	require.Len(t, spans, 1)
	require.Equal(t, codes.Error, spans[0].Status().Code)
	require.Equal(t, "internal_error", spans[0].Status().Description)
	require.Equal(t, "internal_error", tracingtest.Attribute(spans[0], "error.type"))
	require.Len(t, spans[0].Events(), 1)
	for _, attr := range spans[0].Events()[0].Attributes {
		require.NotEqual(t, attribute.Key("exception.stacktrace"), attr.Key)
		if attr.Key == "exception.message" {
			require.Equal(t, "internal_error", attr.Value.AsString())
		}
	}
}

func TestEnd_RecordsPanicAndRepanicsSameValue(t *testing.T) {
	const secret = "dummy-secret-value"
	tracer, recorder := tracingtest.NewRecorder()

	require.PanicsWithValue(t, secret, func() {
		_, span := Start(context.Background(), tracer, LayerHandler, "PostController.ListPostsHandler")
		defer func() { End(span, ResultSuccess, recover()) }()
		panic(secret)
	})

	spans := recorder.Ended()
	require.Len(t, spans, 1)
	require.Equal(t, "panic", tracingtest.Attribute(spans[0], "threadly.result"))
	require.Equal(t, codes.Error, spans[0].Status().Code)
	require.Equal(t, "panic", spans[0].Status().Description)
	for _, event := range spans[0].Events() {
		for _, attr := range event.Attributes {
			require.NotContains(t, attr.Value.String(), secret)
		}
	}
}
