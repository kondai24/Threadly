package dto

import (
	"time"

	"Threadly/internal/domain/models"
	"Threadly/internal/usecase/services"
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
	LikeCount int64              `json:"likeCount"`
	LikedByMe bool               `json:"likedByMe"`
}

type PostDetailResponse struct {
	ID        string             `json:"id"`
	Title     string             `json:"title"`
	Content   string             `json:"content"`
	Author    PublicUserResponse `json:"author"`
	CreatedAt time.Time          `json:"createdAt"`
	UpdatedAt time.Time          `json:"updatedAt"`
	LikeCount int64              `json:"likeCount"`
	LikedByMe bool               `json:"likedByMe"`
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

func PostListResponseFromRead(read services.PostRead) PostListResponse {
	response := PostListResponseFromModel(read.Post)
	response.LikeCount = read.Summary.Count
	response.LikedByMe = read.Summary.LikedByMe
	return response
}

func PostListResponsesFromReads(reads []services.PostRead) []PostListResponse {
	responses := make([]PostListResponse, 0, len(reads))
	for _, read := range reads {
		if read.Post == nil {
			continue
		}
		responses = append(responses, PostListResponseFromRead(read))
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

func PostDetailResponseFromRead(read services.PostRead) PostDetailResponse {
	response := PostDetailResponseFromModel(read.Post)
	response.LikeCount = read.Summary.Count
	response.LikedByMe = read.Summary.LikedByMe
	return response
}
