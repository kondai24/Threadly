package routes

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"
	"Threadly/internal/interface/controllers"
	"Threadly/internal/usecase/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type routePostRepository struct {
	posts  map[uint]*models.Post
	nextID uint
}

func newRoutePostRepository() *routePostRepository {
	return &routePostRepository{
		posts:  make(map[uint]*models.Post),
		nextID: 1,
	}
}

func (r *routePostRepository) GetByID(_ context.Context, postID uint) (*models.Post, error) {
	post, ok := r.posts[postID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return clonePost(post), nil
}

func (r *routePostRepository) GetByIDForOwner(_ context.Context, userID uint, id uint) (*models.Post, error) {
	post, ok := r.posts[id]
	if !ok || post.AuthorID != userID {
		return nil, gorm.ErrRecordNotFound
	}
	return clonePost(post), nil
}

func (r *routePostRepository) Create(_ context.Context, post *models.Post) error {
	post.ID = r.nextID
	r.nextID++
	post.Author = models.User{
		BaseModel: models.BaseModel{ID: post.AuthorID},
		Username:  "user-" + strconv.FormatUint(uint64(post.AuthorID), 10),
	}
	r.posts[post.ID] = clonePost(post)
	return nil
}

func (r *routePostRepository) Update(_ context.Context, userID uint, post *models.Post) error {
	stored, ok := r.posts[post.ID]
	if !ok || stored.AuthorID != userID {
		return gorm.ErrRecordNotFound
	}
	stored.Title = post.Title
	stored.Content = post.Content
	return nil
}

func (r *routePostRepository) DeleteByID(_ context.Context, userID uint, postID uint) (int64, error) {
	post, ok := r.posts[postID]
	if !ok || post.AuthorID != userID {
		return 0, nil
	}
	delete(r.posts, postID)
	return 1, nil
}

func (r *routePostRepository) ListAll(_ context.Context) ([]*models.Post, error) {
	posts := make([]*models.Post, 0)
	for _, post := range r.posts {
		posts = append(posts, clonePost(post))
	}
	return posts, nil
}

type routeUserRepository struct{}

func (routeUserRepository) FindByUsername(context.Context, string) (*models.User, error) {
	return nil, repositories.ErrUserNotFound
}

func (routeUserRepository) FindByID(context.Context, uint) (*models.User, error) {
	return nil, repositories.ErrUserNotFound
}

func (routeUserRepository) Create(context.Context, *models.User) error {
	return errors.New("not used")
}

type routePasswordHasher struct{}

func (routePasswordHasher) Hash(string) (string, error) {
	return "hash", nil
}

func (routePasswordHasher) Compare(string, string) error {
	return nil
}

type routeTokenIssuer struct{}

func (routeTokenIssuer) Issue(userID uint) (string, error) {
	return "user-" + strconv.FormatUint(uint64(userID), 10), nil
}

func (routeTokenIssuer) Parse(rawToken string) (uint, error) {
	if !strings.HasPrefix(rawToken, "user-") {
		return 0, services.ErrInvalidToken
	}
	userID, err := strconv.ParseUint(strings.TrimPrefix(rawToken, "user-"), 10, 64)
	if err != nil || userID == 0 {
		return 0, services.ErrInvalidToken
	}
	return uint(userID), nil
}

func clonePost(post *models.Post) *models.Post {
	cloned := *post
	return &cloned
}

func newTestRouter(postRepo *routePostRepository) *gin.Engine {
	tokenIssuer := routeTokenIssuer{}
	authService := services.NewAuthService(
		routeUserRepository{},
		routePasswordHasher{},
		tokenIssuer,
	)
	postService := services.NewPostService(postRepo)
	return SetupRouter(Handlers{
		Auth:        controllers.NewAuthController(authService),
		Post:        controllers.NewPostController(postService),
		TokenIssuer: tokenIssuer,
	})
}

type routePostAuthorResponse struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}

type routePostResponse struct {
	ID      uint                    `json:"id"`
	Title   string                  `json:"title"`
	Content string                  `json:"content"`
	Author  routePostAuthorResponse `json:"author"`
}

func TestSetupRouter_PostsAreReadableByAllAuthenticatedUsers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	postRepo := newRoutePostRepository()
	router := newTestRouter(postRepo)

	response := performRequest(router, http.MethodGet, "/api/posts", "", "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated list status = %d, want 401", response.Code)
	}

	response = performRequest(
		router,
		http.MethodPost,
		"/api/posts",
		"user-1",
		`{"title":"owned","content":"content","authorId":999}`,
	)
	if response.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201", response.Code)
	}
	if postRepo.posts[1].AuthorID != 1 {
		t.Fatalf("stored author ID = %d, want token user ID 1", postRepo.posts[1].AuthorID)
	}

	response = performRequest(router, http.MethodGet, "/api/posts", "user-1", "")
	if response.Code != http.StatusOK {
		t.Fatalf("owner list status = %d, want 200", response.Code)
	}
	var ownerPosts []routePostResponse
	if err := json.Unmarshal(response.Body.Bytes(), &ownerPosts); err != nil {
		t.Fatalf("decode owner list: %v", err)
	}
	if len(ownerPosts) != 1 {
		t.Fatalf("owner list length = %d, want 1", len(ownerPosts))
	}
	if ownerPosts[0].Author.ID != 1 || ownerPosts[0].Author.Username != "user-1" {
		t.Fatalf("owner list author = %+v, want user-1", ownerPosts[0].Author)
	}

	response = performRequest(router, http.MethodGet, "/api/posts", "user-2", "")
	if response.Code != http.StatusOK {
		t.Fatalf("other user list status = %d, want 200", response.Code)
	}
	var otherPosts []routePostResponse
	if err := json.Unmarshal(response.Body.Bytes(), &otherPosts); err != nil {
		t.Fatalf("decode other user list: %v", err)
	}
	if len(otherPosts) != 1 {
		t.Fatalf("other user list length = %d, want 1", len(otherPosts))
	}

	response = performRequest(router, http.MethodGet, "/api/posts/1", "user-2", "")
	if response.Code != http.StatusOK {
		t.Fatalf("other user detail status = %d, want 200", response.Code)
	}
	var otherPost routePostResponse
	if err := json.Unmarshal(response.Body.Bytes(), &otherPost); err != nil {
		t.Fatalf("decode other user detail: %v", err)
	}
	if otherPost.Author.ID != 1 || otherPost.Author.Username != "user-1" {
		t.Fatalf("other user detail author = %+v, want user-1", otherPost.Author)
	}

	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		response = performRequest(router, method, "/api/posts/1", "user-2", `{"title":"tampered"}`)
		if response.Code != http.StatusNotFound {
			t.Fatalf("other user %s status = %d, want 404", method, response.Code)
		}
	}
}

func performRequest(
	router http.Handler,
	method string,
	path string,
	token string,
	body string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}
