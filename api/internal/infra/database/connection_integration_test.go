//go:build integration

package database

import (
	"os"
	"testing"

	"Threadly/internal/domain/models"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestMigration_CreatesUserAndPostSchema(t *testing.T) {
	db := openIntegrationDB(t)
	migrator := db.Migrator()

	require.True(t, migrator.HasTable(&models.User{}))
	require.True(t, migrator.HasTable(&models.Post{}))
	require.True(t, migrator.HasIndex(&models.User{}, "idx_users_username"))
	require.True(t, migrator.HasConstraint(&models.Post{}, "Author"))
}

func TestMigration_EnforcesUserAndPostConstraints(t *testing.T) {
	t.Run("usernameのunique制約を検証する", func(t *testing.T) {
		tx := newIntegrationTransaction(t)
		first := &models.User{Username: "schema-user", PasswordHash: "first-hash"}
		require.NoError(t, tx.Create(first).Error)

		duplicate := &models.User{Username: "schema-user", PasswordHash: "second-hash"}
		require.ErrorIs(t, tx.Create(duplicate).Error, gorm.ErrDuplicatedKey)
	})

	t.Run("Postの外部キー制約を検証する", func(t *testing.T) {
		tx := newIntegrationTransaction(t)
		post := &models.Post{
			AuthorID: ^uint(0),
			Title:    "orphan post",
			Content:  "must be rejected",
		}

		require.ErrorIs(t, tx.Create(post).Error, gorm.ErrForeignKeyViolated)
	})
}

func openIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_DSN must be set for integration tests")
	}
	db, err := gorm.Open(mysql.Open(dsn), NewGORMConfig())
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, sqlDB.Close())
	})
	return db
}

func newIntegrationTransaction(t *testing.T) *gorm.DB {
	t.Helper()

	db := openIntegrationDB(t)
	tx := db.Begin()
	require.NoError(t, tx.Error)
	t.Cleanup(func() {
		require.NoError(t, tx.Rollback().Error)
	})
	return tx
}
