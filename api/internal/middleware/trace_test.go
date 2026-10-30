package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"Threadly/internal/tracing/tracingtest"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"
)

func TestTraceHTTPEndsSpanAndRepanicsWhenPanicEscapes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tracer, recorder := tracingtest.NewRecorder()
	router := gin.New()
	// Recoveryを外側に置かない構成でも、HTTP Spanを終了してから同じpanicを上位へ戻す。
	router.Use(TraceHTTP(tracer))
	router.GET("/panic", func(*gin.Context) { panic("dummy-secret-value") })

	require.PanicsWithValue(t, "dummy-secret-value", func() {
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/panic", nil))
	})

	spans := recorder.Ended()
	require.Equal(t, []string{"GET /panic panic"}, tracingtest.Results(spans))
	require.Equal(t, codes.Error, spans[0].Status().Code)
	require.Empty(t, tracingtest.Attribute(spans[0], "http.response.status_code"))
}

func TestTraceHTTPRecordsHandlerPanicMarker(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tracer, recorder := tracingtest.NewRecorder()
	router := gin.New()
	router.Use(TraceHTTP(tracer))
	router.GET("/marked", func(c *gin.Context) {
		MarkTracePanic(c)
		c.Status(http.StatusInternalServerError)
	})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/marked", nil))

	spans := recorder.Ended()
	require.Equal(t, []string{"GET /marked panic"}, tracingtest.Results(spans))
	require.Equal(t, "500", tracingtest.Attribute(spans[0], "http.response.status_code"))
}
