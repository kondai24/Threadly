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
	"gorm.io/gorm"
)

func TestUnitOfWorkRollsBackCrossRepositoryDeletesTogether(t *testing.T) {
	db := openTestDB(t)
	user := &models.User{
		Username:     "uow-rollback-" + strconv.FormatInt(time.Now().UnixNano(), 10),
		PasswordHash: "hash",
	}
	require.NoError(t, db.Create(user).Error)
	t.Cleanup(func() {
		require.NoError(t, db.Unscoped().Where("id = ?", user.ID).Delete(&models.User{}).Error)
	})

	post := &models.Post{
		AuthorID: user.ID,
		Title:    "post",
		Content:  "content",
	}
	comment := &models.Comment{
		PostID:   post.ID,
		AuthorID: user.ID,
		Content:  "comment",
	}
	expectedErr := errors.New("rollback transaction")
	uow := NewUnitOfWork(db)

	err := uow.WithinTransaction(context.Background(), func(tx repositories.TransactionRepositories) error {
		if err := tx.Post.Create(context.Background(), post); err != nil {
			return err
		}
		comment.PostID = post.ID
		if err := tx.Comment.Create(context.Background(), comment); err != nil {
			return err
		}
		if err := tx.PostLike.Ensure(context.Background(), user.ID, post.ID); err != nil {
			return err
		}
		if err := tx.CommentLike.Ensure(context.Background(), user.ID, comment.ID); err != nil {
			return err
		}
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
	require.ErrorIs(t, db.Unscoped().First(&storedPost, post.ID).Error, gorm.ErrRecordNotFound)
	var storedComment models.Comment
	require.ErrorIs(t, db.Unscoped().First(&storedComment, comment.ID).Error, gorm.ErrRecordNotFound)
	var postLikes []models.PostLike
	require.NoError(t, db.Where("post_id = ?", post.ID).Find(&postLikes).Error)
	require.Empty(t, postLikes)
	var commentLikes []models.CommentLike
	require.NoError(t, db.Where("comment_id = ?", comment.ID).Find(&commentLikes).Error)
	require.Empty(t, commentLikes)
}
