//go:build integration

package repository

import (
	"context"
	"testing"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"
	"Threadly/internal/tracing/tracingtest"

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

	return &PostRepository{DB: tx, Tracer: noopTracer()}, tx
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

func TestPostRepository_RecordsOneSpanPerOperation(t *testing.T) {
	repo, tx := newTestPostRepository(t)
	tracer, recorder := tracingtest.NewRecorder()
	repo.Tracer = tracer
	ctx := context.Background()
	user := &models.User{Username: "post-span-user", PasswordHash: "hash"}
	require.NoError(t, tx.Create(user).Error)
	post := &models.Post{AuthorID: user.ID, Title: "title", Content: "content"}

	require.NoError(t, repo.Create(ctx, post))
	_, err := repo.GetByID(ctx, post.ID)
	require.NoError(t, err)
	_, err = repo.GetByIDForUpdate(ctx, post.ID)
	require.NoError(t, err)
	_, err = repo.GetByIDForOwner(ctx, user.ID, post.ID)
	require.NoError(t, err)
	require.NoError(t, repo.Update(ctx, user.ID, post))
	_, err = repo.ListAll(ctx)
	require.NoError(t, err)
	_, err = repo.DeleteByID(ctx, user.ID, post.ID)
	require.NoError(t, err)
	_, err = repo.GetByID(ctx, post.ID)
	require.ErrorIs(t, err, repositories.ErrPostNotFound)

	require.Equal(t, []string{
		"PostRepository.Create success",
		"PostRepository.GetByID success",
		"PostRepository.GetByIDForUpdate success",
		"PostRepository.GetByIDForOwner success",
		"PostRepository.Update success",
		"PostRepository.ListAll success",
		"PostRepository.DeleteByID success",
		"PostRepository.GetByID not_found",
	}, tracingtest.Results(recorder.Ended()))
}
