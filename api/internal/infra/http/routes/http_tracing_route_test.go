package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"Threadly/internal/domain/models"
	"Threadly/internal/middleware"
	"Threadly/internal/tracing/tracingtest"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/mock/gomock"
)

const (
	remoteTraceID = "4bf92f3577b34da6a3ce929d0e0e4736"
	remoteSpanID  = "00f067aa0ba902b7"
)

func TestTraceHTTP_RecordsHealthAsSingleServerSpan(t *testing.T) {
	router, _, recorder := newRecordingRouter(t)

	response := performRequest(router, http.MethodGet, "/health", "", "")

	require.Equal(t, http.StatusOK, response.Code)
	spans := recorder.Ended()
	require.Equal(t, []string{"GET /health success"}, tracingtest.Results(spans))
	require.Equal(t, trace.SpanKindServer, spans[0].SpanKind())
	require.Equal(t, "http", tracingtest.Attribute(spans[0], "threadly.layer"))
	require.Equal(t, "GET", tracingtest.Attribute(spans[0], "http.request.method"))
	require.Equal(t, "/health", tracingtest.Attribute(spans[0], "http.route"))
	require.Equal(t, "200", tracingtest.Attribute(spans[0], "http.response.status_code"))
	require.Empty(t, response.Header().Get("traceparent"))
}

func performTracedRequest(t *testing.T, router http.Handler, method string, path string, traceparent string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, nil)
	if traceparent != "" {
		request.Header.Set("traceparent", traceparent)
	}
	request.Header.Set("baggage", "user.id="+routeSecret)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestTraceHTTP_ContinuesValidRemoteParent(t *testing.T) {
	router, _, recorder := newRecordingRouter(t)

	performTracedRequest(t, router, http.MethodGet, "/health", "00-"+remoteTraceID+"-"+remoteSpanID+"-01")

	spans := recorder.Ended()
	require.Len(t, spans, 1)
	require.Equal(t, remoteTraceID, spans[0].SpanContext().TraceID().String())
	require.Equal(t, remoteSpanID, spans[0].Parent().SpanID().String())
	require.True(t, spans[0].Parent().IsRemote())
	requireNoSecretInSpans(t, spans)
}

func TestTraceHTTP_StartsNewRootForInvalidParent(t *testing.T) {
	router, _, recorder := newRecordingRouter(t)

	performTracedRequest(t, router, http.MethodGet, "/health", "00-not-a-valid-parent-01")

	spans := recorder.Ended()
	require.Len(t, spans, 1)
	require.False(t, spans[0].Parent().IsValid())
	require.NotEqual(t, remoteTraceID, spans[0].SpanContext().TraceID().String())
}

func TestTraceHTTP_RespectsUnsampledRemoteParent(t *testing.T) {
	router, _, recorder := newRecordingRouter(t)

	response := performTracedRequest(t, router, http.MethodGet, "/health", "00-"+remoteTraceID+"-"+remoteSpanID+"-00")

	require.Equal(t, http.StatusOK, response.Code)
	require.Empty(t, recorder.Ended())
}

func TestTraceHTTP_DoesNotExposeRawPathForUnmatchedRoutes(t *testing.T) {
	cases := []struct {
		name     string
		method   string
		wantName string
	}{
		{name: "未一致routeはunmatchedにする", method: http.MethodGet, wantName: "GET unmatched"},
		{name: "未知methodは_OTHERへ正規化する", method: "DUMMY-SECRET-VALUE", wantName: "_OTHER unmatched"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			router, _, recorder := newRecordingRouter(t)

			response := performTracedRequest(t, router, tt.method, "/api/"+routeSecret+"?token="+routeSecret, "")

			require.Equal(t, http.StatusNotFound, response.Code)
			spans := recorder.Ended()
			require.Equal(t, []string{tt.wantName + " not_found"}, tracingtest.Results(spans))
			require.Empty(t, tracingtest.Attribute(spans[0], "http.route"))
			require.NotContains(t, tracingtest.Attribute(spans[0], "http.request.method"), "DUMMY")
			requireNoSecretInSpans(t, spans)
		})
	}
}

func TestTraceHTTP_KeepsRequestContextForHandlers(t *testing.T) {
	router, repos, recorder := newRecordingRouter(t)
	ctx, cancel := context.WithCancel(context.Background())
	var repositoryCtx context.Context
	repos.Post.EXPECT().ListAll(gomock.Any()).DoAndReturn(func(ctx context.Context) ([]*models.Post, error) {
		repositoryCtx = ctx
		return nil, nil
	})
	request := httptest.NewRequest(http.MethodGet, "/api/posts", nil).WithContext(ctx)
	request.AddCookie(middleware.NewSessionCookie("user-" + string(routeUserID)))

	router.ServeHTTP(httptest.NewRecorder(), request)
	cancel()

	userID, ok := middleware.UserIDFromContext(repositoryCtx)
	require.True(t, ok)
	require.Equal(t, routeUserID, userID)
	require.ErrorIs(t, repositoryCtx.Err(), context.Canceled)
	spans := recorder.Ended()
	httpSpan := spans[len(spans)-1]
	require.Equal(t, "GET /api/posts success", tracingtest.Results(spans)[len(spans)-1])
	require.Equal(t, httpSpan.SpanContext().TraceID(), trace.SpanContextFromContext(repositoryCtx).TraceID())
	require.Equal(t, codes.Unset, httpSpan.Status().Code)
}
