package repositories

import (
	"context"

	"Threadly/internal/domain/models"
)

// PostLikeRepositoryはPostLikeの永続化と対象ID集合の集計を担当する。
type PostLikeRepository interface {
	Ensure(ctx context.Context, userID models.UUID, postID models.UUID) error
	Delete(ctx context.Context, userID models.UUID, postID models.UUID) error
	DeleteByPostID(ctx context.Context, postID models.UUID) error
	CountByPostIDs(ctx context.Context, postIDs []models.UUID) (map[models.UUID]int64, error)
	FindLikedPostIDs(ctx context.Context, userID models.UUID, postIDs []models.UUID) (map[models.UUID]struct{}, error)
}

// CommentLikeRepositoryはCommentLikeの永続化と対象ID集合の集計を担当する。
type CommentLikeRepository interface {
	Ensure(ctx context.Context, userID models.UUID, commentID models.UUID) error
	Delete(ctx context.Context, userID models.UUID, commentID models.UUID) error
	DeleteByCommentIDs(ctx context.Context, commentIDs []models.UUID) error
	DeleteByCommentIDWithReplies(ctx context.Context, commentID models.UUID) error
	DeleteByCommentsOfPostID(ctx context.Context, postID models.UUID) error
	CountByCommentIDs(ctx context.Context, commentIDs []models.UUID) (map[models.UUID]int64, error)
	FindLikedCommentIDs(ctx context.Context, userID models.UUID, commentIDs []models.UUID) (map[models.UUID]struct{}, error)
}
