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

func (r *CommentRepository) DeleteByID(
	ctx context.Context,
	userID models.UUID,
	commentID models.UUID,
) (int64, error) {
	var rowsAffected int64
	// Comment本体と返信の削除範囲を同じTransactionで確定し、親子の一方だけが残る状態を防ぐ。
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var comment models.Comment
		// 削除対象の親子関係を確定するまで対象行をロックし、同時処理との競合を抑える。
		result := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND author_id = ?", commentID, userID).
			First(&comment)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil
		}
		if result.Error != nil {
			return fmt.Errorf("find comment for delete: %w", result.Error)
		}

		commentIDQuery := tx.Model(&models.Comment{}).
			Select("id").
			Where("id = ?", comment.ID)
		if comment.ParentID == nil {
			commentIDQuery = commentIDQuery.Or("parent_id = ?", comment.ID)
		}
		if err := tx.
			Where("comment_id IN (?)", commentIDQuery).
			Delete(&models.CommentLike{}).Error; err != nil {
			return fmt.Errorf("delete comment likes: %w", err)
		}

		var deleteQuery *gorm.DB
		if comment.ParentID == nil {
			// 親Commentは、会話の階層を部分的に残さないよう直接の返信も削除範囲に含める。
			deleteQuery = tx.Where("id = ? OR parent_id = ?", comment.ID, comment.ID)
		} else {
			// 返信は親Commentと兄弟の返信を残し、自身だけを削除範囲にする。
			deleteQuery = tx.Where("id = ?", comment.ID)
		}
		result = deleteQuery.Delete(&models.Comment{})
		if result.Error != nil {
			return fmt.Errorf("delete comment: %w", result.Error)
		}
		rowsAffected = result.RowsAffected
		return nil
	})
	if err != nil {
		return 0, err
	}
	return rowsAffected, nil
}
