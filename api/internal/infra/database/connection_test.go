package database

import (
	"bytes"
	"errors"
	"log"
	"strings"
	"testing"
	"time"

	"Threadly/internal/domain/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestGORMLogger_HidesParametersOnErrorAndSlowQuery(t *testing.T) {
	tests := []struct {
		name          string
		slowThreshold time.Duration
		forceError    bool
		wantLog       string
	}{
		{
			name:          "DBエラー時にパラメータを秘匿する",
			slowThreshold: gormSlowThreshold,
			forceError:    true,
			wantLog:       "forced database error",
		},
		{
			name:          "slow SQL時にパラメータを秘匿する",
			slowThreshold: time.Nanosecond,
			wantLog:       "SLOW SQL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			db := openDryRunDB(t, &output, tt.slowThreshold)
			if tt.forceError {
				err := db.Callback().Create().After("gorm:create").Register(
					"test:force-error",
					func(tx *gorm.DB) {
						tx.AddError(errors.New("forced database error"))
					},
				)
				if err != nil {
					t.Fatalf("register error callback: %v", err)
				}
			}

			plainPassword := "sensitive-password-value"
			passwordHash := "$argon2id$v=19$m=65536,t=3,p=2$" + plainPassword
			result := db.Create(&models.User{
				Username:     "alice",
				PasswordHash: passwordHash,
			})
			if tt.forceError && result.Error == nil {
				t.Fatal("Create() error = nil, want forced database error")
			}

			logOutput := output.String()
			if strings.Contains(logOutput, plainPassword) {
				t.Fatalf("log exposes password: %s", logOutput)
			}
			if strings.Contains(logOutput, passwordHash) {
				t.Fatalf("log exposes password hash: %s", logOutput)
			}
			if !strings.Contains(logOutput, tt.wantLog) {
				t.Fatalf("log = %q, want marker %q", logOutput, tt.wantLog)
			}
			if !strings.Contains(logOutput, "?") {
				t.Fatalf("log = %q, want parameter placeholders", logOutput)
			}
		})
	}
}

func openDryRunDB(
	t *testing.T,
	output *bytes.Buffer,
	slowThreshold time.Duration,
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
				slowThreshold,
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
