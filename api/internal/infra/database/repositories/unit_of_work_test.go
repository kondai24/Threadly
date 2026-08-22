//go:build integration

package repository

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"

	"github.com/stretchr/testify/require"
)

func TestUnitOfWorkRollsBackCrossRepositoryDeletesTogether(t *testing.T) {
	db := openTestDB(t)
	user := &models.User{
		Username:     "uow-rollback-" + strconv.FormatInt(time.Now().UnixNano(), 10),
		PasswordHash: "hash",
	}
	require.NoError(t, db.Create(user).Error)

	post := &models.Post{
		AuthorID: user.ID,
		Title:    "post",
		Content:  "content",
	}
	require.NoError(t, db.Create(post).Error)

	comment := &models.Comment{
		PostID:   post.ID,
		AuthorID: user.ID,
		Content:  "comment",
	}
	require.NoError(t, db.Create(comment).Error)
	require.NoError(t, db.Create(&models.PostLike{UserID: user.ID, PostID: post.ID}).Error)
	require.NoError(t, db.Create(&models.CommentLike{UserID: user.ID, CommentID: comment.ID}).Error)

	t.Cleanup(func() {
		require.NoError(t, db.Unscoped().Where("comment_id = ?", comment.ID).Delete(&models.CommentLike{}).Error)
		require.NoError(t, db.Unscoped().Where("post_id = ?", post.ID).Delete(&models.PostLike{}).Error)
		require.NoError(t, db.Unscoped().Where("id = ?", comment.ID).Delete(&models.Comment{}).Error)
		require.NoError(t, db.Unscoped().Where("id = ?", post.ID).Delete(&models.Post{}).Error)
		require.NoError(t, db.Unscoped().Where("id = ?", user.ID).Delete(&models.User{}).Error)
	})

	expectedErr := errors.New("rollback transaction")
	uow := NewUnitOfWork(db)

	err := uow.WithinTransaction(context.Background(), func(tx repositories.TransactionRepositories) error {
		if err := tx.CommentLike.DeleteByCommentIDs(context.Background(), []models.UUID{comment.ID}); err != nil {
			return err
		}
		if err := tx.PostLike.DeleteByPostID(context.Background(), post.ID); err != nil {
			return err
		}
		if _, err := tx.Comment.DeleteByPostID(context.Background(), post.ID); err != nil {
			return err
		}
		if _, err := tx.Post.DeleteByID(context.Background(), user.ID, post.ID); err != nil {
			return err
		}
		return expectedErr
	})

	require.ErrorIs(t, err, expectedErr)
	var storedPost models.Post
	require.NoError(t, db.Unscoped().First(&storedPost, post.ID).Error)
	require.False(t, storedPost.DeletedAt.Valid)
	var storedComment models.Comment
	require.NoError(t, db.Unscoped().First(&storedComment, comment.ID).Error)
	require.False(t, storedComment.DeletedAt.Valid)
	var postLikeCount int64
	require.NoError(t, db.Model(&models.PostLike{}).Where("post_id = ?", post.ID).Count(&postLikeCount).Error)
	require.Equal(t, int64(1), postLikeCount)
	var commentLikeCount int64
	require.NoError(t, db.Model(&models.CommentLike{}).Where("comment_id = ?", comment.ID).Count(&commentLikeCount).Error)
	require.Equal(t, int64(1), commentLikeCount)
}
