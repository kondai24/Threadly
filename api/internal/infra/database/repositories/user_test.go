//go:build integration

package repository

import (
	"context"
	"os"
	"testing"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"
	"Threadly/internal/infra/database"
	"Threadly/internal/tracing/tracingtest"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := testDatabaseDSN()
	if dsn == "" {
		t.Fatal("TEST_DATABASE_DSN must be set for integration tests")
	}

	db, err := gorm.Open(mysql.Open(dsn), database.NewGORMConfig())
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	return db
}

func testDatabaseDSN() string {
	return os.Getenv("TEST_DATABASE_DSN")
}

func newTestUserRepository(t *testing.T) (*UserRepository, *gorm.DB) {
	t.Helper()

	db := openTestDB(t)
	tx := db.Begin()
	require.NoError(t, tx.Error)
	t.Cleanup(func() {
		_ = tx.Rollback().Error
	})

	return &UserRepository{DB: tx, Tracer: noopTracer()}, tx
}

// noopTracerは、Span記録を検証しないテストで明示的に使う無記録Tracer。
func noopTracer() trace.Tracer {
	return noop.NewTracerProvider().Tracer("test")
}

func TestUserRepository_RecordsOneSpanPerOperation(t *testing.T) {
	repo, _ := newTestUserRepository(t)
	tracer, recorder := tracingtest.NewRecorder()
	repo.Tracer = tracer
	ctx := context.Background()
	user := &models.User{Username: "span_alice", PasswordHash: "hash"}

	require.NoError(t, repo.Create(ctx, user))
	require.ErrorIs(t, repo.Create(ctx, &models.User{Username: user.Username, PasswordHash: "hash"}), repositories.ErrUsernameAlreadyExists)
	_, err := repo.FindByUsername(ctx, user.Username)
	require.NoError(t, err)
	_, err = repo.FindByID(ctx, models.UUID("99999999-9999-4999-8999-999999999999"))
	require.ErrorIs(t, err, repositories.ErrUserNotFound)

	spans := recorder.Ended()
	require.Equal(t, []string{
		"UserRepository.Create success",
		"UserRepository.Create conflict",
		"UserRepository.FindByUsername success",
		"UserRepository.FindByID not_found",
	}, tracingtest.Results(spans))
	for _, span := range spans {
		require.Equal(t, trace.SpanKindInternal, span.SpanKind())
		require.Equal(t, "repository", tracingtest.Attribute(span, "threadly.layer"))
		require.Equal(t, codes.Unset, span.Status().Code)
	}
}

func TestUserRepository_CreateAndFind(t *testing.T) {
	repo, _ := newTestUserRepository(t)
	ctx := context.Background()
	user := &models.User{Username: "alice", PasswordHash: "hash"}
	require.NoError(t, repo.Create(ctx, user))
	_, err := models.ParseUUID(string(user.ID))
	require.NoError(t, err)
	byName, err := repo.FindByUsername(ctx, user.Username)
	require.NoError(t, err)
	require.Equal(t, user.ID, byName.ID)
	require.Equal(t, user.Username, byName.Username)
	require.Equal(t, user.PasswordHash, byName.PasswordHash)
	byID, err := repo.FindByID(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, byName, byID)
	duplicate := &models.User{Username: user.Username, PasswordHash: "second-hash"}
	require.ErrorIs(t, repo.Create(ctx, duplicate), repositories.ErrUsernameAlreadyExists)
}

func TestUserRepository_MissingUser(t *testing.T) {
	repo, _ := newTestUserRepository(t)
	ctx := context.Background()
	byName, err := repo.FindByUsername(ctx, "nobody")
	require.ErrorIs(t, err, repositories.ErrUserNotFound)
	require.Nil(t, byName)
	byID, err := repo.FindByID(ctx, models.UUID("99999999-9999-4999-8999-999999999999"))
	require.ErrorIs(t, err, repositories.ErrUserNotFound)
	require.Nil(t, byID)
}

func TestUserRepository_UsesCanceledContext(t *testing.T) {
	repo, _ := newTestUserRepository(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tracer, recorder := tracingtest.NewRecorder()
	repo.Tracer = tracer

	user, err := repo.FindByUsername(ctx, "alice")

	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, user)
	// context cancellationは想定内の業務結果ではないため、Repositoryの障害として記録する。
	spans := recorder.Ended()
	require.Equal(t, []string{"UserRepository.FindByUsername internal_error"}, tracingtest.Results(spans))
	require.Equal(t, codes.Error, spans[0].Status().Code)
}
