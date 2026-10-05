package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	commentRouteRootID  models.UUID = "77777777-7777-4777-8777-777777777777"
	commentRouteReplyID models.UUID = "88888888-8888-4888-8888-888888888888"
)

func TestSetupRouter_CommentsExposeThreadAndAuthenticatedAuthor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router, repos := newTestRouter(t)
	post := &models.Post{UUIDBaseModel: models.UUIDBaseModel{ID: routePostID}}
	repos.Post.EXPECT().GetByIDForUpdate(gomock.Any(), routePostID).Return(post, nil).Times(2)
	root := &models.Comment{
		UUIDBaseModel: models.UUIDBaseModel{ID: commentRouteRootID},
		PostID:        routePostID,
		AuthorID:      routeUserID,
		Author:        models.User{UUIDBaseModel: models.UUIDBaseModel{ID: routeUserID}, Username: "alice"},
		Content:       "root",
	}
	reply := &models.Comment{
		UUIDBaseModel: models.UUIDBaseModel{ID: commentRouteReplyID},
		PostID:        routePostID,
		ParentID:      &root.ID,
		Author:        models.User{UUIDBaseModel: models.UUIDBaseModel{ID: routeOtherUserID}, Username: "bob"},
		Content:       "reply",
	}
	repos.Comment.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, comment *models.Comment) error {
			require.Equal(t, routeUserID, comment.AuthorID)
			require.Equal(t, routePostID, comment.PostID)
			require.Equal(t, "root", comment.Content)
			require.Nil(t, comment.ParentID)
			return nil
		},
	)
	path := "/api/posts/" + string(routePostID) + "/comments"
	response := performRequest(
		router,
		http.MethodPost,
		path,
		"user-"+string(routeUserID),
		`{"content":"  root  ","authorId":"99999999-9999-4999-8999-999999999999"}`,
	)
	require.Equal(t, http.StatusCreated, response.Code)
	repos.Comment.EXPECT().GetByIDForUpdate(gomock.Any(), root.ID).Return(root, nil)
	repos.Comment.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, comment *models.Comment) error {
			require.Equal(t, routeOtherUserID, comment.AuthorID)
			require.Equal(t, &root.ID, comment.ParentID)
			require.Equal(t, "reply", comment.Content)
			return nil
		},
	)
	response = performRequest(
		router,
		http.MethodPost,
		path,
		"user-"+string(routeOtherUserID),
		`{"content":"reply","parentId":"77777777-7777-4777-8777-777777777777"}`,
	)
	require.Equal(t, http.StatusCreated, response.Code)
	root.Replies = []*models.Comment{reply}
	repos.Post.EXPECT().GetByID(gomock.Any(), routePostID).Return(post, nil)
	repos.Comment.EXPECT().ListByPostID(gomock.Any(), routePostID).Return([]*models.Comment{root}, nil)
	response = performRequest(router, http.MethodGet, path, "user-"+string(routeOtherUserID), "")
	require.Equal(t, http.StatusOK, response.Code)
	var comments []struct {
		ID      models.UUID             `json:"id"`
		Author  routePostAuthorResponse `json:"author"`
		Replies []struct {
			ID     models.UUID             `json:"id"`
			Author routePostAuthorResponse `json:"author"`
		} `json:"replies"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &comments))
	require.Len(t, comments, 1)
	require.Equal(t, root.ID, comments[0].ID)
	require.Equal(t, routeUserID, comments[0].Author.ID)
	require.Len(t, comments[0].Replies, 1)
	require.Equal(t, reply.ID, comments[0].Replies[0].ID)
	require.Equal(t, routeOtherUserID, comments[0].Replies[0].Author.ID)
	require.Equal(t, "bob", comments[0].Replies[0].Author.Username)

	repos.Comment.EXPECT().Update(gomock.Any(), routeUserID, root.ID, "updated").Return(int64(1), nil)
	response = performRequest(
		router,
		http.MethodPut,
		"/api/comments/"+string(root.ID),
		"user-"+string(routeUserID),
		`{"content":"  updated  "}`,
	)
	require.Equal(t, http.StatusOK, response.Code)
	repos.Comment.EXPECT().GetByIDForUpdate(gomock.Any(), root.ID).Return(root, nil)
	repos.CommentLike.EXPECT().DeleteByCommentIDWithReplies(gomock.Any(), root.ID).Return(nil)
	repos.Comment.EXPECT().DeleteByIDWithReplies(gomock.Any(), routeUserID, root.ID).Return(int64(2), nil)
	response = performRequest(router, http.MethodDelete, "/api/comments/"+string(root.ID), "user-"+string(routeUserID), "")
	require.Equal(t, http.StatusNoContent, response.Code)
}

func TestSetupRouter_CommentErrorsPreserveHTTPBoundaries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rootID := commentRouteRootID
	post := &models.Post{UUIDBaseModel: models.UUIDBaseModel{ID: routePostID}}
	cases := []struct {
		name   string
		method string
		path   string
		body   string
		setup  func(routeRepositories)
		status int
	}{
		{
			name:   "不正なparent IDを400にする",
			method: http.MethodPost,
			path:   "/api/posts/" + string(routePostID) + "/comments",
			body:   `{"content":"reply","parentId":"not-a-uuid"}`,
			status: http.StatusBadRequest,
		},
		{
			name:   "返信への返信を400にする",
			method: http.MethodPost,
			path:   "/api/posts/" + string(routePostID) + "/comments",
			body:   `{"content":"reply","parentId":"88888888-8888-4888-8888-888888888888"}`,
			setup: func(r routeRepositories) {
				r.Post.EXPECT().GetByIDForUpdate(gomock.Any(), routePostID).Return(post, nil)
				r.Comment.EXPECT().GetByIDForUpdate(gomock.Any(), commentRouteReplyID).Return(&models.Comment{
					PostID: routePostID, ParentID: &rootID,
				}, nil)
			},
			status: http.StatusBadRequest,
		},
		{
			name:   "削除済みの親への返信を404にする",
			method: http.MethodPost,
			path:   "/api/posts/" + string(routePostID) + "/comments",
			body:   `{"content":"reply","parentId":"77777777-7777-4777-8777-777777777777"}`,
			setup: func(r routeRepositories) {
				r.Post.EXPECT().GetByIDForUpdate(gomock.Any(), routePostID).Return(post, nil)
				r.Comment.EXPECT().GetByIDForUpdate(gomock.Any(), commentRouteRootID).
					Return(nil, repositories.ErrCommentNotFound)
			},
			status: http.StatusNotFound,
		},
		{
			name:   "他UserのComment更新を404にする",
			method: http.MethodPut,
			path:   "/api/comments/" + string(commentRouteRootID),
			body:   `{"content":"tampered"}`,
			setup: func(r routeRepositories) {
				r.Comment.EXPECT().Update(gomock.Any(), routeOtherUserID, commentRouteRootID, "tampered").
					Return(int64(0), repositories.ErrCommentNotFound)
			},
			status: http.StatusNotFound,
		},
		{
			name:   "削除済みPostのComment一覧を404にする",
			method: http.MethodGet,
			path:   "/api/posts/" + string(routePostID) + "/comments",
			setup: func(r routeRepositories) {
				r.Post.EXPECT().GetByID(gomock.Any(), routePostID).Return(nil, repositories.ErrPostNotFound)
			},
			status: http.StatusNotFound,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			router, repos := newTestRouter(t)
			if tt.setup != nil {
				tt.setup(repos)
			}
			response := performRequest(router, tt.method, tt.path, "user-"+string(routeOtherUserID), tt.body)
			require.Equal(t, tt.status, response.Code)
		})
	}
}
