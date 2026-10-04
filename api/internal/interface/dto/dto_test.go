package dto

import (
	"encoding/json"
	"testing"
	"time"

	"Threadly/internal/domain/models"
	"Threadly/internal/usecase"

	"github.com/stretchr/testify/require"
)

func TestResponsesExposePublicJSONContracts(t *testing.T) {
	timestamp := time.Date(2026, 8, 16, 4, 0, 0, 0, time.UTC)
	user := models.User{
		UUIDBaseModel: models.UUIDBaseModel{
			ID: "11111111-1111-4111-8111-111111111111", CreatedAt: timestamp, UpdatedAt: timestamp,
		},
		Username:     "alice",
		PasswordHash: "must-not-leak",
	}
	post := &models.Post{
		UUIDBaseModel: models.UUIDBaseModel{
			ID: "77777777-7777-4777-8777-777777777777", CreatedAt: timestamp, UpdatedAt: timestamp,
		},
		Author: user, Title: "title", Content: "content",
	}
	reply := &models.Comment{
		UUIDBaseModel: models.UUIDBaseModel{
			ID: "99999999-9999-4999-8999-999999999999", CreatedAt: timestamp, UpdatedAt: timestamp,
		},
		Author: user, Content: "reply",
	}
	comment := &models.Comment{
		UUIDBaseModel: models.UUIDBaseModel{
			ID: "88888888-8888-4888-8888-888888888888", CreatedAt: timestamp, UpdatedAt: timestamp,
		},
		Author: user, Content: "comment", Replies: []*models.Comment{reply},
	}
	cases := []struct {
		name     string
		response any
		json     string
	}{
		{
			name:     "Userはhashを公開しない",
			response: UserResponseFromModel(&user),
			json: `{"id":"11111111-1111-4111-8111-111111111111","username":"alice",
    "createdAt":"2026-08-16T04:00:00Z","updatedAt":"2026-08-16T04:00:00Z"}`,
		},
		{
			name: "Post一覧は公開投稿者とLike集計を返す",
			response: PostListResponsesFromReads([]usecase.PostRead{{
				Post: post, Summary: models.LikeSummary{Count: 3, LikedByMe: true},
			}}),
			json: `[{"id":"77777777-7777-4777-8777-777777777777","title":"title",
    "author":{"id":"11111111-1111-4111-8111-111111111111","username":"alice"},
    "createdAt":"2026-08-16T04:00:00Z","likeCount":3,"likedByMe":true}]`,
		},
		{
			name: "Post詳細は本文と更新日時を返す",
			response: PostDetailResponseFromRead(usecase.PostRead{
				Post: post, Summary: models.LikeSummary{Count: 3, LikedByMe: true},
			}),
			json: `{"id":"77777777-7777-4777-8777-777777777777","title":"title","content":"content",
    "author":{"id":"11111111-1111-4111-8111-111111111111","username":"alice"},
    "createdAt":"2026-08-16T04:00:00Z","updatedAt":"2026-08-16T04:00:00Z","likeCount":3,"likedByMe":true}`,
		},
		{
			name: "Commentと返信は個別のLike集計と空Repliesを返す",
			response: CommentResponsesFromRead(usecase.CommentListRead{
				Comments: []*models.Comment{comment},
				Summaries: map[models.UUID]models.LikeSummary{
					comment.ID: {Count: 2, LikedByMe: true},
					reply.ID:   {Count: 1},
				},
			}),
			json: `[{"id":"88888888-8888-4888-8888-888888888888","content":"comment",
    "author":{"id":"11111111-1111-4111-8111-111111111111","username":"alice"},
    "createdAt":"2026-08-16T04:00:00Z","updatedAt":"2026-08-16T04:00:00Z","likeCount":2,"likedByMe":true,
    "replies":[{"id":"99999999-9999-4999-8999-999999999999","content":"reply",
    "author":{"id":"11111111-1111-4111-8111-111111111111","username":"alice"},
    "createdAt":"2026-08-16T04:00:00Z","updatedAt":"2026-08-16T04:00:00Z","likeCount":1,"likedByMe":false,
    "replies":[]}]}]`,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(tt.response)
			require.NoError(t, err)
			require.JSONEq(t, tt.json, string(body))
		})
	}
}
