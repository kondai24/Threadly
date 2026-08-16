package routes

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"
	"Threadly/internal/interface/controllers"
	"Threadly/internal/interface/dto"
	"Threadly/internal/usecase/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type likeRouteKey struct {
	userID   models.UUID
	targetID models.UUID
}

type likeRoutePostRepository struct {
	*commentRoutePostRepository
}

func (r *likeRoutePostRepository) GetByID(
	ctx context.Context,
	postID models.UUID,
) (*models.Post, error) {
	post, err := r.commentRoutePostRepository.GetByID(ctx, postID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, repositories.ErrPostNotFound
	}
	return post, err
}

func (r *likeRoutePostRepository) GetByIDForOwner(
	ctx context.Context,
	userID models.UUID,
	postID models.UUID,
) (*models.Post, error) {
	post, err := r.commentRoutePostRepository.GetByIDForOwner(ctx, userID, postID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, repositories.ErrPostNotFound
	}
	return post, err
}

type likeRoutePostLikeRepository struct {
	likes map[likeRouteKey]struct{}
}

func (r *likeRoutePostLikeRepository) Ensure(_ context.Context, userID, postID models.UUID) error {
	r.likes[likeRouteKey{userID: userID, targetID: postID}] = struct{}{}
	return nil
}

func (r *likeRoutePostLikeRepository) Delete(_ context.Context, userID, postID models.UUID) error {
	delete(r.likes, likeRouteKey{userID: userID, targetID: postID})
	return nil
}

func (r *likeRoutePostLikeRepository) CountByPostIDs(
	_ context.Context,
	postIDs []models.UUID,
) (map[models.UUID]int64, error) {
	counts := make(map[models.UUID]int64, len(postIDs))
	for key := range r.likes {
		for _, postID := range postIDs {
			if key.targetID == postID {
				counts[postID]++
			}
		}
	}
	return counts, nil
}

func (r *likeRoutePostLikeRepository) FindLikedPostIDs(
	_ context.Context,
	userID models.UUID,
	postIDs []models.UUID,
) (map[models.UUID]struct{}, error) {
	likedIDs := make(map[models.UUID]struct{})
	for _, postID := range postIDs {
		if _, ok := r.likes[likeRouteKey{userID: userID, targetID: postID}]; ok {
			likedIDs[postID] = struct{}{}
		}
	}
	return likedIDs, nil
}

type likeRouteCommentLikeRepository struct {
	likes map[likeRouteKey]struct{}
}

func (r *likeRouteCommentLikeRepository) Ensure(_ context.Context, userID, commentID models.UUID) error {
	r.likes[likeRouteKey{userID: userID, targetID: commentID}] = struct{}{}
	return nil
}

func (r *likeRouteCommentLikeRepository) Delete(_ context.Context, userID, commentID models.UUID) error {
	delete(r.likes, likeRouteKey{userID: userID, targetID: commentID})
	return nil
}

func (r *likeRouteCommentLikeRepository) CountByCommentIDs(
	_ context.Context,
	commentIDs []models.UUID,
) (map[models.UUID]int64, error) {
	counts := make(map[models.UUID]int64, len(commentIDs))
	for key := range r.likes {
		for _, commentID := range commentIDs {
			if key.targetID == commentID {
				counts[commentID]++
			}
		}
	}
	return counts, nil
}

func (r *likeRouteCommentLikeRepository) FindLikedCommentIDs(
	_ context.Context,
	userID models.UUID,
	commentIDs []models.UUID,
) (map[models.UUID]struct{}, error) {
	likedIDs := make(map[models.UUID]struct{})
	for _, commentID := range commentIDs {
		if _, ok := r.likes[likeRouteKey{userID: userID, targetID: commentID}]; ok {
			likedIDs[commentID] = struct{}{}
		}
	}
	return likedIDs, nil
}

func newLikeRouteRouter(store *commentRouteStore) *gin.Engine {
	tokenIssuer := routeTokenIssuer{}
	authService := services.NewAuthService(
		newRouteUserRepository(),
		routePasswordHasher{},
		tokenIssuer,
	)
	postRepo := &likeRoutePostRepository{
		commentRoutePostRepository: &commentRoutePostRepository{store: store},
	}
	commentRepo := &commentRouteCommentRepository{store: store}
	postLikeRepo := &likeRoutePostLikeRepository{likes: make(map[likeRouteKey]struct{})}
	commentLikeRepo := &likeRouteCommentLikeRepository{likes: make(map[likeRouteKey]struct{})}
	likeService := services.NewLikeService(postRepo, commentRepo, postLikeRepo, commentLikeRepo)
	return SetupRouter(Handlers{
		Auth: controllers.NewAuthController(authService),
		Post: controllers.NewPostController(
			services.NewPostServiceWithLikes(postRepo, likeService),
		),
		Comment: controllers.NewCommentController(
			services.NewCommentServiceWithLikes(commentRepo, postRepo, likeService),
		),
		Like:        controllers.NewLikeController(likeService),
		TokenIssuer: tokenIssuer,
	})
}

func seedLikeRouteData(store *commentRouteStore) {
	seedCommentRoutePost(store)
	store.comments[commentRouteRootID] = &models.Comment{
		UUIDBaseModel: models.UUIDBaseModel{ID: commentRouteRootID},
		PostID:        routePostID,
		AuthorID:      routeUserID,
		Author: models.User{
			UUIDBaseModel: models.UUIDBaseModel{ID: routeUserID},
			Username:      "user-" + string(routeUserID),
		},
		Content: "root",
	}
}

func TestLikeRoutesAreIdempotentAndExposeUserSpecificSummaries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("COOKIE_SECURE", "false")
	store := newCommentRouteStore()
	seedLikeRouteData(store)
	router := newLikeRouteRouter(store)
	postLikePath := "/api/posts/" + string(routePostID) + "/like"
	commentLikePath := "/api/comments/" + string(commentRouteRootID) + "/like"

	response := performRequest(router, http.MethodPut, postLikePath, "user-"+string(routeUserID), "")
	assertLikeResponse(t, response, routePostID, 1, true)
	response = performRequest(router, http.MethodPut, postLikePath, "user-"+string(routeUserID), "")
	assertLikeResponse(t, response, routePostID, 1, true)
	response = performRequest(router, http.MethodPut, postLikePath, "user-"+string(routeOtherUserID), "")
	assertLikeResponse(t, response, routePostID, 2, true)
	response = performRequest(router, http.MethodDelete, postLikePath, "user-"+string(routeUserID), "")
	assertLikeResponse(t, response, routePostID, 1, false)
	response = performRequest(router, http.MethodDelete, postLikePath, "user-"+string(routeUserID), "")
	assertLikeResponse(t, response, routePostID, 1, false)

	response = performRequest(router, http.MethodPut, commentLikePath, "user-"+string(routeOtherUserID), "")
	assertLikeResponse(t, response, commentRouteRootID, 1, true)
	response = performRequest(router, http.MethodDelete, commentLikePath, "user-"+string(routeOtherUserID), "")
	assertLikeResponse(t, response, commentRouteRootID, 0, false)

	response = performRequest(router, http.MethodGet, "/api/posts", "user-"+string(routeUserID), "")
	if response.Code != http.StatusOK {
		t.Fatalf("list posts status = %d, want 200", response.Code)
	}
	var posts []struct {
		ID        models.UUID `json:"id"`
		LikeCount int64       `json:"likeCount"`
		LikedByMe bool        `json:"likedByMe"`
	}
	if err := decodeJSON(response, &posts); err != nil {
		t.Fatalf("decode posts response: %v", err)
	}
	if len(posts) != 1 || posts[0].LikeCount != 1 || posts[0].LikedByMe {
		t.Fatalf("post likes = %+v, want count 1 and likedByMe false", posts)
	}

	response = performRequest(router, http.MethodGet, "/api/posts/"+string(routePostID)+"/comments", "user-"+string(routeUserID), "")
	if response.Code != http.StatusOK {
		t.Fatalf("list comments status = %d, want 200", response.Code)
	}
	var comments []struct {
		LikeCount int64 `json:"likeCount"`
		LikedByMe bool  `json:"likedByMe"`
	}
	if err := decodeJSON(response, &comments); err != nil {
		t.Fatalf("decode comments response: %v", err)
	}
	if len(comments) != 1 || comments[0].LikeCount != 0 || comments[0].LikedByMe {
		t.Fatalf("comment likes = %+v, want count 0 and likedByMe false", comments)
	}
}

func TestLikeRoutesValidateAuthenticationUUIDAndTarget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := newCommentRouteStore()
	seedLikeRouteData(store)
	router := newLikeRouteRouter(store)

	response := performRequest(router, http.MethodPut, "/api/posts/not-a-uuid/like", "", "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated invalid UUID status = %d, want 401", response.Code)
	}
	response = performRequest(router, http.MethodPut, "/api/posts/not-a-uuid/like", "user-"+string(routeUserID), "")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid UUID status = %d, want 400", response.Code)
	}
	response = performRequest(router, http.MethodPut, "/api/posts/99999999-9999-4999-8999-999999999999/like", "user-"+string(routeUserID), "")
	if response.Code != http.StatusNotFound {
		t.Fatalf("missing target status = %d, want 404", response.Code)
	}
}

func assertLikeResponse(
	t *testing.T,
	response *httptest.ResponseRecorder,
	targetID models.UUID,
	wantCount int64,
	wantLikedByMe bool,
) {
	t.Helper()
	if response.Code != http.StatusOK {
		t.Fatalf("like response status = %d, want 200: %s", response.Code, response.Body.String())
	}
	var body dto.LikeActionResponse
	if err := decodeJSON(response, &body); err != nil {
		t.Fatalf("decode like response: %v", err)
	}
	if body.TargetID != string(targetID) || body.LikeCount != wantCount || body.LikedByMe != wantLikedByMe {
		t.Fatalf("like response = %+v, want target=%s count=%d likedByMe=%t", body, targetID, wantCount, wantLikedByMe)
	}
}
