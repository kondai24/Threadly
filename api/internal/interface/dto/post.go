package dto

import (
	"time"

	"Threadly/internal/domain/models"
)

type CreatePostRequest struct {
	Title   string `json:"title" binding:"required"`
	Content string `json:"content" binding:"required"`
}

type UpdatePostRequest struct {
	Title   *string `json:"title"`
	Content *string `json:"content"`
}

type PostListResponse struct {
	ID        string             `json:"id"`
	Title     string             `json:"title"`
	Author    PublicUserResponse `json:"author"`
	CreatedAt time.Time          `json:"createdAt"`
}

type PostDetailResponse struct {
	ID        string             `json:"id"`
	Title     string             `json:"title"`
	Content   string             `json:"content"`
	Author    PublicUserResponse `json:"author"`
	CreatedAt time.Time          `json:"createdAt"`
	UpdatedAt time.Time          `json:"updatedAt"`
}

func PostListResponseFromModel(post *models.Post) PostListResponse {
	if post == nil {
		return PostListResponse{}
	}
	return PostListResponse{
		ID:        string(post.ID),
		Title:     post.Title,
		Author:    PublicUserResponseFromModel(post.Author),
		CreatedAt: post.CreatedAt,
	}
}

func PostListResponsesFromModels(posts []*models.Post) []PostListResponse {
	responses := make([]PostListResponse, 0, len(posts))
	for _, post := range posts {
		if post == nil {
			continue
		}
		responses = append(responses, PostListResponseFromModel(post))
	}
	return responses
}

func PostDetailResponseFromModel(post *models.Post) PostDetailResponse {
	if post == nil {
		return PostDetailResponse{}
	}
	return PostDetailResponse{
		ID:        string(post.ID),
		Title:     post.Title,
		Content:   post.Content,
		Author:    PublicUserResponseFromModel(post.Author),
		CreatedAt: post.CreatedAt,
		UpdatedAt: post.UpdatedAt,
	}
}
