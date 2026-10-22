package tracing

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTTPResult_ClassifiesFinalStatus(t *testing.T) {
	cases := map[int]Result{
		http.StatusOK:                  ResultSuccess,
		http.StatusNoContent:           ResultSuccess,
		http.StatusBadRequest:          ResultInvalidInput,
		http.StatusUnauthorized:        ResultUnauthenticated,
		http.StatusForbidden:           ResultForbidden,
		http.StatusNotFound:            ResultNotFound,
		http.StatusConflict:            ResultConflict,
		http.StatusMethodNotAllowed:    ResultRejected,
		http.StatusTooManyRequests:     ResultRejected,
		http.StatusInternalServerError: ResultInternalError,
		http.StatusServiceUnavailable:  ResultInternalError,
	}
	for status, want := range cases {
		require.Equal(t, want, HTTPResult(status), "status %d", status)
	}
}

func TestHTTPMethod_NormalizesUnknownMethods(t *testing.T) {
	require.Equal(t, http.MethodPatch, HTTPMethod(http.MethodPatch))
	require.Equal(t, "_OTHER", HTTPMethod("DUMMY-SECRET"))
	require.Equal(t, "_OTHER", HTTPMethod("get"))
}
