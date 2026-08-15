//go:build integration

package repository

import (
	"context"
	"testing"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newTestPostRepository(t *testing.T) (*PostRepository, *gorm.DB) {
	t.Helper()

	db := openTestDB(t)
	tx := db.Begin()
	require.NoError(t, tx.Error)
	t.Cleanup(func() {
		require.NoError(t, tx.Rollback().Error)
	})

	return &PostRepository{DB: tx}, tx
}

func TestPostRepository_ReturnsPostNotFound(t *testing.T) {
	t.Run("GetByIDで存在しないPostを取得する", func(t *testing.T) {
		repo, _ := newTestPostRepository(t)

		post, err := repo.GetByID(context.Background(), models.UUID("99999999-9999-4999-8999-999999999999"))

		require.ErrorIs(t, err, repositories.ErrPostNotFound)
		require.Nil(t, post)
	})

	t.Run("GetByIDForOwnerで存在しないPostを取得する", func(t *testing.T) {
		repo, _ := newTestPostRepository(t)

		post, err := repo.GetByIDForOwner(
			context.Background(),
			models.UUID("11111111-1111-4111-8111-111111111111"),
			models.UUID("99999999-9999-4999-8999-999999999999"),
		)

		require.ErrorIs(t, err, repositories.ErrPostNotFound)
		require.Nil(t, post)
	})

	t.Run("Updateで対象が存在しない場合", func(t *testing.T) {
		repo, _ := newTestPostRepository(t)

		err := repo.Update(
			context.Background(),
			models.UUID("11111111-1111-4111-8111-111111111111"),
			&models.Post{
				UUIDBaseModel: models.UUIDBaseModel{
					ID: models.UUID("99999999-9999-4999-8999-999999999999"),
				},
				Title:   "title",
				Content: "content",
			},
		)

		require.ErrorIs(t, err, repositories.ErrPostNotFound)
	})
}
