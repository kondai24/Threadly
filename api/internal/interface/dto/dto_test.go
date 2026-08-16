package dto

import (
	"testing"
	"time"

	"Threadly/internal/domain/models"
	"Threadly/internal/usecase/services"
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

func TestResponsesIncludeLikeSummaryWithoutInternalFields(t *testing.T) {
	post := &models.Post{
		UUIDBaseModel: models.UUIDBaseModel{ID: "77777777-7777-4777-8777-777777777777"},
		Author: models.User{
			UUIDBaseModel: models.UUIDBaseModel{ID: "11111111-1111-4111-8111-111111111111"},
			Username:      "alice",
			PasswordHash:  "must-not-leak",
		},
		Title: "title",
	}
	response := PostListResponseFromRead(services.PostRead{
		Post:    post,
		Summary: models.LikeSummary{Count: 3, LikedByMe: true},
	})
	if response.LikeCount != 3 || !response.LikedByMe {
		t.Fatalf("post like summary = %+v", response)
	}

	comment := &models.Comment{
		UUIDBaseModel: models.UUIDBaseModel{ID: "88888888-8888-4888-8888-888888888888"},
		Author:        post.Author,
		Content:       "comment",
	}
	commentResponses := CommentResponsesFromRead(services.CommentListRead{
		Comments: []*models.Comment{comment},
		Summaries: map[models.UUID]models.LikeSummary{
			comment.ID: {Count: 2, LikedByMe: true},
		},
	})
	if len(commentResponses) != 1 || commentResponses[0].LikeCount != 2 || !commentResponses[0].LikedByMe {
		t.Fatalf("comment like summary = %+v", commentResponses)
	}
	if commentResponses[0].Replies == nil {
		t.Fatal("replies is nil, want an empty slice")
	}
}
