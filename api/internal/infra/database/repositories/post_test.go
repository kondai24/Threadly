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

func TestPostRepository_DeleteByIDSoftDeletesPostOnly(t *testing.T) {
	db := openTestDB(t)
	tx := db.Begin()
	require.NoError(t, tx.Error)
	t.Cleanup(func() {
		require.NoError(t, tx.Rollback().Error)
	})

	user := &models.User{Username: "post-repository-user", PasswordHash: "hash"}
	require.NoError(t, tx.Create(user).Error)
	post := &models.Post{AuthorID: user.ID, Title: "post", Content: "content"}
	require.NoError(t, tx.Create(post).Error)
	root := &models.Comment{PostID: post.ID, AuthorID: user.ID, Content: "root"}
	require.NoError(t, tx.Create(root).Error)
	reply := &models.Comment{
		PostID:   post.ID,
		AuthorID: user.ID,
		ParentID: &root.ID,
		Content:  "reply",
	}
	require.NoError(t, tx.Create(reply).Error)
	require.NoError(t, tx.Create(&models.PostLike{UserID: user.ID, PostID: post.ID}).Error)
	require.NoError(t, tx.Create(&models.CommentLike{UserID: user.ID, CommentID: root.ID}).Error)
	require.NoError(t, tx.Create(&models.CommentLike{UserID: user.ID, CommentID: reply.ID}).Error)

	repo := &PostRepository{DB: tx}
	rows, err := repo.DeleteByID(context.Background(), user.ID, post.ID)

	require.NoError(t, err)
	require.Equal(t, int64(1), rows)
	var activePost models.Post
	require.Error(t, tx.First(&activePost, post.ID).Error)
	var activeComments []models.Comment
	require.NoError(t, tx.Where("post_id = ?", post.ID).Find(&activeComments).Error)
	require.Len(t, activeComments, 2)

	var deletedPost models.Post
	require.NoError(t, tx.Unscoped().First(&deletedPost, post.ID).Error)
	require.True(t, deletedPost.DeletedAt.Valid)
	var postLikes []models.PostLike
	require.NoError(t, tx.Where("post_id = ?", post.ID).Find(&postLikes).Error)
	require.Len(t, postLikes, 1)
	var commentLikes []models.CommentLike
	require.NoError(t, tx.Where("comment_id IN ?", []models.UUID{root.ID, reply.ID}).Find(&commentLikes).Error)
	require.Len(t, commentLikes, 2)
}
