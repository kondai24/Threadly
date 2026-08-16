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

type PostRepository struct {
	DB *gorm.DB
}

func NewPostRepository(db *gorm.DB) repositories.PostRepository {
	return &PostRepository{DB: db}
}

func (r *PostRepository) GetByID(ctx context.Context, postID models.UUID) (*models.Post, error) {
	var post models.Post
	result := r.DB.WithContext(ctx).
		Preload("Author").
		Where("id = ?", postID).
		First(&post)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, repositories.ErrPostNotFound
	}
	if result.Error != nil {
		return nil, fmt.Errorf("find post by id: %w", result.Error)
	}
	return &post, nil
}

func (r *PostRepository) GetByIDForOwner(ctx context.Context, userID models.UUID, postID models.UUID) (*models.Post, error) {
	var post models.Post
	result := r.DB.WithContext(ctx).
		Preload("Author").
		Where("id = ? AND author_id = ?", postID, userID).
		First(&post)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, repositories.ErrPostNotFound
	}
	if result.Error != nil {
		return nil, fmt.Errorf("find post by owner: %w", result.Error)
	}
	return &post, nil
}

func (r *PostRepository) Create(ctx context.Context, post *models.Post) error {
	return r.DB.WithContext(ctx).Create(post).Error
}

func (r *PostRepository) Update(ctx context.Context, userID models.UUID, post *models.Post) error {
	// mapを使い、更新値が空文字でもGORMに無視されないようにする。
	result := r.DB.WithContext(ctx).
		Model(&models.Post{}).
		Where("id = ? AND author_id = ?", post.ID, userID).
		Updates(map[string]any{
			"title":   post.Title,
			"content": post.Content,
		})
	if result.Error != nil {
		return fmt.Errorf("update post: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return repositories.ErrPostNotFound
	}
	return nil
}

func (r *PostRepository) DeleteByID(ctx context.Context, userID models.UUID, postID models.UUID) (int64, error) {
	var rowsAffected int64
	// Postと配下のCommentを同じTransactionで論理削除し、Postだけが消えてCommentが残る状態を防ぐ。
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var post models.Post
		// 削除対象を確定するまでPost行をロックし、子行と本体で同じPostを扱う。
		result := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND author_id = ?", postID, userID).
			First(&post)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil
		}
		if result.Error != nil {
			return fmt.Errorf("find post for delete: %w", result.Error)
		}

		// Postに属する親Commentと返信を先に論理削除し、通常Queryからも一緒に除外する。
		var commentIDs []models.UUID
		if result := tx.Model(&models.Comment{}).
			Where("post_id = ?", post.ID).
			Pluck("id", &commentIDs); result.Error != nil {
			return fmt.Errorf("find post comment ids for like cleanup: %w", result.Error)
		}
		if len(commentIDs) > 0 {
			if result := tx.Where("comment_id IN ?", commentIDs).Delete(&models.CommentLike{}); result.Error != nil {
				return fmt.Errorf("delete post comment likes: %w", result.Error)
			}
		}
		if result := tx.Where("post_id = ?", post.ID).Delete(&models.PostLike{}); result.Error != nil {
			return fmt.Errorf("delete post likes: %w", result.Error)
		}
		if result := tx.Where("post_id = ?", post.ID).Delete(&models.Comment{}); result.Error != nil {
			return fmt.Errorf("delete post comments: %w", result.Error)
		}

		result = tx.Where("id = ?", post.ID).Delete(&models.Post{})
		if result.Error != nil {
			return fmt.Errorf("delete post: %w", result.Error)
		}
		rowsAffected = result.RowsAffected
		return nil
	})
	if err != nil {
		return 0, err
	}
	return rowsAffected, nil
}

func (r *PostRepository) ListAll(ctx context.Context) ([]*models.Post, error) {
	posts := make([]*models.Post, 0)
	result := r.DB.WithContext(ctx).
		Preload("Author").
		Find(&posts)
	return posts, result.Error
}
