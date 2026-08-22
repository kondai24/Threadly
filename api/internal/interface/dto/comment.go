package dto

import (
	"time"

	"Threadly/internal/domain/models"
	"Threadly/internal/usecase/services"
)

type CreateCommentRequest struct {
	Content  string  `json:"content" binding:"required" minLength:"1" maxLength:"1000" example:"コメント本文"`
	ParentID *string `json:"parentId,omitempty" example:"33333333-3333-4333-8333-333333333333"`
}

type UpdateCommentRequest struct {
	Content string `json:"content" binding:"required" minLength:"1" maxLength:"1000" example:"更新したコメント本文"`
}

type CommentResponse struct {
	ID        string             `json:"id"`
	Content   string             `json:"content"`
	Author    PublicUserResponse `json:"author"`
	CreatedAt time.Time          `json:"createdAt"`
	UpdatedAt time.Time          `json:"updatedAt"`
	LikeCount int64              `json:"likeCount"`
	LikedByMe bool               `json:"likedByMe"`
	Replies   []CommentResponse  `json:"replies"`
}

func CommentResponseFromModel(comment *models.Comment) CommentResponse {
	if comment == nil {
		return CommentResponse{}
	}
	return CommentResponse{
		ID:        string(comment.ID),
		Content:   comment.Content,
		Author:    PublicUserResponseFromModel(comment.Author),
		CreatedAt: comment.CreatedAt,
		UpdatedAt: comment.UpdatedAt,
		Replies:   CommentResponsesFromModels(comment.Replies),
	}
}

func CommentResponsesFromModels(comments []*models.Comment) []CommentResponse {
	return commentResponsesFromModels(comments, nil)
}

func CommentResponsesFromRead(read services.CommentListRead) []CommentResponse {
	return commentResponsesFromModels(read.Comments, read.Summaries)
}

func commentResponsesFromModels(
	comments []*models.Comment,
	summaries map[models.UUID]models.LikeSummary,
) []CommentResponse {
	// repliesをnilではなく空配列で返し、Frontが親Commentと返信を同じ契約で描画できるようにする。
	responses := make([]CommentResponse, 0, len(comments))
	for _, comment := range comments {
		if comment == nil {
			continue
		}
		responses = append(responses, commentResponseFromModel(comment, summaries))
	}
	return responses
}

func commentResponseFromModel(
	comment *models.Comment,
	summaries map[models.UUID]models.LikeSummary,
) CommentResponse {
	if comment == nil {
		return CommentResponse{}
	}
	response := CommentResponseFromModel(comment)
	response.LikeCount = summaries[comment.ID].Count
	response.LikedByMe = summaries[comment.ID].LikedByMe
	response.Replies = commentResponsesFromModels(comment.Replies, summaries)
	return response
}
