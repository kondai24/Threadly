package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"
	"Threadly/internal/interface/controllers"
	"Threadly/internal/usecase/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	commentRouteRootID     models.UUID = "77777777-7777-4777-8777-777777777777"
	commentRouteReplyID    models.UUID = "88888888-8888-4888-8888-888888888888"
	commentRouteOtherID    models.UUID = "99999999-9999-4999-8999-999999999999"
	commentRouteSecondPost models.UUID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
)

type commentRouteStore struct {
	posts    map[models.UUID]*models.Post
	comments map[models.UUID]*models.Comment
	nextID   int
}

type commentRoutePostRepository struct {
	store *commentRouteStore
}

type commentRouteCommentRepository struct {
	store *commentRouteStore
}

func newCommentRouteStore() *commentRouteStore {
	return &commentRouteStore{
		posts:    make(map[models.UUID]*models.Post),
		comments: make(map[models.UUID]*models.Comment),
	}
}

func (r *commentRoutePostRepository) GetByID(
	_ context.Context,
	postID models.UUID,
) (*models.Post, error) {
	post, ok := r.store.posts[postID]
	if !ok || post.DeletedAt.Valid {
		return nil, gorm.ErrRecordNotFound
	}
	return cloneCommentRoutePost(post), nil
}

func (r *commentRoutePostRepository) GetByIDForOwner(
	_ context.Context,
	userID models.UUID,
	postID models.UUID,
) (*models.Post, error) {
	post, err := r.GetByID(context.Background(), postID)
	if err != nil || post.AuthorID != userID {
		return nil, gorm.ErrRecordNotFound
	}
	return post, nil
}

func (r *commentRoutePostRepository) Create(
	_ context.Context,
	post *models.Post,
) error {
	post.ID = commentRouteSecondPost
	post.Author = models.User{
		UUIDBaseModel: models.UUIDBaseModel{ID: post.AuthorID},
		Username:      "user-" + string(post.AuthorID),
	}
	r.store.posts[post.ID] = cloneCommentRoutePost(post)
	return nil
}

func (r *commentRoutePostRepository) Update(
	_ context.Context,
	userID models.UUID,
	post *models.Post,
) error {
	stored, ok := r.store.posts[post.ID]
	if !ok || stored.DeletedAt.Valid || stored.AuthorID != userID {
		return gorm.ErrRecordNotFound
	}
	stored.Title = post.Title
	stored.Content = post.Content
	return nil
}

func (r *commentRoutePostRepository) DeleteByID(
	_ context.Context,
	userID models.UUID,
	postID models.UUID,
) (int64, error) {
	post, ok := r.store.posts[postID]
	if !ok || post.DeletedAt.Valid || post.AuthorID != userID {
		return 0, nil
	}

	now := time.Now()
	post.DeletedAt = gorm.DeletedAt{Time: now, Valid: true}
	for _, comment := range r.store.comments {
		if comment.PostID == postID && !comment.DeletedAt.Valid {
			comment.DeletedAt = gorm.DeletedAt{Time: now, Valid: true}
		}
	}
	return 1, nil
}

func (r *commentRoutePostRepository) ListAll(
	_ context.Context,
) ([]*models.Post, error) {
	posts := make([]*models.Post, 0, len(r.store.posts))
	for _, post := range r.store.posts {
		if !post.DeletedAt.Valid {
			posts = append(posts, cloneCommentRoutePost(post))
		}
	}
	return posts, nil
}

func (r *commentRouteCommentRepository) Create(
	_ context.Context,
	comment *models.Comment,
) error {
	r.store.nextID++
	commentID := commentRouteOtherID
	if comment.ParentID == nil {
		commentID = commentRouteRootID
		if r.store.nextID > 1 {
			commentID = commentRouteOtherID
		}
	} else {
		commentID = commentRouteReplyID
	}
	now := time.Unix(int64(r.store.nextID), 0)
	comment.ID = commentID
	comment.CreatedAt = now
	comment.UpdatedAt = now
	comment.Author = models.User{
		UUIDBaseModel: models.UUIDBaseModel{ID: comment.AuthorID},
		Username:      "user-" + string(comment.AuthorID),
	}
	r.store.comments[comment.ID] = cloneCommentRouteComment(comment)
	return nil
}

func (r *commentRouteCommentRepository) ListByPostID(
	_ context.Context,
	postID models.UUID,
) ([]*models.Comment, error) {
	roots := make([]*models.Comment, 0)
	for _, comment := range r.store.comments {
		if comment.PostID != postID || comment.DeletedAt.Valid || comment.ParentID != nil {
			continue
		}
		root := cloneCommentRouteComment(comment)
		root.Replies = make([]*models.Comment, 0)
		for _, reply := range r.store.comments {
			if reply.PostID == postID && !reply.DeletedAt.Valid &&
				reply.ParentID != nil && *reply.ParentID == comment.ID {
				root.Replies = append(root.Replies, cloneCommentRouteComment(reply))
			}
		}
		sortCommentRouteComments(root.Replies)
		roots = append(roots, root)
	}
	sortCommentRouteComments(roots)
	return roots, nil
}

func (r *commentRouteCommentRepository) GetByID(
	_ context.Context,
	commentID models.UUID,
) (*models.Comment, error) {
	comment, ok := r.store.comments[commentID]
	if !ok || comment.DeletedAt.Valid {
		return nil, repositories.ErrCommentNotFound
	}
	return cloneCommentRouteComment(comment), nil
}

func (r *commentRouteCommentRepository) Update(
	_ context.Context,
	userID models.UUID,
	commentID models.UUID,
	content string,
) (int64, error) {
	comment, ok := r.store.comments[commentID]
	if !ok || comment.DeletedAt.Valid || comment.AuthorID != userID {
		return 0, nil
	}
	comment.Content = content
	comment.UpdatedAt = time.Now()
	return 1, nil
}

func (r *commentRouteCommentRepository) DeleteByID(
	_ context.Context,
	userID models.UUID,
	commentID models.UUID,
) (int64, error) {
	comment, ok := r.store.comments[commentID]
	if !ok || comment.DeletedAt.Valid || comment.AuthorID != userID {
		return 0, nil
	}

	now := time.Now()
	comment.DeletedAt = gorm.DeletedAt{Time: now, Valid: true}
	rows := int64(1)
	if comment.ParentID == nil {
		for _, reply := range r.store.comments {
			if reply.ParentID != nil && *reply.ParentID == commentID && !reply.DeletedAt.Valid {
				reply.DeletedAt = gorm.DeletedAt{Time: now, Valid: true}
				rows++
			}
		}
	}
	return rows, nil
}

func newCommentRouteRouter(store *commentRouteStore) *gin.Engine {
	tokenIssuer := routeTokenIssuer{}
	authService := services.NewAuthService(
		newRouteUserRepository(),
		routePasswordHasher{},
		tokenIssuer,
	)
	postRepo := &commentRoutePostRepository{store: store}
	commentRepo := &commentRouteCommentRepository{store: store}
	return SetupRouter(Handlers{
		Auth: controllers.NewAuthController(authService),
		Post: controllers.NewPostController(services.NewPostService(postRepo)),
		Comment: controllers.NewCommentController(
			services.NewCommentService(commentRepo, postRepo),
		),
		TokenIssuer: tokenIssuer,
	})
}

func seedCommentRoutePost(store *commentRouteStore) {
	store.posts[routePostID] = &models.Post{
		UUIDBaseModel: models.UUIDBaseModel{ID: routePostID},
		AuthorID:      routeUserID,
		Author: models.User{
			UUIDBaseModel: models.UUIDBaseModel{ID: routeUserID},
			Username:      "user-" + string(routeUserID),
		},
		Title:   "post",
		Content: "content",
	}
}

func TestSetupRouter_CommentsRespectThreadAndOwnershipBoundaries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("COOKIE_SECURE", "false")
	store := newCommentRouteStore()
	seedCommentRoutePost(store)
	router := newCommentRouteRouter(store)
	postPath := "/api/posts/" + string(routePostID)
	rootPath := postPath + "/comments"

	response := performRequest(
		router,
		http.MethodPost,
		rootPath,
		"user-"+string(routeUserID),
		`{"content":"  root comment  ","authorId":"99999999-9999-4999-8999-999999999999"}`,
	)
	if response.Code != http.StatusCreated {
		t.Fatalf("root comment status = %d, want 201", response.Code)
	}

	response = performRequest(
		router,
		http.MethodPost,
		rootPath,
		"user-"+string(routeUserID),
		`{"content":"invalid parent","parentId":"not-a-uuid"}`,
	)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid parent status = %d, want 400", response.Code)
	}

	response = performRequest(
		router,
		http.MethodPost,
		rootPath,
		"user-"+string(routeOtherUserID),
		`{"content":"other root"}`,
	)
	if response.Code != http.StatusCreated {
		t.Fatalf("other root comment status = %d, want 201", response.Code)
	}

	response = performRequest(
		router,
		http.MethodPost,
		rootPath,
		"user-"+string(routeOtherUserID),
		`{"content":"reply","parentId":"77777777-7777-4777-8777-777777777777"}`,
	)
	if response.Code != http.StatusCreated {
		t.Fatalf("reply status = %d, want 201", response.Code)
	}

	response = performRequest(
		router,
		http.MethodPost,
		rootPath,
		"user-"+string(routeOtherUserID),
		`{"content":"second reply","parentId":"88888888-8888-4888-8888-888888888888"}`,
	)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("second reply status = %d, want 400", response.Code)
	}

	response = performRequest(
		router,
		http.MethodGet,
		rootPath,
		"user-"+string(routeOtherUserID),
		"",
	)
	if response.Code != http.StatusOK {
		t.Fatalf("list comments status = %d, want 200", response.Code)
	}
	var comments []commentRouteResponse
	if err := decodeJSON(response, &comments); err != nil {
		t.Fatalf("decode comments response: %v", err)
	}
	if len(comments) != 2 || comments[0].ID != commentRouteOtherID {
		t.Fatalf("root comments = %+v, want newer other root first", comments)
	}
	if len(comments[1].Replies) != 1 || comments[1].Replies[0].ID != commentRouteReplyID {
		t.Fatalf("root replies = %+v, want one nested reply", comments[1].Replies)
	}
	if comments[1].Replies[0].Author.Username != "user-"+string(routeOtherUserID) {
		t.Fatalf("reply author = %+v, want authenticated user", comments[1].Replies[0].Author)
	}

	response = performRequest(
		router,
		http.MethodPut,
		"/api/comments/"+string(commentRouteRootID),
		"user-"+string(routeOtherUserID),
		`{"content":"tampered"}`,
	)
	if response.Code != http.StatusNotFound {
		t.Fatalf("other user update status = %d, want 404", response.Code)
	}

	response = performRequest(
		router,
		http.MethodPut,
		"/api/comments/"+string(commentRouteRootID),
		"user-"+string(routeUserID),
		`{"content":"  updated root  "}`,
	)
	if response.Code != http.StatusOK {
		t.Fatalf("owner update status = %d, want 200", response.Code)
	}

	response = performRequest(
		router,
		http.MethodDelete,
		"/api/comments/"+string(commentRouteRootID),
		"user-"+string(routeUserID),
		"",
	)
	if response.Code != http.StatusNoContent {
		t.Fatalf("owner delete status = %d, want 204", response.Code)
	}

	response = performRequest(
		router,
		http.MethodPost,
		rootPath,
		"user-"+string(routeOtherUserID),
		`{"content":"reply to deleted","parentId":"77777777-7777-4777-8777-777777777777"}`,
	)
	if response.Code != http.StatusNotFound {
		t.Fatalf("deleted parent reply status = %d, want 404", response.Code)
	}

	response = performRequest(
		router,
		http.MethodDelete,
		"/api/posts/"+string(routePostID),
		"user-"+string(routeUserID),
		"",
	)
	if response.Code != http.StatusNoContent {
		t.Fatalf("post delete status = %d, want 204", response.Code)
	}

	response = performRequest(router, http.MethodGet, rootPath, "user-"+string(routeUserID), "")
	if response.Code != http.StatusNotFound {
		t.Fatalf("deleted post comment list status = %d, want 404", response.Code)
	}
}

func decodeJSON(response *httptest.ResponseRecorder, value any) error {
	return json.NewDecoder(response.Body).Decode(value)
}

type commentRouteAuthorResponse struct {
	ID       models.UUID `json:"id"`
	Username string      `json:"username"`
}

type commentRouteResponse struct {
	ID      models.UUID                `json:"id"`
	Content string                     `json:"content"`
	Author  commentRouteAuthorResponse `json:"author"`
	Replies []commentRouteResponse     `json:"replies"`
}

func cloneCommentRoutePost(post *models.Post) *models.Post {
	cloned := *post
	return &cloned
}

func cloneCommentRouteComment(comment *models.Comment) *models.Comment {
	cloned := *comment
	if comment.ParentID != nil {
		parentID := *comment.ParentID
		cloned.ParentID = &parentID
	}
	cloned.Replies = nil
	return &cloned
}

func sortCommentRouteComments(comments []*models.Comment) {
	sort.Slice(comments, func(i, j int) bool {
		if comments[i].CreatedAt.Equal(comments[j].CreatedAt) {
			return comments[i].ID > comments[j].ID
		}
		return comments[i].CreatedAt.After(comments[j].CreatedAt)
	})
}

var _ repositories.CommentRepository = (*commentRouteCommentRepository)(nil)
var _ repositories.PostRepository = (*commentRoutePostRepository)(nil)
