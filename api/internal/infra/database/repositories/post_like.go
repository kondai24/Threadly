package repository

import (
	"context"
	"fmt"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"
	"Threadly/internal/tracing"

	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PostLikeRepositoryは、PostLikeRepository契約をGORMへ適配する。
type PostLikeRepository struct {
	DB     *gorm.DB
	Tracer trace.Tracer
}

// NewPostLikeRepositoryは、指定されたDB handleへ結び付いたPostLikeRepositoryを生成する。
func NewPostLikeRepository(db *gorm.DB, tracer trace.Tracer) repositories.PostLikeRepository {
	return &PostLikeRepository{DB: db, Tracer: tracer}
}

func (r *PostLikeRepository) Ensure(
	ctx context.Context,
	userID models.UUID,
	postID models.UUID,
) (err error) {
	ctx, span := tracing.Start(ctx, r.Tracer, tracing.LayerRepository, "PostLikeRepository.Ensure")
	defer func() { tracing.End(span, repositoryResult(err), recover()) }()

	like := &models.PostLike{UserID: userID, PostID: postID}
	// 未Likeなら作成し、Like済みなら何もしないことで、同じLikeを繰り返しても
	// エラーにしない。
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
) (err error) {
	ctx, span := tracing.Start(ctx, r.Tracer, tracing.LayerRepository, "PostLikeRepository.Delete")
	defer func() { tracing.End(span, repositoryResult(err), recover()) }()

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
) (err error) {
	ctx, span := tracing.Start(ctx, r.Tracer, tracing.LayerRepository, "PostLikeRepository.DeleteByPostID")
	defer func() { tracing.End(span, repositoryResult(err), recover()) }()

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
) (_ map[models.UUID]int64, err error) {
	ctx, span := tracing.Start(ctx, r.Tracer, tracing.LayerRepository, "PostLikeRepository.CountByPostIDs")
	defer func() { tracing.End(span, repositoryResult(err), recover()) }()

	// Post一覧の各行を個別COUNTせず、対象ID集合をGROUP BYしてN+1を避ける。
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
) (_ map[models.UUID]struct{}, err error) {
	ctx, span := tracing.Start(ctx, r.Tracer, tracing.LayerRepository, "PostLikeRepository.FindLikedPostIDs")
	defer func() { tracing.End(span, repositoryResult(err), recover()) }()

	// 件数とは別にcurrent-userのLike状態を取得し、全UserのLike情報を公開しない。
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
