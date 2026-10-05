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
	"Threadly/internal/usecase"
	"Threadly/internal/usecase/mocks"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	routeUserID      models.UUID = "11111111-1111-4111-8111-111111111111"
	routeOtherUserID models.UUID = "22222222-2222-4222-8222-222222222222"
	routePostID      models.UUID = "33333333-3333-4333-8333-333333333333"
)

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
		return usecase.ErrPasswordMismatch
	}
	return nil
}

func routeHashPassword(password string) string {
	passwordHash := sha256.Sum256([]byte(password))
	return string(passwordHash[:])
}

type routeTokenIssuer struct{}

func (routeTokenIssuer) Issue(userID models.UUID) (string, error) {
	return "user-" + string(userID), nil
}

func (routeTokenIssuer) Parse(rawToken string) (models.UUID, error) {
	if !strings.HasPrefix(rawToken, "user-") {
		return "", usecase.ErrInvalidToken
	}
	userID, err := models.ParseUUID(strings.TrimPrefix(rawToken, "user-"))
	if err != nil || userID == "" {
		return "", usecase.ErrInvalidToken
	}
	return userID, nil
}

func cloneUser(user *models.User) *models.User {
	cloned := *user
	return &cloned
}

type routeRepositories struct {
	Post        *mocks.MockPostRepository
	Comment     *mocks.MockCommentRepository
	PostLike    *mocks.MockPostLikeRepository
	CommentLike *mocks.MockCommentLikeRepository
}

type routeUnitOfWork struct {
	repos repositories.TransactionRepositories
}

func (u routeUnitOfWork) WithinTransaction(
	_ context.Context,
	fn func(repositories.TransactionRepositories) error,
) error {
	return fn(u.repos)
}

func newTestRouter(t *testing.T) (*gin.Engine, routeRepositories) {
	t.Helper()
	return newRouteRouter(t, false)
}

func newRouteRouter(t *testing.T, withLikes bool) (*gin.Engine, routeRepositories) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repos := routeRepositories{
		Post:        mocks.NewMockPostRepository(ctrl),
		Comment:     mocks.NewMockCommentRepository(ctrl),
		PostLike:    mocks.NewMockPostLikeRepository(ctrl),
		CommentLike: mocks.NewMockCommentLikeRepository(ctrl),
	}
	uow := routeUnitOfWork{repos: repositories.TransactionRepositories{
		Post:        repos.Post,
		Comment:     repos.Comment,
		PostLike:    repos.PostLike,
		CommentLike: repos.CommentLike,
	}}
	tokens := routeTokenIssuer{}
	auth := usecase.NewAuthUsecase(newRouteUserRepository(), routePasswordHasher{}, tokens)
	post := usecase.NewPostUsecase(repos.Post, uow)
	comment := usecase.NewCommentUsecase(repos.Comment, repos.Post, uow)
	var likeController *controllers.LikeController
	if withLikes {
		likes := usecase.NewLikeUsecase(repos.Post, repos.Comment, repos.PostLike, repos.CommentLike)
		post = usecase.NewPostUsecaseWithLikeReader(repos.Post, uow, likes)
		comment = usecase.NewCommentUsecaseWithLikeReader(repos.Comment, repos.Post, uow, likes)
		likeController = controllers.NewLikeController(likes)
	}
	return SetupRouter(Handlers{
		Auth:        controllers.NewAuthController(auth),
		Post:        controllers.NewPostController(post),
		Comment:     controllers.NewCommentController(comment),
		Like:        likeController,
		TokenIssuer: tokens,
	}), repos
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
	router, _ := newTestRouter(t)
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
	router, _ := newTestRouter(t)

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
	router, _ := newRouteRouter(t, true)
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
		{name: "Post Likeを拒否する", method: http.MethodPut, path: "/api/posts/not-a-uuid/like"},
		{name: "Post Unlikeを拒否する", method: http.MethodDelete, path: postPath + "/like"},
		{name: "Comment Likeを拒否する", method: http.MethodPut, path: "/api/comments/" + string(commentRouteRootID) + "/like"},
		{name: "Comment Unlikeを拒否する", method: http.MethodDelete, path: "/api/comments/" + string(commentRouteRootID) + "/like"},
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
	router, repos := newTestRouter(t)
	post := &models.Post{
		UUIDBaseModel: models.UUIDBaseModel{ID: routePostID},
		AuthorID:      routeUserID,
		Author:        models.User{UUIDBaseModel: models.UUIDBaseModel{ID: routeUserID}, Username: "alice"},
		Title:         "owned",
		Content:       "content",
	}
	repos.Post.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, created *models.Post) error {
			require.Equal(t, routeUserID, created.AuthorID)
			require.Equal(t, post.Title, created.Title)
			require.Equal(t, post.Content, created.Content)
			return nil
		},
	)
	response := performRequest(
		router,
		http.MethodPost,
		"/api/posts",
		"user-"+string(routeUserID),
		`{"title":"owned","content":"content","authorId":"99999999-9999-4999-8999-999999999999"}`,
	)
	require.Equal(t, http.StatusCreated, response.Code)

	repos.Post.EXPECT().ListAll(gomock.Any()).Return([]*models.Post{post}, nil)
	response = performRequest(router, http.MethodGet, "/api/posts", "user-"+string(routeOtherUserID), "")
	require.Equal(t, http.StatusOK, response.Code)
	var posts []routePostResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &posts))
	require.Len(t, posts, 1)
	require.Equal(t, routeUserID, posts[0].Author.ID)
	require.Equal(t, "alice", posts[0].Author.Username)

	repos.Post.EXPECT().GetByID(gomock.Any(), routePostID).Return(post, nil)
	response = performRequest(
		router,
		http.MethodGet,
		"/api/posts/"+string(routePostID),
		"user-"+string(routeOtherUserID),
		"",
	)
	require.Equal(t, http.StatusOK, response.Code)
	var detail routePostResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &detail))
	require.Equal(t, posts[0].Author, detail.Author)
	require.Equal(t, post.Title, detail.Title)
	require.Equal(t, post.Content, detail.Content)

	repos.Post.EXPECT().GetByIDForOwner(gomock.Any(), routeOtherUserID, routePostID).
		Return(nil, repositories.ErrPostNotFound)
	repos.Post.EXPECT().GetByIDForUpdate(gomock.Any(), routePostID).Return(post, nil)
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		response = performRequest(
			router,
			method,
			"/api/posts/"+string(routePostID),
			"user-"+string(routeOtherUserID),
			`{"title":"tampered"}`,
		)
		require.Equal(t, http.StatusNotFound, response.Code)
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
