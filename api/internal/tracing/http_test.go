package tracing

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTTPResult_ClassifiesFinalStatus(t *testing.T) {
	cases := []struct {
		name   string
		status int
		want   Result
	}{
		{name: "2xxはsuccess", status: http.StatusNoContent, want: ResultSuccess},
		{name: "400はinvalid_input", status: http.StatusBadRequest, want: ResultInvalidInput},
		{name: "401はunauthenticated", status: http.StatusUnauthorized, want: ResultUnauthenticated},
		{name: "403はforbidden", status: http.StatusForbidden, want: ResultForbidden},
		{name: "404はnot_found", status: http.StatusNotFound, want: ResultNotFound},
		{name: "409はconflict", status: http.StatusConflict, want: ResultConflict},
		{name: "その他の4xxはrejected", status: http.StatusTooManyRequests, want: ResultRejected},
		{name: "5xxはinternal_error", status: http.StatusServiceUnavailable, want: ResultInternalError},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, HTTPResult(tt.status))
		})
	}
}

func TestHTTPMethod_NormalizesUnknownMethods(t *testing.T) {
	cases := []struct {
		name   string
		method string
		want   string
	}{
		{name: "標準methodは維持する", method: http.MethodPatch, want: http.MethodPatch},
		{name: "未知methodは_OTHERにする", method: "DUMMY-SECRET", want: "_OTHER"},
		{name: "小文字のmethodも標準外として扱う", method: "get", want: "_OTHER"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, HTTPMethod(tt.method))
		})
	}
}
