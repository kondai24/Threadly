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

// PostRepositoryは、PostRepository契約をGORMへ適配する。
// DBにはroot DBまたはUnitOfWorkが生成したtransaction-bound DBが入る。
type PostRepository struct {
	DB *gorm.DB
}

// NewPostRepositoryは、指定されたDB handleへ結び付いたPostRepositoryを生成する。
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

func (r *PostRepository) GetByIDForUpdate(
	ctx context.Context,
	postID models.UUID,
) (*models.Post, error) {
	// 呼び出し元が同じTransaction内で後続更新するPostをロックし、削除との
	// 競合を直列化する。
	var post models.Post
	result := r.DB.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", postID).
		First(&post)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, repositories.ErrPostNotFound
	}
	if result.Error != nil {
		return nil, fmt.Errorf("find post for update: %w", result.Error)
	}
	return &post, nil
}

func (r *PostRepository) GetByIDForOwner(
	ctx context.Context,
	userID models.UUID,
	postID models.UUID,
) (*models.Post, error) {
	// 所有者条件を検索へ含め、存在しない・削除済み・他User所有のPostを
	// 同じNotFound契約にする。
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
	// 関連Comment・LikeのcleanupはUsecaseが別Repositoryへ委譲するため、ここでは
	// Post本体だけを論理削除する。
	result := r.DB.WithContext(ctx).
		Where("id = ? AND author_id = ?", postID, userID).
		Delete(&models.Post{})
	if result.Error != nil {
		return 0, fmt.Errorf("delete post: %w", result.Error)
	}
	return result.RowsAffected, nil
}

func (r *PostRepository) ListAll(ctx context.Context) ([]*models.Post, error) {
	posts := make([]*models.Post, 0)
	result := r.DB.WithContext(ctx).
		Preload("Author").
		Find(&posts)
	return posts, result.Error
}
