//go:build integration

package repository

import (
	"context"
	"errors"
	"testing"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"
	"Threadly/internal/usecase"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type rollbackUnitOfWork struct {
	repositories.UnitOfWork
	err error
}

func (u rollbackUnitOfWork) WithinTransaction(
	ctx context.Context,
	fn func(repositories.TransactionRepositories) error,
) error {
	return u.UnitOfWork.WithinTransaction(ctx, func(tx repositories.TransactionRepositories) error {
		if err := fn(tx); err != nil {
			return err
		}
		return u.err
	})
}

func TestDeleteUsecasesCommitOrRollBackRelatedData(t *testing.T) {
	db := openTestDB(t)
	for _, target := range []string{"Post", "Comment"} {
		for _, rollback := range []bool{false, true} {
			outcome := "成功時に対象範囲を削除する"
			if rollback {
				outcome = "失敗時に全変更を巻き戻す"
			}
			t.Run(target+"/"+outcome, func(t *testing.T) {
				user, posts, comments := seedDeleteData(t, db)
				ctx := context.Background()
				uow := NewUnitOfWork(db)
				expectedErr := errors.New("transaction failed after deletes")
				if rollback {
					uow = rollbackUnitOfWork{UnitOfWork: uow, err: expectedErr}
				}
				var err error
				if target == "Post" {
					service := usecase.NewPostUsecase(NewPostRepository(db), uow)
					err = service.DeletePost(ctx, user.ID, posts[0].ID)
				} else {
					service := usecase.NewCommentUsecase(NewCommentRepository(db), NewPostRepository(db), uow)
					err = service.DeleteComment(ctx, user.ID, comments[0].ID)
				}
				if rollback {
					require.ErrorIs(t, err, expectedErr)
				} else {
					require.NoError(t, err)
				}

				for i, post := range posts {
					deleted := !rollback && target == "Post" && i == 0
					var stored models.Post
					require.NoError(t, db.Unscoped().First(&stored, post.ID).Error)
					require.Equal(t, deleted, stored.DeletedAt.Valid)
					var activeRows, likeRows int64
					require.NoError(t, db.Model(&models.Post{}).Where("id = ?", post.ID).Count(&activeRows).Error)
					require.NoError(t, db.Unscoped().Model(&models.PostLike{}).Where("post_id = ?", post.ID).Count(&likeRows).Error)
					wantRows := int64(1)
					if deleted {
						wantRows = 0
					}
					require.Equal(t, wantRows, activeRows)
					require.Equal(t, wantRows, likeRows)
				}
				for i, comment := range comments {
					deleted := !rollback && (i < 2 || (target == "Post" && i == 2))
					var stored models.Comment
					require.NoError(t, db.Unscoped().First(&stored, comment.ID).Error)
					require.Equal(t, deleted, stored.DeletedAt.Valid)
					var activeRows, likeRows int64
					require.NoError(t, db.Model(&models.Comment{}).Where("id = ?", comment.ID).Count(&activeRows).Error)
					require.NoError(t, db.Unscoped().Model(&models.CommentLike{}).Where("comment_id = ?", comment.ID).Count(&likeRows).Error)
					wantRows := int64(1)
					if deleted {
						wantRows = 0
					}
					require.Equal(t, wantRows, activeRows)
					require.Equal(t, wantRows, likeRows)
				}
			})
		}
	}
}

func seedDeleteData(t *testing.T, db *gorm.DB) (*models.User, []*models.Post, []*models.Comment) {
	t.Helper()
	user := &models.User{Username: "delete_" + string(models.NewUUID())[:24], PasswordHash: "hash"}
	var postIDs, commentIDs []models.UUID
	t.Cleanup(func() {
		if len(commentIDs) > 0 {
			require.NoError(t, db.Unscoped().Where("comment_id IN ?", commentIDs).Delete(&models.CommentLike{}).Error)
			require.NoError(t, db.Unscoped().Where("id IN ? AND parent_id IS NOT NULL", commentIDs).Delete(&models.Comment{}).Error)
			require.NoError(t, db.Unscoped().Where("id IN ?", commentIDs).Delete(&models.Comment{}).Error)
		}
		if len(postIDs) > 0 {
			require.NoError(t, db.Unscoped().Where("post_id IN ?", postIDs).Delete(&models.PostLike{}).Error)
			require.NoError(t, db.Unscoped().Where("id IN ?", postIDs).Delete(&models.Post{}).Error)
		}
		require.NoError(t, db.Unscoped().Where("id = ?", user.ID).Delete(&models.User{}).Error)
	})
	require.NoError(t, db.Create(user).Error)
	posts := []*models.Post{
		{AuthorID: user.ID, Title: "target post", Content: "content"},
		{AuthorID: user.ID, Title: "unrelated post", Content: "content"},
	}
	for _, post := range posts {
		require.NoError(t, db.Create(post).Error)
		postIDs = append(postIDs, post.ID)
		require.NoError(t, db.Create(&models.PostLike{UserID: user.ID, PostID: post.ID}).Error)
	}
	comments := []*models.Comment{
		{PostID: posts[0].ID, AuthorID: user.ID, Content: "root"},
		{PostID: posts[0].ID, AuthorID: user.ID, Content: "reply"},
		{PostID: posts[0].ID, AuthorID: user.ID, Content: "unrelated root"},
		{PostID: posts[1].ID, AuthorID: user.ID, Content: "unrelated post comment"},
	}
	for i, comment := range comments {
		if i == 1 {
			comment.ParentID = &comments[0].ID
		}
		require.NoError(t, NewCommentRepository(db).Create(context.Background(), comment))
		commentIDs = append(commentIDs, comment.ID)
		_, err := models.ParseUUID(string(comment.ID))
		require.NoError(t, err)
		require.NoError(t, db.Create(&models.CommentLike{UserID: user.ID, CommentID: comment.ID}).Error)
	}
	return user, posts, comments
}
