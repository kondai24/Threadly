package middleware

import (
	"Threadly/internal/tracing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// tracePanicKeyは、Handlerがpanicを記録したことをHTTP Spanへ伝えるgin Contextのkey。
const tracePanicKey = "threadly.trace.panic"

// TraceHTTPは、認証拒否やRecovery後の500を含むMiddleware以降のHTTP処理全体を1本のSpanにする。
// gin.Recoveryより外側に登録し、Recoveryが書いた最終statusを記録する。
func TraceHTTP(tracer trace.Tracer) gin.HandlerFunc {
	// baggageは扱わず、W3C TraceContextだけを親として受け入れる。
	propagator := propagation.TraceContext{}
	return func(c *gin.Context) {
		ctx := propagator.Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))
		// ginはMiddlewareの実行前にrouteを解決するため、ここでroute templateから名前を決められる。
		route := c.FullPath()
		ctx, span := tracing.StartServer(ctx, tracer, httpSpanName(c.Request.Method, route))
		c.Request = c.Request.WithContext(ctx)
		// 後続が差し替えたc.Requestではなく、開始時に保持したHTTP Span自体を更新する。
		// Recoveryより外へpanicが抜けた場合も、Spanを終了してから同じpanicを再送出する。
		defer func() {
			recovered := recover()
			status := c.Writer.Status()
			if recovered != nil {
				// 応答を書く前に抜けたpanicでは、既定の200を最終statusとして記録しない。
				status = 0
			}
			tracing.SetHTTPAttributes(span, c.Request.Method, route, status)
			result := tracing.HTTPResult(status)
			if c.GetBool(tracePanicKey) {
				result = tracing.ResultPanic
			}
			tracing.End(span, result, recovered)
		}()

		c.Next()
	}
}

// MarkTracePanicは、Handlerで発生したpanicをHTTP Spanの結果にも反映させる。
func MarkTracePanic(c *gin.Context) {
	c.Set(tracePanicKey, true)
}

// httpSpanNameは、raw pathやIDを含めず、標準methodとroute templateだけでSpan名を作る。
func httpSpanName(method string, route string) string {
	if route == "" {
		route = "unmatched"
	}
	return tracing.HTTPMethod(method) + " " + route
}
