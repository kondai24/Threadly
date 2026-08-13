package main

import (
	"io"
	"log"
	"os"

	"Threadly/internal/domain/models"

	"ariga.io/atlas-provider-gorm/gormschema"
)

func main() {
	stmts, err := gormschema.New("mysql").Load(
		&models.User{},
		&models.Post{},
	)
	if err != nil {
		log.Fatalf("failed to load GORM schema: %v", err)
	}
	if _, err := io.WriteString(os.Stdout, stmts); err != nil {
		log.Fatalf("failed to write GORM schema: %v", err)
	}
}
