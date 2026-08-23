package repository

import (
	"context"
	"fmt"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CommentLikeRepository struct {
	DB *gorm.DB
}

func NewCommentLikeRepository(db *gorm.DB) repositories.CommentLikeRepository {
	return &CommentLikeRepository{DB: db}
}

func (r *CommentLikeRepository) Ensure(
	ctx context.Context,
	userID models.UUID,
	commentID models.UUID,
) error {
	like := &models.CommentLike{UserID: userID, CommentID: commentID}
	// 未Likeなら作成し、Like済みなら何もしないことで、同じLikeを繰り返してもエラーにしない。
	result := r.DB.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "comment_id"}},
			DoNothing: true,
		}).
		Create(like)
	if result.Error != nil {
		return fmt.Errorf("ensure comment like: %w", result.Error)
	}
	return nil
}

func (r *CommentLikeRepository) Delete(
	ctx context.Context,
	userID models.UUID,
	commentID models.UUID,
) error {
	result := r.DB.WithContext(ctx).
		Where("user_id = ? AND comment_id = ?", userID, commentID).
		Delete(&models.CommentLike{})
	if result.Error != nil {
		return fmt.Errorf("delete comment like: %w", result.Error)
	}
	return nil
}

func (r *CommentLikeRepository) DeleteByCommentIDs(
	ctx context.Context,
	commentIDs []models.UUID,
) error {
	if len(commentIDs) == 0 {
		return nil
	}

	result := r.DB.WithContext(ctx).
		Where("comment_id IN ?", commentIDs).
		Delete(&models.CommentLike{})
	if result.Error != nil {
		return fmt.Errorf("delete comment likes by comments: %w", result.Error)
	}
	return nil
}

// DeleteByCommentIDWithRepliesは、対象Commentと直接の返信に紐づくLikeをサブクエリで物理削除する。
func (r *CommentLikeRepository) DeleteByCommentIDWithReplies(
	ctx context.Context,
	commentID models.UUID,
) error {
	commentIDQuery := r.DB.WithContext(ctx).
		Model(&models.Comment{}).
		Select("id").
		Where("id = ? OR parent_id = ?", commentID, commentID)
	result := r.DB.WithContext(ctx).
		Where("comment_id IN (?)", commentIDQuery).
		Delete(&models.CommentLike{})
	if result.Error != nil {
		return fmt.Errorf("delete comment likes with replies: %w", result.Error)
	}
	return nil
}

// DeleteByCommentsOfPostIDは、指定Postに属するComment（返信を含む）自体は削除せず、
// それらに紐づくCommentLikeだけをCommentのIDを用いたサブクエリで物理削除する。
func (r *CommentLikeRepository) DeleteByCommentsOfPostID(
	ctx context.Context,
	postID models.UUID,
) error {
	commentIDQuery := r.DB.WithContext(ctx).
		Model(&models.Comment{}).
		Select("id").
		Where("post_id = ?", postID)
	result := r.DB.WithContext(ctx).
		Where("comment_id IN (?)", commentIDQuery).
		Delete(&models.CommentLike{})
	if result.Error != nil {
		return fmt.Errorf("delete comment likes of post comments: %w", result.Error)
	}
	return nil
}

func (r *CommentLikeRepository) CountByCommentIDs(
	ctx context.Context,
	commentIDs []models.UUID,
) (map[models.UUID]int64, error) {
	counts := make(map[models.UUID]int64, len(commentIDs))
	if len(commentIDs) == 0 {
		return counts, nil
	}

	var rows []struct {
		CommentID models.UUID `gorm:"column:comment_id"`
		Count     int64       `gorm:"column:count"`
	}
	result := r.DB.WithContext(ctx).
		Model(&models.CommentLike{}).
		Select("comment_id, COUNT(*) AS count").
		Where("comment_id IN ?", commentIDs).
		Group("comment_id").
		Scan(&rows)
	if result.Error != nil {
		return nil, fmt.Errorf("count comment likes: %w", result.Error)
	}
	for _, row := range rows {
		counts[row.CommentID] = row.Count
	}
	return counts, nil
}

func (r *CommentLikeRepository) FindLikedCommentIDs(
	ctx context.Context,
	userID models.UUID,
	commentIDs []models.UUID,
) (map[models.UUID]struct{}, error) {
	likedIDs := make(map[models.UUID]struct{}, len(commentIDs))
	if len(commentIDs) == 0 {
		return likedIDs, nil
	}

	var ids []models.UUID
	result := r.DB.WithContext(ctx).
		Model(&models.CommentLike{}).
		Where("user_id = ? AND comment_id IN ?", userID, commentIDs).
		Pluck("comment_id", &ids)
	if result.Error != nil {
		return nil, fmt.Errorf("find liked comments: %w", result.Error)
	}
	for _, id := range ids {
		likedIDs[id] = struct{}{}
	}
	return likedIDs, nil
}
