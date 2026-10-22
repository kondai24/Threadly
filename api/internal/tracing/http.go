package tracing

import (
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	httpMethodKey = attribute.Key("http.request.method")
	httpRouteKey  = attribute.Key("http.route")
	httpStatusKey = attribute.Key("http.response.status_code")
)

// HTTPResultは、HTTP・Handlerの最終応答statusを固定分類へ変換する。
// 5xxだけを想定外障害とし、4xxは入力・認証・認可などの想定内応答として扱う。
func HTTPResult(status int) Result {
	switch {
	case status >= http.StatusInternalServerError:
		return ResultInternalError
	case status == http.StatusBadRequest:
		return ResultInvalidInput
	case status == http.StatusUnauthorized:
		return ResultUnauthenticated
	case status == http.StatusForbidden:
		return ResultForbidden
	case status == http.StatusNotFound:
		return ResultNotFound
	case status == http.StatusConflict:
		return ResultConflict
	case status >= http.StatusBadRequest:
		return ResultRejected
	default:
		return ResultSuccess
	}
}

// HTTPMethodは、標準method以外を_OTHERへ正規化し、Span名と属性の値を有限にする。
func HTTPMethod(method string) string {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch,
		http.MethodDelete, http.MethodConnect, http.MethodOptions, http.MethodTrace:
		return method
	default:
		return "_OTHER"
	}
}

// SetHTTPAttributesは、method、解決済みroute、最終statusを記録する。
// routeは未一致時に空となるため記録せず、raw pathの代わりに使わない。statusが0なら応答前として省く。
func SetHTTPAttributes(span trace.Span, method string, route string, status int) {
	span.SetAttributes(httpMethodKey.String(HTTPMethod(method)))
	if route != "" {
		span.SetAttributes(httpRouteKey.String(route))
	}
	if status > 0 {
		span.SetAttributes(httpStatusKey.Int(status))
	}
}
