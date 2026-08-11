package main

import (
	"Threadly/internal/di"
	"log"
	"os"

	"github.com/joho/godotenv"
)

// @title Thready API
// @version 1.0
// @description Threadly learning API with username/password authentication and authenticated post browsing.
// @securityDefinitions.apikey SessionCookie
// @in header
// @name Cookie
// @description HttpOnly, Secure, SameSite session cookie.
func main() {
	// 環境変数ファイルを読み込む。
	_ = godotenv.Load()

	container, err := di.NewContainer()
	if err != nil {
		log.Fatalf("Failed to build container: %v", err)
	}

	r, err := di.ResolveRouter(container)
	if err != nil {
		log.Fatalf("Failed to resolve router: %v", err)
	}

	// サーバーを起動する。
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port

	log.Println("Server started on port " + addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}
