//go:build integration

package repository

import (
	"context"
	"testing"

	"Threadly/internal/domain/models"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func seedLikeRepositoryData(t *testing.T, db *gorm.DB) (*models.User, *models.Post, *models.Comment) {
	t.Helper()

	user := &models.User{Username: "like-repository-user", PasswordHash: "hash"}
	require.NoError(t, db.Create(user).Error)
	post := &models.Post{AuthorID: user.ID, Title: "post", Content: "content"}
	require.NoError(t, db.Create(post).Error)
	comment := &models.Comment{PostID: post.ID, AuthorID: user.ID, Content: "comment"}
	require.NoError(t, db.Create(comment).Error)
	return user, post, comment
}

func TestPostLikeRepository_IsIdempotentAndSupportsRelike(t *testing.T) {
	db := openTestDB(t)
	tx := db.Begin()
	require.NoError(t, tx.Error)
	t.Cleanup(func() { require.NoError(t, tx.Rollback().Error) })

	user, post, _ := seedLikeRepositoryData(t, tx)
	repo := &PostLikeRepository{DB: tx}

	require.NoError(t, repo.Ensure(context.Background(), user.ID, post.ID))
	require.NoError(t, repo.Ensure(context.Background(), user.ID, post.ID))

	var rows int64
	require.NoError(t, tx.Model(&models.PostLike{}).Where("user_id = ? AND post_id = ?", user.ID, post.ID).Count(&rows).Error)
	require.Equal(t, int64(1), rows)

	counts, err := repo.CountByPostIDs(context.Background(), []models.UUID{post.ID})
	require.NoError(t, err)
	require.Equal(t, int64(1), counts[post.ID])
	likedIDs, err := repo.FindLikedPostIDs(context.Background(), user.ID, []models.UUID{post.ID})
	require.NoError(t, err)
	_, liked := likedIDs[post.ID]
	require.True(t, liked)

	require.NoError(t, repo.Delete(context.Background(), user.ID, post.ID))
	require.NoError(t, repo.Delete(context.Background(), user.ID, post.ID))
	require.NoError(t, repo.Ensure(context.Background(), user.ID, post.ID))
	require.NoError(t, tx.Model(&models.PostLike{}).Where("user_id = ? AND post_id = ?", user.ID, post.ID).Count(&rows).Error)
	require.Equal(t, int64(1), rows)
	require.NoError(t, repo.DeleteByPostID(context.Background(), post.ID))
	require.NoError(t, tx.Model(&models.PostLike{}).Where("post_id = ?", post.ID).Count(&rows).Error)
	require.Zero(t, rows)
}

func TestCommentLikeRepository_IsIdempotentAndSupportsRelike(t *testing.T) {
	db := openTestDB(t)
	tx := db.Begin()
	require.NoError(t, tx.Error)
	t.Cleanup(func() { require.NoError(t, tx.Rollback().Error) })

	user, _, comment := seedLikeRepositoryData(t, tx)
	repo := &CommentLikeRepository{DB: tx}

	require.NoError(t, repo.Ensure(context.Background(), user.ID, comment.ID))
	require.NoError(t, repo.Ensure(context.Background(), user.ID, comment.ID))

	counts, err := repo.CountByCommentIDs(context.Background(), []models.UUID{comment.ID})
	require.NoError(t, err)
	require.Equal(t, int64(1), counts[comment.ID])
	likedIDs, err := repo.FindLikedCommentIDs(context.Background(), user.ID, []models.UUID{comment.ID})
	require.NoError(t, err)
	_, liked := likedIDs[comment.ID]
	require.True(t, liked)

	require.NoError(t, repo.Delete(context.Background(), user.ID, comment.ID))
	require.NoError(t, repo.Ensure(context.Background(), user.ID, comment.ID))
	require.NoError(t, repo.DeleteByCommentIDs(context.Background(), []models.UUID{comment.ID}))
	var rows int64
	require.NoError(t, tx.Model(&models.CommentLike{}).Where("user_id = ? AND comment_id = ?", user.ID, comment.ID).Count(&rows).Error)
	require.Equal(t, int64(1), rows)
}
