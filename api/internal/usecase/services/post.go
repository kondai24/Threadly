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

// 認証済みUserが閲覧できるPostを取得する。閲覧時は所有者条件を付けない。
func (s *PostService) GetPostByID(ctx context.Context, postID models.UUID) (*models.Post, error) {
	return s.repo.GetByID(ctx, postID)
}

// 更新前の所有者確認など、所有者だけが扱うPostを取得する。
func (s *PostService) GetPostByIDForOwner(ctx context.Context, userID models.UUID, postID models.UUID) (*models.Post, error) {
	return s.repo.GetByIDForOwner(ctx, userID, postID)
}

// 認証済みUserが閲覧できる全Postを取得する。
func (s *PostService) ListAllPosts(ctx context.Context) ([]*models.Post, error) {
	return s.repo.ListAll(ctx)
}

// author_idはリクエストではなく、検証済みtokenのUser IDから設定する。
func (s *PostService) CreatePost(ctx context.Context, userID models.UUID, title string, content string) error {
	post := &models.Post{
		AuthorID: userID,
		Title:    title,
		Content:  content,
	}
	if err := post.Validate(); err != nil {
		return err
	}
	return s.repo.Create(ctx, post)
}

// 所有者でない場合はNotFoundとして扱い、他UserのPostの存在を隠す。
func (s *PostService) UpdatePost(ctx context.Context, userID models.UUID, post *models.Post) error {
	if post.AuthorID != userID {
		return ErrPostNotFound
	}
	if err := post.Validate(); err != nil {
		return err
	}
	return s.repo.Update(ctx, userID, post)
}

// 削除もRepositoryでuserIDを条件に含め、所有者境界を維持する。
func (s *PostService) DeletePost(ctx context.Context, userID models.UUID, postID models.UUID) error {
	rows, err := s.repo.DeleteByID(ctx, userID, postID)
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrPostNotFound
	}
	return nil
}
