package main

import (
	"Threadly/internal/di"
	"log"
	"os"

	"github.com/joho/godotenv"
)

// @title Thready API
// @version 1.0
func main() {
	// envファイルの読み込み
	_ = godotenv.Load()

	container, err := di.NewContainer()
	if err != nil {
		log.Fatalf("Failed to build container: %v", err)
	}

	r, err := di.ResolveRouter(container)
	if err != nil {
		log.Fatalf("Failed to resolve router: %v", err)
	}

	// Start the server
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
