package database

import (
	"bytes"
	"errors"
	"log"
	"strings"
	"testing"

	"Threadly/internal/domain/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestGORMLogger_HidesParametersOnError(t *testing.T) {
	var output bytes.Buffer
	db := openDryRunDB(t, &output)
	expectedErr := errors.New("forced database error")
	err := db.Callback().Create().After("gorm:create").Register(
		"test:force-error",
		func(tx *gorm.DB) { _ = tx.AddError(expectedErr) },
	)
	if err != nil {
		t.Fatalf("register callback: %v", err)
	}
	passwordHash := "sensitive-hash-value"
	result := db.Create(&models.User{Username: "alice", PasswordHash: passwordHash})
	if !errors.Is(result.Error, expectedErr) {
		t.Fatalf("Create() error = %v, want forced error", result.Error)
	}
	logOutput := output.String()
	if strings.Contains(logOutput, passwordHash) {
		t.Fatal("log exposes a bound parameter")
	}
	if !strings.Contains(logOutput, expectedErr.Error()) || !strings.Contains(logOutput, "?") {
		t.Fatalf("log = %q, want error marker and parameter placeholders", logOutput)
	}
}

func openDryRunDB(
	t *testing.T,
	output *bytes.Buffer,
) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(
		mysql.New(mysql.Config{
			DSN:                       "test:test@tcp(127.0.0.1:3306)/test?parseTime=True",
			SkipInitializeWithVersion: true,
		}),
		&gorm.Config{
			Logger: newGORMLogger(
				log.New(output, "", 0),
				gormSlowThreshold,
			),
			DryRun:                 true,
			DisableAutomaticPing:   true,
			SkipDefaultTransaction: true,
		},
	)
	if err != nil {
		t.Fatalf("open dry-run database: %v", err)
	}
	return db
}
