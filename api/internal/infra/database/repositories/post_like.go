package repository

import (
	"context"
	"fmt"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PostLikeRepository struct {
	DB *gorm.DB
}

func NewPostLikeRepository(db *gorm.DB) repositories.PostLikeRepository {
	return &PostLikeRepository{DB: db}
}

func (r *PostLikeRepository) Ensure(
	ctx context.Context,
	userID models.UUID,
	postID models.UUID,
) error {
	like := &models.PostLike{UserID: userID, PostID: postID}
	// 未Likeなら作成し、Like済みなら何もしないことで、同じLikeを繰り返してもエラーにしない。
	result := r.DB.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "post_id"}},
			DoNothing: true,
		}).
		Create(like)
	if result.Error != nil {
		return fmt.Errorf("ensure post like: %w", result.Error)
	}
	return nil
}

func (r *PostLikeRepository) Delete(
	ctx context.Context,
	userID models.UUID,
	postID models.UUID,
) error {
	result := r.DB.WithContext(ctx).
		Where("user_id = ? AND post_id = ?", userID, postID).
		Delete(&models.PostLike{})
	if result.Error != nil {
		return fmt.Errorf("delete post like: %w", result.Error)
	}
	return nil
}

func (r *PostLikeRepository) DeleteByPostID(
	ctx context.Context,
	postID models.UUID,
) error {
	result := r.DB.WithContext(ctx).
		Where("post_id = ?", postID).
		Delete(&models.PostLike{})
	if result.Error != nil {
		return fmt.Errorf("delete post likes by post: %w", result.Error)
	}
	return nil
}

func (r *PostLikeRepository) CountByPostIDs(
	ctx context.Context,
	postIDs []models.UUID,
) (map[models.UUID]int64, error) {
	counts := make(map[models.UUID]int64, len(postIDs))
	if len(postIDs) == 0 {
		return counts, nil
	}

	var rows []struct {
		PostID models.UUID `gorm:"column:post_id"`
		Count  int64       `gorm:"column:count"`
	}
	result := r.DB.WithContext(ctx).
		Model(&models.PostLike{}).
		Select("post_id, COUNT(*) AS count").
		Where("post_id IN ?", postIDs).
		Group("post_id").
		Scan(&rows)
	if result.Error != nil {
		return nil, fmt.Errorf("count post likes: %w", result.Error)
	}
	for _, row := range rows {
		counts[row.PostID] = row.Count
	}
	return counts, nil
}

func (r *PostLikeRepository) FindLikedPostIDs(
	ctx context.Context,
	userID models.UUID,
	postIDs []models.UUID,
) (map[models.UUID]struct{}, error) {
	likedIDs := make(map[models.UUID]struct{}, len(postIDs))
	if len(postIDs) == 0 {
		return likedIDs, nil
	}

	var ids []models.UUID
	result := r.DB.WithContext(ctx).
		Model(&models.PostLike{}).
		Where("user_id = ? AND post_id IN ?", userID, postIDs).
		Pluck("post_id", &ids)
	if result.Error != nil {
		return nil, fmt.Errorf("find liked posts: %w", result.Error)
	}
	for _, id := range ids {
		likedIDs[id] = struct{}{}
	}
	return likedIDs, nil
}
