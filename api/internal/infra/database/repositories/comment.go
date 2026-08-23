package repository

import (
	"context"
	"errors"
	"fmt"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CommentRepository struct {
	DB *gorm.DB
}

func NewCommentRepository(db *gorm.DB) repositories.CommentRepository {
	return &CommentRepository{DB: db}
}

func (r *CommentRepository) Create(ctx context.Context, comment *models.Comment) error {
	if err := r.DB.WithContext(ctx).Create(comment).Error; err != nil {
		return fmt.Errorf("create comment: %w", err)
	}
	return nil
}

func (r *CommentRepository) ListByPostID(
	ctx context.Context,
	postID models.UUID,
) ([]*models.Comment, error) {
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
) (*models.Comment, error) {
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
) (*models.Comment, error) {
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
) (int64, error) {
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
) (int64, error) {
	result := r.DB.WithContext(ctx).
		Where("post_id = ?", postID).
		Delete(&models.Comment{})
	if result.Error != nil {
		return 0, fmt.Errorf("delete comments by post: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// DeleteByIDWithRepliesは、所有者が削除できるCommentと、その直接の返信を同じ論理削除処理で扱う。
// 先に対象Commentの所有者条件を確認するため、権限のない場合に返信だけが削除されることはない。
func (r *CommentRepository) DeleteByIDWithReplies(
	ctx context.Context,
	userID models.UUID,
	commentID models.UUID,
) (int64, error) {
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
