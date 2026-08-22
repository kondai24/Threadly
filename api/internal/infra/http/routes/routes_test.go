package routes

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"
	"Threadly/internal/interface/controllers"
	"Threadly/internal/middleware"
	"Threadly/internal/usecase/services"

	"github.com/gin-gonic/gin"
)

type routePostRepository struct {
	posts map[models.UUID]*models.Post
}

const (
	routeUserID      models.UUID = "11111111-1111-4111-8111-111111111111"
	routeOtherUserID models.UUID = "22222222-2222-4222-8222-222222222222"
	routePostID      models.UUID = "33333333-3333-4333-8333-333333333333"
)

func newRoutePostRepository() *routePostRepository {
	return &routePostRepository{
		posts: make(map[models.UUID]*models.Post),
	}
}

func (r *routePostRepository) GetByID(_ context.Context, postID models.UUID) (*models.Post, error) {
	post, ok := r.posts[postID]
	if !ok {
		return nil, repositories.ErrPostNotFound
	}
	return clonePost(post), nil
}

func (r *routePostRepository) GetByIDForUpdate(
	ctx context.Context,
	postID models.UUID,
) (*models.Post, error) {
	return r.GetByID(ctx, postID)
}

func (r *routePostRepository) GetByIDForOwner(_ context.Context, userID models.UUID, id models.UUID) (*models.Post, error) {
	post, ok := r.posts[id]
	if !ok || post.AuthorID != userID {
		return nil, repositories.ErrPostNotFound
	}
	return clonePost(post), nil
}

func (r *routePostRepository) Create(_ context.Context, post *models.Post) error {
	post.ID = routePostID
	post.Author = models.User{
		UUIDBaseModel: models.UUIDBaseModel{ID: post.AuthorID},
		Username:      "user-" + string(post.AuthorID),
	}
	r.posts[post.ID] = clonePost(post)
	return nil
}

func (r *routePostRepository) Update(_ context.Context, userID models.UUID, post *models.Post) error {
	stored, ok := r.posts[post.ID]
	if !ok || stored.AuthorID != userID {
		return repositories.ErrPostNotFound
	}
	stored.Title = post.Title
	stored.Content = post.Content
	return nil
}

func (r *routePostRepository) DeleteByID(_ context.Context, userID models.UUID, postID models.UUID) (int64, error) {
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

type routeUserRepository struct {
	users map[models.UUID]*models.User
}

func newRouteUserRepository() *routeUserRepository {
	return &routeUserRepository{
		users: make(map[models.UUID]*models.User),
	}
}

func (r *routeUserRepository) FindByUsername(_ context.Context, username string) (*models.User, error) {
	for _, user := range r.users {
		if user.Username == username {
			return cloneUser(user), nil
		}
	}
	return nil, repositories.ErrUserNotFound
}

func (r *routeUserRepository) FindByID(_ context.Context, userID models.UUID) (*models.User, error) {
	user, ok := r.users[userID]
	if !ok {
		return nil, repositories.ErrUserNotFound
	}
	return cloneUser(user), nil
}

func (r *routeUserRepository) Create(_ context.Context, user *models.User) error {
	for _, stored := range r.users {
		if stored.Username == user.Username {
			return repositories.ErrUsernameAlreadyExists
		}
	}

	now := time.Now()
	user.ID = routeUserID
	user.CreatedAt = now
	user.UpdatedAt = now
	r.users[user.ID] = cloneUser(user)
	return nil
}

type routePasswordHasher struct{}

func (routePasswordHasher) Hash(password string) (string, error) {
	return routeHashPassword(password), nil
}

func (routePasswordHasher) Compare(encodedHash string, password string) error {
	actualHash := routeHashPassword(password)
	if subtle.ConstantTimeCompare([]byte(encodedHash), []byte(actualHash)) != 1 {
		return services.ErrPasswordMismatch
	}
	return nil
}

func routeHashPassword(password string) string {
	passwordHash := sha256.Sum256([]byte(password))
	return string(passwordHash[:])
}

type routeTokenIssuer struct{}

type routePostLikeRepository struct{}

func (routePostLikeRepository) Ensure(context.Context, models.UUID, models.UUID) error {
	return nil
}

func (routePostLikeRepository) Delete(context.Context, models.UUID, models.UUID) error {
	return nil
}

func (routePostLikeRepository) DeleteByPostID(context.Context, models.UUID) error {
	return nil
}

func (routePostLikeRepository) CountByPostIDs(
	context.Context,
	[]models.UUID,
) (map[models.UUID]int64, error) {
	return map[models.UUID]int64{}, nil
}

func (routePostLikeRepository) FindLikedPostIDs(
	context.Context,
	models.UUID,
	[]models.UUID,
) (map[models.UUID]struct{}, error) {
	return map[models.UUID]struct{}{}, nil
}

type routeCommentLikeRepository struct{}

func (routeCommentLikeRepository) Ensure(context.Context, models.UUID, models.UUID) error {
	return nil
}

func (routeCommentLikeRepository) Delete(context.Context, models.UUID, models.UUID) error {
	return nil
}

func (routeCommentLikeRepository) DeleteByCommentIDs(context.Context, []models.UUID) error {
	return nil
}

func (routeCommentLikeRepository) DeleteByCommentIDWithReplies(context.Context, models.UUID) error {
	return nil
}

func (routeCommentLikeRepository) DeleteByCommentsOfPostID(context.Context, models.UUID) error {
	return nil
}

func (routeCommentLikeRepository) CountByCommentIDs(
	context.Context,
	[]models.UUID,
) (map[models.UUID]int64, error) {
	return map[models.UUID]int64{}, nil
}

func (routeCommentLikeRepository) FindLikedCommentIDs(
	context.Context,
	models.UUID,
	[]models.UUID,
) (map[models.UUID]struct{}, error) {
	return map[models.UUID]struct{}{}, nil
}

type routeUnitOfWork struct {
	post        repositories.PostRepository
	comment     repositories.CommentRepository
	postLike    repositories.PostLikeRepository
	commentLike repositories.CommentLikeRepository
}

func (u routeUnitOfWork) WithinTransaction(
	ctx context.Context,
	fn func(repositories.TransactionRepositories) error,
) error {
	postLike := u.postLike
	if postLike == nil {
		postLike = routePostLikeRepository{}
	}
	commentLike := u.commentLike
	if commentLike == nil {
		commentLike = routeCommentLikeRepository{}
	}
	return fn(repositories.TransactionRepositories{
		Post:        u.post,
		Comment:     u.comment,
		PostLike:    postLike,
		CommentLike: commentLike,
	})
}

func (routeTokenIssuer) Issue(userID models.UUID) (string, error) {
	return "user-" + string(userID), nil
}

func (routeTokenIssuer) Parse(rawToken string) (models.UUID, error) {
	if !strings.HasPrefix(rawToken, "user-") {
		return "", services.ErrInvalidToken
	}
	userID, err := models.ParseUUID(strings.TrimPrefix(rawToken, "user-"))
	if err != nil || userID == "" {
		return "", services.ErrInvalidToken
	}
	return userID, nil
}

func clonePost(post *models.Post) *models.Post {
	cloned := *post
	return &cloned
}

func cloneUser(user *models.User) *models.User {
	cloned := *user
	return &cloned
}

func newTestRouter(postRepo *routePostRepository) *gin.Engine {
	tokenIssuer := routeTokenIssuer{}
	authService := services.NewAuthService(
		newRouteUserRepository(),
		routePasswordHasher{},
		tokenIssuer,
	)
	postService := services.NewPostService(postRepo, routeUnitOfWork{post: postRepo})
	return SetupRouter(Handlers{
		Auth:        controllers.NewAuthController(authService),
		Post:        controllers.NewPostController(postService),
		TokenIssuer: tokenIssuer,
	})
}

type routePostAuthorResponse struct {
	ID       models.UUID `json:"id"`
	Username string      `json:"username"`
}

type routePostResponse struct {
	ID      models.UUID             `json:"id"`
	Title   string                  `json:"title"`
	Content string                  `json:"content"`
	Author  routePostAuthorResponse `json:"author"`
}

type routeUserResponse struct {
	ID       models.UUID `json:"id"`
	Username string      `json:"username"`
}

type routeAuthResponse struct {
	User routeUserResponse `json:"user"`
}

func TestSetupRouter_RegisterLoginAndMe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("COOKIE_SECURE", "true")
	router := newTestRouter(newRoutePostRepository())
	credentials := `{"username":"alice","password":"password"}`

	registerResponse := performRequest(
		router,
		http.MethodPost,
		"/api/auth/register",
		"",
		credentials,
	)
	if registerResponse.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want 201", registerResponse.Code)
	}
	var registered routeAuthResponse
	if err := json.Unmarshal(registerResponse.Body.Bytes(), &registered); err != nil {
		t.Fatalf("decode register response: %v", err)
	}
	if registered.User.ID != routeUserID || registered.User.Username != "alice" {
		t.Fatalf("registered user = %+v, want alice with ID %s", registered.User, routeUserID)
	}
	registerCookie := sessionCookie(t, registerResponse)
	if !registerCookie.HttpOnly || !registerCookie.Secure || registerCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("session cookie attributes = %+v, want HttpOnly, Secure, SameSite=Lax", registerCookie)
	}
	if registerCookie.Path != "/" || registerCookie.Name != middleware.SessionCookieName {
		t.Fatalf("session cookie = %+v, want __Host- cookie with Path=/", registerCookie)
	}
	if strings.Contains(registerResponse.Body.String(), "password") ||
		strings.Contains(registerResponse.Body.String(), "hash") ||
		strings.Contains(registerResponse.Body.String(), "token") {
		t.Fatalf("register response exposes password data: %s", registerResponse.Body.String())
	}

	loginResponse := performRequest(
		router,
		http.MethodPost,
		"/api/auth/login",
		"",
		credentials,
	)
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200", loginResponse.Code)
	}
	var loggedIn routeAuthResponse
	if err := json.Unmarshal(loginResponse.Body.Bytes(), &loggedIn); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if loggedIn.User != registered.User {
		t.Fatalf("login user = %+v, want %+v", loggedIn.User, registered.User)
	}
	loginCookie := sessionCookie(t, loginResponse)
	if loginCookie.Value == "" {
		t.Fatal("login session cookie is empty")
	}

	meResponse := performCookieRequest(router, http.MethodGet, "/api/me", loginCookie, "")
	if meResponse.Code != http.StatusOK {
		t.Fatalf("me status = %d, want 200", meResponse.Code)
	}
	var currentUser routeUserResponse
	if err := json.Unmarshal(meResponse.Body.Bytes(), &currentUser); err != nil {
		t.Fatalf("decode me response: %v", err)
	}
	if currentUser != registered.User {
		t.Fatalf("current user = %+v, want %+v", currentUser, registered.User)
	}

	logoutResponse := performCookieRequest(router, http.MethodPost, "/api/auth/logout", loginCookie, "")
	if logoutResponse.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want 204", logoutResponse.Code)
	}
	logoutCookie := sessionCookie(t, logoutResponse)
	if logoutCookie.MaxAge >= 0 || !logoutCookie.HttpOnly || !logoutCookie.Secure || logoutCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("logout cookie = %+v, want expired secure session cookie", logoutCookie)
	}
}

func TestSetupRouter_HTTPDevelopmentUsesCompatibleSessionCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("COOKIE_SECURE", "false")
	router := newTestRouter(newRoutePostRepository())

	response := performRequest(
		router,
		http.MethodPost,
		"/api/auth/register",
		"",
		`{"username":"alice","password":"password"}`,
	)
	if response.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want 201", response.Code)
	}

	cookie := sessionCookie(t, response)
	if cookie.Name == middleware.SessionCookieName {
		t.Fatalf("HTTP session cookie uses __Host- name: %q", cookie.Name)
	}
	if cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("HTTP session cookie attributes = %+v, want non-secure HttpOnly SameSite=Lax", cookie)
	}

	meResponse := performCookieRequest(router, http.MethodGet, "/api/me", cookie, "")
	if meResponse.Code != http.StatusOK {
		t.Fatalf("me status = %d, want 200 with the HTTP session cookie", meResponse.Code)
	}
}

func TestSetupRouter_ProtectedRoutesRequireSessionCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newTestRouter(newRoutePostRepository())
	postPath := "/api/posts/" + string(routePostID)
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "現在User取得を拒否する", method: http.MethodGet, path: "/api/me"},
		{name: "Post一覧取得を拒否する", method: http.MethodGet, path: "/api/posts"},
		{name: "Post詳細取得を拒否する", method: http.MethodGet, path: postPath},
		{
			name:   "Comment一覧取得を拒否する",
			method: http.MethodGet,
			path:   postPath + "/comments",
		},
		{
			name:   "Post作成を拒否する",
			method: http.MethodPost,
			path:   "/api/posts",
			body:   `{"title":"title","content":"content"}`,
		},
		{
			name:   "Comment作成を拒否する",
			method: http.MethodPost,
			path:   postPath + "/comments",
			body:   `{"content":"comment"}`,
		},
		{
			name:   "Post更新を拒否する",
			method: http.MethodPut,
			path:   postPath,
			body:   `{"title":"updated"}`,
		},
		{
			name:   "Comment更新を拒否する",
			method: http.MethodPut,
			path:   "/api/comments/" + string(routePostID),
			body:   `{"content":"updated"}`,
		},
		{name: "Post削除を拒否する", method: http.MethodDelete, path: postPath},
		{
			name:   "Comment削除を拒否する",
			method: http.MethodDelete,
			path:   "/api/comments/" + string(routePostID),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := performRequest(router, tt.method, tt.path, "", tt.body)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("%s %s status = %d, want 401", tt.method, tt.path, response.Code)
			}
		})
	}
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
		"user-"+string(routeUserID),
		`{"title":"owned","content":"content","authorId":"99999999-9999-4999-8999-999999999999"}`,
	)
	if response.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201", response.Code)
	}
	if postRepo.posts[routePostID].AuthorID != routeUserID {
		t.Fatalf("stored author ID = %s, want token user ID %s", postRepo.posts[routePostID].AuthorID, routeUserID)
	}

	response = performRequest(router, http.MethodGet, "/api/posts", "user-"+string(routeUserID), "")
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
	if ownerPosts[0].Author.ID != routeUserID || ownerPosts[0].Author.Username != "user-"+string(routeUserID) {
		t.Fatalf("owner list author = %+v, want route user", ownerPosts[0].Author)
	}

	response = performRequest(router, http.MethodGet, "/api/posts", "user-"+string(routeOtherUserID), "")
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

	response = performRequest(router, http.MethodGet, "/api/posts/"+string(routePostID), "user-"+string(routeOtherUserID), "")
	if response.Code != http.StatusOK {
		t.Fatalf("other user detail status = %d, want 200", response.Code)
	}
	var otherPost routePostResponse
	if err := json.Unmarshal(response.Body.Bytes(), &otherPost); err != nil {
		t.Fatalf("decode other user detail: %v", err)
	}
	if otherPost.Author.ID != routeUserID || otherPost.Author.Username != "user-"+string(routeUserID) {
		t.Fatalf("other user detail author = %+v, want route user", otherPost.Author)
	}

	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		response = performRequest(router, method, "/api/posts/"+string(routePostID), "user-"+string(routeOtherUserID), `{"title":"tampered"}`)
		if response.Code != http.StatusNotFound {
			t.Fatalf("other user %s status = %d, want 404", method, response.Code)
		}
	}
}

func performRequest(
	router http.Handler,
	method string,
	path string,
	cookieValue string,
	body string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if cookieValue != "" {
		request.AddCookie(middleware.NewSessionCookie(cookieValue))
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func performCookieRequest(
	router http.Handler,
	method string,
	path string,
	cookie *http.Cookie,
	body string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func sessionCookie(t *testing.T, response *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, cookie := range response.Result().Cookies() {
		if strings.HasSuffix(cookie.Name, "threadly-session") {
			return cookie
		}
	}
	t.Fatal("session cookie is missing from response")
	return nil
}
