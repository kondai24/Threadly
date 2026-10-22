package repository

import (
	"context"
	"errors"
	"fmt"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"
	"Threadly/internal/tracing"

	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CommentRepositoryは、CommentRepository契約をGORMへ適配する。
// DBにはroot DBまたはUnitOfWorkが生成したtransaction-bound DBが入る。
type CommentRepository struct {
	DB     *gorm.DB
	Tracer trace.Tracer
}

// NewCommentRepositoryは、指定されたDB handleへ結び付いたCommentRepositoryを生成する。
func NewCommentRepository(db *gorm.DB, tracer trace.Tracer) repositories.CommentRepository {
	return &CommentRepository{DB: db, Tracer: tracer}
}

func (r *CommentRepository) Create(ctx context.Context, comment *models.Comment) (err error) {
	ctx, span := tracing.Start(ctx, r.Tracer, tracing.LayerRepository, "CommentRepository.Create")
	defer func() { tracing.End(span, repositoryResult(err), recover()) }()

	if err := r.DB.WithContext(ctx).Create(comment).Error; err != nil {
		return fmt.Errorf("create comment: %w", err)
	}
	return nil
}

func (r *CommentRepository) ListByPostID(
	ctx context.Context,
	postID models.UUID,
) (_ []*models.Comment, err error) {
	ctx, span := tracing.Start(ctx, r.Tracer, tracing.LayerRepository, "CommentRepository.ListByPostID")
	defer func() { tracing.End(span, repositoryResult(err), recover()) }()

	// 親Commentだけを取得し、RepliesはGORMのPreloadで1段階に限定して復元する。
	comments := make([]*models.Comment, 0)
	result := r.DB.WithContext(ctx).
		Preload("Author").
		Preload("Replies", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC").Order("id DESC")
		}).
		Preload("Replies.Author").
		Where("post_id = ? AND parent_id IS NULL", postID).
		Order("created_at DESC").
		Order("id DESC").
		Find(&comments)
	if result.Error != nil {
		return nil, fmt.Errorf("list comments by post: %w", result.Error)
	}
	return comments, nil
}

func (r *CommentRepository) GetByID(
	ctx context.Context,
	commentID models.UUID,
) (_ *models.Comment, err error) {
	ctx, span := tracing.Start(ctx, r.Tracer, tracing.LayerRepository, "CommentRepository.GetByID")
	defer func() { tracing.End(span, repositoryResult(err), recover()) }()

	var comment models.Comment
	result := r.DB.WithContext(ctx).First(&comment, "id = ?", commentID)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		// GORMのNotFoundをRepository契約へ変換し、上位層がORMのエラー型に依存しないようにする。
		return nil, repositories.ErrCommentNotFound
	}
	if result.Error != nil {
		return nil, fmt.Errorf("find comment by id: %w", result.Error)
	}
	return &comment, nil
}

func (r *CommentRepository) GetByIDForUpdate(
	ctx context.Context,
	commentID models.UUID,
) (_ *models.Comment, err error) {
	ctx, span := tracing.Start(ctx, r.Tracer, tracing.LayerRepository, "CommentRepository.GetByIDForUpdate")
	defer func() { tracing.End(span, repositoryResult(err), recover()) }()

	// 返信作成・削除の前提となるCommentをロックし、存在確認直後の競合更新を防ぐ。
	var comment models.Comment
	result := r.DB.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&comment, "id = ?", commentID)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, repositories.ErrCommentNotFound
	}
	if result.Error != nil {
		return nil, fmt.Errorf("find comment for update: %w", result.Error)
	}
	return &comment, nil
}

func (r *CommentRepository) Update(
	ctx context.Context,
	userID models.UUID,
	commentID models.UUID,
	content string,
) (_ int64, err error) {
	ctx, span := tracing.Start(ctx, r.Tracer, tracing.LayerRepository, "CommentRepository.Update")
	defer func() { tracing.End(span, repositoryResult(err), recover()) }()

	// 所有者条件をSQLへ含め、非所有者へ更新対象の存在を返さない。
	result := r.DB.WithContext(ctx).
		Model(&models.Comment{}).
		Where("id = ? AND author_id = ?", commentID, userID).
		Updates(map[string]any{"content": content})
	if result.Error != nil {
		return 0, fmt.Errorf("update comment: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return 0, repositories.ErrCommentNotFound
	}
	return result.RowsAffected, nil
}

func (r *CommentRepository) DeleteByPostID(
	ctx context.Context,
	postID models.UUID,
) (_ int64, err error) {
	ctx, span := tracing.Start(ctx, r.Tracer, tracing.LayerRepository, "CommentRepository.DeleteByPostID")
	defer func() { tracing.End(span, repositoryResult(err), recover()) }()

	// Post配下の全Commentを論理削除する。CommentLikeの物理削除は別Repositoryが担当する。
	result := r.DB.WithContext(ctx).
		Where("post_id = ?", postID).
		Delete(&models.Comment{})
	if result.Error != nil {
		return 0, fmt.Errorf("delete comments by post: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// DeleteByIDWithRepliesは、所有者が削除できるCommentと、その直接の返信を論理削除する。
// 先に対象Commentの所有者条件を確認し、親が削除できない場合は返信へ
// 変更を加えない。
// 親と返信を同じ業務Transactionにする責務は、呼び出し元のUsecase/UoWにある。
func (r *CommentRepository) DeleteByIDWithReplies(
	ctx context.Context,
	userID models.UUID,
	commentID models.UUID,
) (_ int64, err error) {
	ctx, span := tracing.Start(ctx, r.Tracer, tracing.LayerRepository, "CommentRepository.DeleteByIDWithReplies")
	defer func() { tracing.End(span, repositoryResult(err), recover()) }()

	result := r.DB.WithContext(ctx).
		Where("id = ? AND author_id = ?", commentID, userID).
		Delete(&models.Comment{})
	if result.Error != nil {
		return 0, fmt.Errorf("delete comment with replies: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return 0, nil
	}

	replyResult := r.DB.WithContext(ctx).
		Where("parent_id = ?", commentID).
		Delete(&models.Comment{})
	if replyResult.Error != nil {
		return 0, fmt.Errorf("delete comment replies: %w", replyResult.Error)
	}
	return result.RowsAffected + replyResult.RowsAffected, nil
}
