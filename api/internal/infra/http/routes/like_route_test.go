package routes

import (
	"encoding/json"
	"net/http"
	"testing"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"
	"Threadly/internal/interface/dto"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestLikeRoutesExposeActionAndCurrentUserSummary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	targets := []struct {
		name string
		id   models.UUID
		path string
		post bool
	}{
		{name: "Post", id: routePostID, path: "/api/posts/" + string(routePostID) + "/like", post: true},
		{name: "Comment", id: commentRouteRootID, path: "/api/comments/" + string(commentRouteRootID) + "/like"},
	}
	for _, target := range targets {
		t.Run(target.name, func(t *testing.T) {
			router, repos := newRouteRouter(t, true)
			post := &models.Post{UUIDBaseModel: models.UUIDBaseModel{ID: routePostID}}
			for _, method := range []string{http.MethodPut, http.MethodDelete} {
				liked := method == http.MethodPut
				count := int64(2)
				likedIDs := map[models.UUID]struct{}{}
				if liked {
					count++
					likedIDs[target.id] = struct{}{}
				}
				ids := []models.UUID{target.id}
				repos.Post.EXPECT().GetByID(gomock.Any(), routePostID).Return(post, nil)
				if target.post {
					if liked {
						repos.PostLike.EXPECT().Ensure(gomock.Any(), routeOtherUserID, target.id).Return(nil)
					} else {
						repos.PostLike.EXPECT().Delete(gomock.Any(), routeOtherUserID, target.id).Return(nil)
					}
					repos.PostLike.EXPECT().CountByPostIDs(gomock.Any(), ids).Return(map[models.UUID]int64{target.id: count}, nil)
					repos.PostLike.EXPECT().FindLikedPostIDs(gomock.Any(), routeOtherUserID, ids).Return(likedIDs, nil)
				} else {
					repos.Comment.EXPECT().GetByID(gomock.Any(), target.id).
						Return(&models.Comment{PostID: routePostID}, nil)
					if liked {
						repos.CommentLike.EXPECT().Ensure(gomock.Any(), routeOtherUserID, target.id).Return(nil)
					} else {
						repos.CommentLike.EXPECT().Delete(gomock.Any(), routeOtherUserID, target.id).Return(nil)
					}
					repos.CommentLike.EXPECT().CountByCommentIDs(gomock.Any(), ids).Return(map[models.UUID]int64{target.id: count}, nil)
					repos.CommentLike.EXPECT().FindLikedCommentIDs(gomock.Any(), routeOtherUserID, ids).Return(likedIDs, nil)
				}
				response := performRequest(router, method, target.path, "user-"+string(routeOtherUserID), "")
				require.Equal(t, http.StatusOK, response.Code)
				var body dto.LikeActionResponse
				require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
				require.Equal(t, dto.LikeActionResponse{TargetID: string(target.id), LikeCount: count, LikedByMe: liked}, body)
			}
		})
	}
}

func TestLikeRoutesIncludeSummariesInLists(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router, repos := newRouteRouter(t, true)
	post := &models.Post{UUIDBaseModel: models.UUIDBaseModel{ID: routePostID}}
	postIDs := []models.UUID{post.ID}
	repos.Post.EXPECT().ListAll(gomock.Any()).Return([]*models.Post{post}, nil)
	repos.PostLike.EXPECT().CountByPostIDs(gomock.Any(), postIDs).Return(map[models.UUID]int64{post.ID: 2}, nil)
	repos.PostLike.EXPECT().FindLikedPostIDs(gomock.Any(), routeUserID, postIDs).Return(map[models.UUID]struct{}{}, nil)
	response := performRequest(router, http.MethodGet, "/api/posts", "user-"+string(routeUserID), "")
	require.Equal(t, http.StatusOK, response.Code)
	var posts []dto.PostListResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &posts))
	require.Len(t, posts, 1)
	require.Equal(t, int64(2), posts[0].LikeCount)
	require.False(t, posts[0].LikedByMe)

	comment := &models.Comment{UUIDBaseModel: models.UUIDBaseModel{ID: commentRouteRootID}}
	ids := []models.UUID{comment.ID}
	repos.Post.EXPECT().GetByID(gomock.Any(), post.ID).Return(post, nil)
	repos.Comment.EXPECT().ListByPostID(gomock.Any(), post.ID).Return([]*models.Comment{comment}, nil)
	repos.CommentLike.EXPECT().CountByCommentIDs(gomock.Any(), ids).Return(map[models.UUID]int64{comment.ID: 3}, nil)
	repos.CommentLike.EXPECT().FindLikedCommentIDs(gomock.Any(), routeUserID, ids).
		Return(map[models.UUID]struct{}{comment.ID: {}}, nil)
	response = performRequest(
		router,
		http.MethodGet,
		"/api/posts/"+string(post.ID)+"/comments",
		"user-"+string(routeUserID),
		"",
	)
	require.Equal(t, http.StatusOK, response.Code)
	var comments []dto.CommentResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &comments))
	require.Len(t, comments, 1)
	require.Equal(t, int64(3), comments[0].LikeCount)
	require.True(t, comments[0].LikedByMe)
}

func TestLikeRoutesValidateUUIDAndTarget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router, repos := newRouteRouter(t, true)
	response := performRequest(router, http.MethodPut, "/api/posts/not-a-uuid/like", "user-"+string(routeUserID), "")
	require.Equal(t, http.StatusBadRequest, response.Code)
	missingID := models.UUID("99999999-9999-4999-8999-999999999999")
	repos.Post.EXPECT().GetByID(gomock.Any(), missingID).Return(nil, repositories.ErrPostNotFound)
	response = performRequest(
		router,
		http.MethodPut,
		"/api/posts/"+string(missingID)+"/like",
		"user-"+string(routeUserID),
		"",
	)
	require.Equal(t, http.StatusNotFound, response.Code)
}
