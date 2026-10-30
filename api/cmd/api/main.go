package main

import (
	"Threadly/internal/di"
	"Threadly/internal/infra/observability"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

const (
	httpShutdownTimeout  = 5 * time.Second
	traceShutdownTimeout = 5 * time.Second
)

// traceShutdownerは、起動処理が所有するTrace Providerの終了処理を表す。
type traceShutdowner interface {
	Shutdown(ctx context.Context) error
}

// @title Thready API
// @version 1.0
// @description Threadly learning API with username/password authentication and authenticated post browsing.
// @securityDefinitions.apikey SessionCookie
// @in header
// @name Cookie
// @description HttpOnly, Secure, SameSite session cookie.
func main() {
	// 環境変数ファイルを読み込む。Trace設定も含め、以降の初期化はこの値を使う。
	_ = godotenv.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Getenv)
	stop()
	// log.Fatalはdeferを実行しないため、run内のcleanupを終えてから終了statusを決める。
	if err != nil {
		log.Printf("Failed to run server: %v", err)
		os.Exit(1)
	}
}

// runは、Trace初期化→DI構築→Router解決→HTTP開始の順に起動し、ctxの終了でHTTPを停止する。
// 起動途中で失敗した場合も、生成済みのTrace Providerを終了してから戻る。
func run(ctx context.Context, getenv func(string) string) error {
	traceRuntime, err := observability.NewTraceRuntime(ctx, getenv)
	if err != nil {
		return fmt.Errorf("initialize trace: %w", err)
	}
	defer shutdownTrace(traceRuntime)

	container, err := di.NewContainer(getenv, traceRuntime.Tracer())
	if err != nil {
		return fmt.Errorf("build container: %w", err)
	}
	router, err := di.ResolveRouter(container)
	if err != nil {
		return fmt.Errorf("resolve router: %w", err)
	}

	port := getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	log.Println("Server started on port " + addr)
	return serve(ctx, &http.Server{Handler: router}, listener)
}

// serveは、ctxが終了するまでHTTPを提供し、受付停止後に最大httpShutdownTimeoutだけ処理中requestを待つ。
func serve(ctx context.Context, server *http.Server, listener net.Listener) error {
	serverErr := make(chan error, 1)
	go func() { serverErr <- server.Serve(listener) }()

	select {
	case err := <-serverErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve http: %w", err)
	case <-ctx.Done():
	}

	// signalで既にcancel済みのctxではなく、終了猶予専用のContextで停止する。
	shutdownCtx, cancel := context.WithTimeout(context.Background(), httpShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}
	return nil
}

// shutdownTraceは、キャンセル済みのrequest/signal Contextとは別の有限Contextで未送信Spanを送る。
// 失敗しても終了を無期限に延ばさず、固定の分類だけを報告する。
func shutdownTrace(runtime traceShutdowner) {
	ctx, cancel := context.WithTimeout(context.Background(), traceShutdownTimeout)
	defer cancel()
	if err := runtime.Shutdown(ctx); err != nil {
		log.Print("trace shutdown_failed")
	}
}
