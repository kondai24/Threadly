package main

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func envOf(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestRun_RejectsInvalidTraceConfigBeforeStartingServer(t *testing.T) {
	err := run(context.Background(), envOf(map[string]string{"OTEL_SDK_DISABLED": "dummy-secret-value"}))

	require.ErrorContains(t, err, "OTEL_SDK_DISABLED")
	require.NotContains(t, err.Error(), "dummy-secret-value")
}

type recordingShutdowner struct {
	err       error
	calls     int
	ctxErr    error
	remaining time.Duration
}

func (s *recordingShutdowner) Shutdown(ctx context.Context) error {
	s.calls++
	s.ctxErr = ctx.Err()
	deadline, _ := ctx.Deadline()
	s.remaining = time.Until(deadline)
	return s.err
}

func listenLocal(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	return listener
}

func TestServe_StopsAcceptingWhenContextIsCanceled(t *testing.T) {
	listener := listenLocal(t)
	addr := listener.Addr().String()
	ctx, cancel := context.WithCancel(context.Background())
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})}
	done := make(chan error, 1)
	go func() { done <- serve(ctx, server, listener) }()

	response, err := http.Get("http://" + addr)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusNoContent, response.StatusCode)
	cancel()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(httpShutdownTimeout + time.Second):
		t.Fatal("serve did not return after context cancellation")
	}
	_, err = http.Get("http://" + addr)
	require.Error(t, err)
}

func TestServe_ReturnsServerError(t *testing.T) {
	listener := listenLocal(t)
	require.NoError(t, listener.Close())

	err := serve(context.Background(), &http.Server{Handler: http.NotFoundHandler()}, listener)

	require.ErrorContains(t, err, "serve http")
}

func TestShutdownTrace_UsesFreshBoundedContextAndReportsFixedFailure(t *testing.T) {
	var output bytes.Buffer
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	shutdowner := &recordingShutdowner{err: errors.New("dummy-secret-value")}

	shutdownTrace(shutdowner)

	require.Equal(t, 1, shutdowner.calls)
	require.NoError(t, shutdowner.ctxErr)
	require.Greater(t, shutdowner.remaining, time.Duration(0))
	require.LessOrEqual(t, shutdowner.remaining, traceShutdownTimeout)
	require.Contains(t, output.String(), "trace shutdown_failed")
	require.NotContains(t, output.String(), "dummy-secret-value")
}

func TestRun_ReturnsStartupFailureAfterTraceInitWithinDeadline(t *testing.T) {
	unreachable := listenLocal(t)
	unreachableAddr := unreachable.Addr().(*net.TCPAddr)
	require.NoError(t, unreachable.Close())
	// 設定検証は通し、接続できないDBを指定してDB接続の段階で起動を失敗させる。
	env := envOf(map[string]string{
		"DB_HOST":                     "127.0.0.1",
		"DB_PORT":                     strconv.Itoa(unreachableAddr.Port),
		"DB_USER":                     "test",
		"DB_PASSWORD":                 "test",
		"DB":                          "test",
		"OTEL_TRACES_EXPORTER":        "otlp",
		"OTEL_SERVICE_NAME":           "threadly-api",
		"OTEL_EXPORTER_OTLP_ENDPOINT": "http://" + unreachable.Addr().String(),
		"OTEL_EXPORTER_OTLP_PROTOCOL": "grpc",
	})

	started := time.Now()
	err := run(context.Background(), env)

	require.ErrorContains(t, err, "resolve router")
	require.NotContains(t, err.Error(), "database config")
	require.Less(t, time.Since(started), traceShutdownTimeout)
}
