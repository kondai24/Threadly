package dto

import (
	"testing"
	"time"

	"Threadly/internal/domain/models"
)

func TestPostListResponsesFromModelsUsesPublicFields(t *testing.T) {
	createdAt := time.Date(2026, 8, 16, 4, 0, 0, 0, time.UTC)
	posts := []*models.Post{
		{
			UUIDBaseModel: models.UUIDBaseModel{ID: "77777777-7777-4777-8777-777777777777", CreatedAt: createdAt},
			Author: models.User{
				UUIDBaseModel: models.UUIDBaseModel{ID: "11111111-1111-4111-8111-111111111111"},
				Username:      "alice",
				PasswordHash:  "must-not-leak",
			},
			Title:   "title",
			Content: "content",
		},
	}

	responses := PostListResponsesFromModels(posts)
	if len(responses) != 1 {
		t.Fatalf("response length = %d, want 1", len(responses))
	}
	if responses[0].ID != string(posts[0].ID) || responses[0].Author.ID != string(posts[0].Author.ID) {
		t.Fatalf("response IDs = %+v, want post and author IDs", responses[0])
	}
	if responses[0].Author.Username != "alice" || responses[0].CreatedAt != createdAt {
		t.Fatalf("response public fields = %+v", responses[0])
	}
}

func TestCommentResponseFromModelInitializesEmptyReplies(t *testing.T) {
	comment := &models.Comment{
		UUIDBaseModel: models.UUIDBaseModel{ID: "77777777-7777-4777-8777-777777777777"},
		Author: models.User{
			UUIDBaseModel: models.UUIDBaseModel{ID: "11111111-1111-4111-8111-111111111111"},
			Username:      "alice",
			PasswordHash:  "must-not-leak",
		},
		Content: "comment",
	}

	response := CommentResponseFromModel(comment)
	if response.Replies == nil {
		t.Fatal("replies is nil, want an empty slice")
	}
	if len(response.Replies) != 0 {
		t.Fatalf("replies length = %d, want 0", len(response.Replies))
	}
	if response.Author.Username != "alice" || response.ID != string(comment.ID) {
		t.Fatalf("response public fields = %+v", response)
	}
}
