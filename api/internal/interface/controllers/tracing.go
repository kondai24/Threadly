package controllers

import (
	"Threadly/internal/middleware"
	"Threadly/internal/tracing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
)

// startHandlerSpanは、Handler Spanを開始し、そのContextをc.Requestへ設定する。
// 既存のc.Request.Context()経路のまま、認証済みUser IDやcancelを保ったContextがUsecaseへ渡る。
func startHandlerSpan(c *gin.Context, tracer trace.Tracer, name string) trace.Span {
	ctx, span := tracing.Start(c.Request.Context(), tracer, tracing.LayerHandler, name)
	c.Request = c.Request.WithContext(ctx)
	return span
}

// endHandlerSpanは、Handlerが書き込んだ最終応答から結果を分類してSpanを終了する。
// panic時は応答前のため、statusを記録せずにpanic分類で終了して再送出する。
func endHandlerSpan(c *gin.Context, span trace.Span, recovered any) {
	status := c.Writer.Status()
	if recovered != nil {
		status = 0
		// HTTP SpanはRecovery後の500しか観測できないため、panicだったことを固定マーカーで伝える。
		middleware.MarkTracePanic(c)
	}
	tracing.SetHTTPAttributes(span, c.Request.Method, c.FullPath(), status)
	tracing.End(span, tracing.HTTPResult(status), recovered)
}
