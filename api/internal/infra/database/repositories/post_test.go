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
	repo, _ := newTestPostRepository(t)
	ctx := context.Background()
	missingID := models.UUID("99999999-9999-4999-8999-999999999999")
	ownerID := models.UUID("11111111-1111-4111-8111-111111111111")
	post, err := repo.GetByID(ctx, missingID)
	require.ErrorIs(t, err, repositories.ErrPostNotFound)
	require.Nil(t, post)
	post, err = repo.GetByIDForOwner(ctx, ownerID, missingID)
	require.ErrorIs(t, err, repositories.ErrPostNotFound)
	require.Nil(t, post)
	err = repo.Update(ctx, ownerID, &models.Post{
		UUIDBaseModel: models.UUIDBaseModel{ID: missingID}, Title: "title", Content: "content",
	})
	require.ErrorIs(t, err, repositories.ErrPostNotFound)
}
