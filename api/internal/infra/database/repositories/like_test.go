//go:build integration

package repository

import (
	"context"
	"testing"

	"Threadly/internal/domain/models"
	"Threadly/internal/tracing/tracingtest"

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
	repo := &PostLikeRepository{DB: tx, Tracer: noopTracer()}

	require.NoError(t, repo.Ensure(context.Background(), user.ID, post.ID))
	require.NoError(t, repo.Ensure(context.Background(), user.ID, post.ID))

	counts, err := repo.CountByPostIDs(context.Background(), []models.UUID{post.ID})
	require.NoError(t, err)
	require.Equal(t, int64(1), counts[post.ID])
	likedIDs, err := repo.FindLikedPostIDs(context.Background(), user.ID, []models.UUID{post.ID})
	require.NoError(t, err)
	_, liked := likedIDs[post.ID]
	require.True(t, liked)

	require.NoError(t, repo.Delete(context.Background(), user.ID, post.ID))
	require.NoError(t, repo.Delete(context.Background(), user.ID, post.ID))
	var rows int64
	require.NoError(t, tx.Unscoped().Model(&models.PostLike{}).Where("post_id = ?", post.ID).Count(&rows).Error)
	require.Zero(t, rows)
	require.NoError(t, repo.Ensure(context.Background(), user.ID, post.ID))
	counts, err = repo.CountByPostIDs(context.Background(), []models.UUID{post.ID})
	require.NoError(t, err)
	require.Equal(t, int64(1), counts[post.ID])
}

func TestCommentLikeRepository_IsIdempotentAndSupportsRelike(t *testing.T) {
	db := openTestDB(t)
	tx := db.Begin()
	require.NoError(t, tx.Error)
	t.Cleanup(func() { require.NoError(t, tx.Rollback().Error) })

	user, _, comment := seedLikeRepositoryData(t, tx)
	repo := &CommentLikeRepository{DB: tx, Tracer: noopTracer()}

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
	require.NoError(t, repo.Delete(context.Background(), user.ID, comment.ID))
	var rows int64
	require.NoError(t, tx.Unscoped().Model(&models.CommentLike{}).Where("comment_id = ?", comment.ID).Count(&rows).Error)
	require.Zero(t, rows)
	require.NoError(t, repo.Ensure(context.Background(), user.ID, comment.ID))
	counts, err = repo.CountByCommentIDs(context.Background(), []models.UUID{comment.ID})
	require.NoError(t, err)
	require.Equal(t, int64(1), counts[comment.ID])
}

func TestLikeRepositories_RecordOneSpanPerOperationIncludingEmptyInput(t *testing.T) {
	db := openTestDB(t)
	tx := db.Begin()
	require.NoError(t, tx.Error)
	t.Cleanup(func() { require.NoError(t, tx.Rollback().Error) })
	user, post, comment := seedLikeRepositoryData(t, tx)
	tracer, recorder := tracingtest.NewRecorder()
	postLikes := NewPostLikeRepository(tx, tracer)
	commentLikes := NewCommentLikeRepository(tx, tracer)
	ctx := context.Background()

	require.NoError(t, postLikes.Ensure(ctx, user.ID, post.ID))
	require.NoError(t, postLikes.Delete(ctx, user.ID, post.ID))
	require.NoError(t, postLikes.DeleteByPostID(ctx, post.ID))
	// 空集合の早期returnでもRepository操作としてSpanを1本残す。
	_, err := postLikes.CountByPostIDs(ctx, nil)
	require.NoError(t, err)
	_, err = postLikes.FindLikedPostIDs(ctx, user.ID, nil)
	require.NoError(t, err)
	require.NoError(t, commentLikes.Ensure(ctx, user.ID, comment.ID))
	require.NoError(t, commentLikes.Delete(ctx, user.ID, comment.ID))
	require.NoError(t, commentLikes.DeleteByCommentIDs(ctx, nil))
	require.NoError(t, commentLikes.DeleteByCommentIDWithReplies(ctx, comment.ID))
	require.NoError(t, commentLikes.DeleteByCommentsOfPostID(ctx, post.ID))
	_, err = commentLikes.CountByCommentIDs(ctx, []models.UUID{comment.ID})
	require.NoError(t, err)
	_, err = commentLikes.FindLikedCommentIDs(ctx, user.ID, []models.UUID{comment.ID})
	require.NoError(t, err)

	require.Equal(t, []string{
		"PostLikeRepository.Ensure success",
		"PostLikeRepository.Delete success",
		"PostLikeRepository.DeleteByPostID success",
		"PostLikeRepository.CountByPostIDs success",
		"PostLikeRepository.FindLikedPostIDs success",
		"CommentLikeRepository.Ensure success",
		"CommentLikeRepository.Delete success",
		"CommentLikeRepository.DeleteByCommentIDs success",
		"CommentLikeRepository.DeleteByCommentIDWithReplies success",
		"CommentLikeRepository.DeleteByCommentsOfPostID success",
		"CommentLikeRepository.CountByCommentIDs success",
		"CommentLikeRepository.FindLikedCommentIDs success",
	}, tracingtest.Results(recorder.Ended()))
}
