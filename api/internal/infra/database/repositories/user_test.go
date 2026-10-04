//go:build integration

package repository

import (
	"context"
	"os"
	"testing"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"
	"Threadly/internal/infra/database"

	"github.com/stretchr/testify/require"
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

	return &UserRepository{DB: tx}, tx
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

	user, err := repo.FindByUsername(ctx, "alice")

	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, user)
}
