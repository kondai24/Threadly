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
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Post{}))

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

func TestUserRepository_FindByUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		wantUser bool
		wantErr  error
		seedUser *models.User
	}{
		{
			name:     "登録済みusernameでUserを取得する",
			username: "alice",
			wantUser: true,
			seedUser: &models.User{Username: "alice", PasswordHash: "hash"},
		},
		{
			name:     "存在しないusernameはNotFoundを返す",
			username: "nobody",
			wantErr:  repositories.ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, db := newTestUserRepository(t)
			if tt.seedUser != nil {
				require.NoError(t, db.Create(tt.seedUser).Error)
			}

			user, err := repo.FindByUsername(context.Background(), tt.username)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, user)
				return
			}
			require.NoError(t, err)
			require.True(t, tt.wantUser)
			require.NotNil(t, user)
			require.Equal(t, tt.seedUser.Username, user.Username)
			require.Equal(t, tt.seedUser.PasswordHash, user.PasswordHash)
		})
	}
}

func TestUserRepository_FindByID(t *testing.T) {
	t.Run("登録済みIDでUserを取得する", func(t *testing.T) {
		repo, db := newTestUserRepository(t)
		seedUser := &models.User{Username: "alice", PasswordHash: "hash"}
		require.NoError(t, db.Create(seedUser).Error)

		user, err := repo.FindByID(context.Background(), seedUser.ID)

		require.NoError(t, err)
		require.NotNil(t, user)
		require.Equal(t, seedUser.ID, user.ID)
		require.Equal(t, seedUser.Username, user.Username)
	})

	t.Run("存在しないIDはNotFoundを返す", func(t *testing.T) {
		repo, _ := newTestUserRepository(t)

		user, err := repo.FindByID(context.Background(), 999999)

		require.ErrorIs(t, err, repositories.ErrUserNotFound)
		require.Nil(t, user)
	})
}

func TestUserRepository_Create(t *testing.T) {
	t.Run("Userを作成する", func(t *testing.T) {
		repo, db := newTestUserRepository(t)
		user := &models.User{Username: "alice", PasswordHash: "hash"}

		err := repo.Create(context.Background(), user)

		require.NoError(t, err)
		require.NotZero(t, user.ID)

		var stored models.User
		require.NoError(t, db.First(&stored, user.ID).Error)
		require.Equal(t, user.Username, stored.Username)
		require.Equal(t, user.PasswordHash, stored.PasswordHash)
	})

	t.Run("重複usernameはAlreadyExistsを返す", func(t *testing.T) {
		repo, db := newTestUserRepository(t)
		first := &models.User{Username: "alice", PasswordHash: "first-hash"}
		require.NoError(t, db.Create(first).Error)

		duplicate := &models.User{Username: "alice", PasswordHash: "second-hash"}
		err := repo.Create(context.Background(), duplicate)

		require.ErrorIs(t, err, repositories.ErrUsernameAlreadyExists)
	})
}

func TestUserRepository_UsesCanceledContext(t *testing.T) {
	repo, _ := newTestUserRepository(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	user, err := repo.FindByUsername(ctx, "alice")

	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, user)
}

func TestUserRepository_WrapsDatabaseError(t *testing.T) {
	db := openTestDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	repo := &UserRepository{DB: db}
	user, err := repo.FindByUsername(context.Background(), "alice")

	require.Error(t, err)
	require.ErrorContains(t, err, "find user by username")
	require.Nil(t, user)
}
