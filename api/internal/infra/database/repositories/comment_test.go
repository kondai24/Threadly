//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newTestCommentRepository(t *testing.T) (*CommentRepository, *gorm.DB) {
	t.Helper()

	db := openTestDB(t)
	tx := db.Begin()
	require.NoError(t, tx.Error)
	t.Cleanup(func() {
		require.NoError(t, tx.Rollback().Error)
	})

	return &CommentRepository{DB: tx}, tx
}

func seedCommentRepositoryData(t *testing.T, db *gorm.DB) (*models.User, *models.Post, *models.Comment, *models.Comment) {
	t.Helper()

	user := &models.User{Username: "comment-repository-user", PasswordHash: "hash"}
	otherUser := &models.User{Username: "comment-repository-other", PasswordHash: "hash"}
	require.NoError(t, db.Create(user).Error)
	require.NoError(t, db.Create(otherUser).Error)

	post := &models.Post{
		AuthorID: user.ID,
		Title:    "post",
		Content:  "content",
	}
	require.NoError(t, db.Create(post).Error)

	root := &models.Comment{
		UUIDBaseModel: models.UUIDBaseModel{
			ID:        models.UUID("77777777-7777-4777-8777-777777777777"),
			CreatedAt: time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC),
		},
		PostID:   post.ID,
		AuthorID: user.ID,
		Content:  "root",
	}
	require.NoError(t, db.Create(root).Error)

	reply := &models.Comment{
		UUIDBaseModel: models.UUIDBaseModel{
			ID:        models.UUID("88888888-8888-4888-8888-888888888888"),
			CreatedAt: time.Date(2026, 8, 15, 12, 1, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 8, 15, 12, 1, 0, 0, time.UTC),
		},
		PostID:   post.ID,
		AuthorID: otherUser.ID,
		ParentID: &root.ID,
		Content:  "reply",
	}
	require.NoError(t, db.Create(reply).Error)

	newRoot := &models.Comment{
		UUIDBaseModel: models.UUIDBaseModel{
			ID:        models.UUID("99999999-9999-4999-8999-999999999999"),
			CreatedAt: time.Date(2026, 8, 15, 12, 2, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 8, 15, 12, 2, 0, 0, time.UTC),
		},
		PostID:   post.ID,
		AuthorID: user.ID,
		Content:  "new root",
	}
	require.NoError(t, db.Create(newRoot).Error)

	return user, post, root, reply
}

func TestCommentRepository_ListByPostID(t *testing.T) {
	repo, db := newTestCommentRepository(t)
	_, post, root, reply := seedCommentRepositoryData(t, db)

	comments, err := repo.ListByPostID(context.Background(), post.ID)

	require.NoError(t, err)
	require.Len(t, comments, 2)
	require.Equal(t, models.UUID("99999999-9999-4999-8999-999999999999"), comments[0].ID)
	require.NotNil(t, comments[0].Replies)
	require.Empty(t, comments[0].Replies)
	require.Equal(t, root.ID, comments[1].ID)
	require.Len(t, comments[1].Replies, 1)
	require.Equal(t, reply.ID, comments[1].Replies[0].ID)
	require.Equal(t, "comment-repository-other", comments[1].Replies[0].Author.Username)
}

func TestCommentRepository_UpdateRequiresAuthor(t *testing.T) {
	repo, db := newTestCommentRepository(t)
	user, post, root, _ := seedCommentRepositoryData(t, db)
	otherUser := &models.User{}
	require.NoError(t, db.Where("username = ?", "comment-repository-other").First(otherUser).Error)

	rows, err := repo.Update(context.Background(), otherUser.ID, root.ID, "tampered")
	require.ErrorIs(t, err, repositories.ErrCommentNotFound)
	require.Zero(t, rows)

	rows, err = repo.Update(context.Background(), user.ID, root.ID, "updated")
	require.NoError(t, err)
	require.Equal(t, int64(1), rows)

	var stored models.Comment
	require.NoError(t, db.Where("post_id = ? AND id = ?", post.ID, root.ID).First(&stored).Error)
	require.Equal(t, "updated", stored.Content)
}

func TestCommentRepository_DeleteMethodsOnlyChangeComments(t *testing.T) {
	repo, db := newTestCommentRepository(t)
	user, post, root, reply := seedCommentRepositoryData(t, db)
	require.NoError(t, db.Create(&models.CommentLike{UserID: user.ID, CommentID: root.ID}).Error)
	require.NoError(t, db.Create(&models.CommentLike{UserID: user.ID, CommentID: reply.ID}).Error)

	replyIDs, err := repo.ListIDsByParentID(context.Background(), root.ID)
	require.NoError(t, err)
	require.Equal(t, []models.UUID{reply.ID}, replyIDs)

	rows, err := repo.DeleteByID(context.Background(), user.ID, root.ID)

	require.NoError(t, err)
	require.Equal(t, int64(1), rows)
	rows, err = repo.DeleteRepliesByParentID(context.Background(), root.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1), rows)
	comments, err := repo.ListByPostID(context.Background(), post.ID)
	require.NoError(t, err)
	require.Len(t, comments, 1)
	require.Equal(t, models.UUID("99999999-9999-4999-8999-999999999999"), comments[0].ID)

	var deleted []models.Comment
	require.NoError(t, db.Unscoped().Where("id IN ?", []models.UUID{root.ID, reply.ID}).Find(&deleted).Error)
	require.Len(t, deleted, 2)
	for _, comment := range deleted {
		require.True(t, comment.DeletedAt.Valid)
	}
	var likes []models.CommentLike
	require.NoError(t, db.Where("comment_id IN ?", []models.UUID{root.ID, reply.ID}).Find(&likes).Error)
	require.Len(t, likes, 2)
}
