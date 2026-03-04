package main

import (
	"Threadly/internal/infra/database"
	repository "Threadly/internal/infra/database/repositories"
	"Threadly/internal/infra/http/routes"
	"Threadly/internal/interface/controllers"
	"Threadly/internal/usecase/services"
	"log"
	"os"

	"github.com/joho/godotenv"
)

// @title Thready API
// @version 1.0
func main() {
	// envファイルの読み込み
	_ = godotenv.Load()
	db, err := database.ConnectionDB()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// DI setup: db → repositories → controllers → routes → server
	postRepo := repository.NewPostRepository(db)

	postSvc := services.NewPostService(postRepo)

	postCtl := controllers.NewPostController(postSvc)

	handler := routes.Handlers{Post: postCtl}
	r := routes.NewRouter(handler)

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
