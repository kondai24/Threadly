package dto

import (
	"time"

	"Threadly/internal/domain/models"
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
	// repliesをnilではなく空配列で返し、Frontが親Commentと返信を同じ契約で描画できるようにする。
	responses := make([]CommentResponse, 0, len(comments))
	for _, comment := range comments {
		if comment == nil {
			continue
		}
		responses = append(responses, CommentResponseFromModel(comment))
	}
	return responses
}
