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

// CommentLikeRepositoryは、CommentLikeRepository契約をGORMへ適配する。
type CommentLikeRepository struct {
	DB     *gorm.DB
	Tracer trace.Tracer
}

// NewCommentLikeRepositoryは、指定されたDB handleへ結び付いたCommentLikeRepositoryを生成する。
func NewCommentLikeRepository(db *gorm.DB, tracer trace.Tracer) repositories.CommentLikeRepository {
	return &CommentLikeRepository{DB: db, Tracer: tracer}
}

func (r *CommentLikeRepository) Ensure(
	ctx context.Context,
	userID models.UUID,
	commentID models.UUID,
) (err error) {
	ctx, span := tracing.Start(ctx, r.Tracer, tracing.LayerRepository, "CommentLikeRepository.Ensure")
	defer func() { tracing.End(span, repositoryResult(err), recover()) }()

	like := &models.CommentLike{UserID: userID, CommentID: commentID}
	// 未Likeなら作成し、Like済みなら何もしないことで、同じLikeを繰り返しても
	// エラーにしない。
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
) (err error) {
	ctx, span := tracing.Start(ctx, r.Tracer, tracing.LayerRepository, "CommentLikeRepository.Delete")
	defer func() { tracing.End(span, repositoryResult(err), recover()) }()

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
) (err error) {
	ctx, span := tracing.Start(ctx, r.Tracer, tracing.LayerRepository, "CommentLikeRepository.DeleteByCommentIDs")
	defer func() { tracing.End(span, repositoryResult(err), recover()) }()

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

// DeleteByCommentIDWithRepliesは、対象Commentと直接の返信に紐づくLikeを
// サブクエリで物理削除する。
// Comment本体の論理削除はCommentRepositoryが担当し、ここではLike行だけを変更する。
func (r *CommentLikeRepository) DeleteByCommentIDWithReplies(
	ctx context.Context,
	commentID models.UUID,
) (err error) {
	ctx, span := tracing.Start(ctx, r.Tracer, tracing.LayerRepository, "CommentLikeRepository.DeleteByCommentIDWithReplies")
	defer func() { tracing.End(span, repositoryResult(err), recover()) }()

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
) (err error) {
	ctx, span := tracing.Start(ctx, r.Tracer, tracing.LayerRepository, "CommentLikeRepository.DeleteByCommentsOfPostID")
	defer func() { tracing.End(span, repositoryResult(err), recover()) }()

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
) (_ map[models.UUID]int64, err error) {
	ctx, span := tracing.Start(ctx, r.Tracer, tracing.LayerRepository, "CommentLikeRepository.CountByCommentIDs")
	defer func() { tracing.End(span, repositoryResult(err), recover()) }()

	// 親Commentと返信をまとめたID集合を一括集計し、CommentごとのCOUNTを発行しない。
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
) (_ map[models.UUID]struct{}, err error) {
	ctx, span := tracing.Start(ctx, r.Tracer, tracing.LayerRepository, "CommentLikeRepository.FindLikedCommentIDs")
	defer func() { tracing.End(span, repositoryResult(err), recover()) }()

	// current-userのLike状態だけをID集合で取得し、responseのlikedByMeへ変換できる形にする。
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
