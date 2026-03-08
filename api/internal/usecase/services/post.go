package services

import (
	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"
	"context"
)

type PostService struct {
	repo repositories.PostRepository
}

func NewPostService(repo repositories.PostRepository) *PostService {
	return &PostService{repo: repo}

}

// IDでPostを取得
func (s *PostService) GetPostById(ctx context.Context, id uint) (*models.Post, error) {
	return s.repo.GetById(ctx, id)
}

// 全てのPostを取得
func (s *PostService) ListPosts(ctx context.Context) ([]*models.Post, error) {
	return s.repo.List(ctx)
}

// 新しいPostを作成
func (s *PostService) CreatePost(ctx context.Context, title string, content string) error {
	post := &models.Post{
		Title:   title,
		Content: content,
	}
	if err := post.Validate(); err != nil {
		return err
	}
	return s.repo.Create(ctx, post)
}

// Postを更新
func (s *PostService) UpdatePost(ctx context.Context, post *models.Post) error {
	if err := post.Validate(); err != nil {
		return err
	}
	return s.repo.Update(ctx, post)
}

// Postを削除
func (s *PostService) DeletePost(ctx context.Context, id uint) error {
	rows, err := s.repo.DeleteById(ctx, id)
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrPostNotFound
	}
	return nil
}
