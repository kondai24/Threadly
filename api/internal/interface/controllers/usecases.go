package controllers

import (
	"context"

	"Threadly/internal/domain/models"
	"Threadly/internal/usecase/services"
)

// AuthUsecaseは、認証Controllerが必要とする業務操作だけを定義する。
// HTTP層は具象AuthServiceではなく、この契約を通じてUsecaseを呼び出す。
type AuthUsecase interface {
	Register(ctx context.Context, username string, password string) (*models.User, string, error)
	Login(ctx context.Context, username string, password string) (*models.User, string, error)
	GetMe(ctx context.Context, userID models.UUID) (*models.User, error)
}

// PostUsecaseは、PostControllerが必要とする業務操作だけを定義する。
type PostUsecase interface {
	ListAllPostsForUser(ctx context.Context, userID models.UUID) ([]services.PostRead, error)
	GetPostByIDForUser(
		ctx context.Context,
		userID models.UUID,
		postID models.UUID,
	) (*services.PostRead, error)
	CreatePost(ctx context.Context, userID models.UUID, title string, content string) error
	GetPostByIDForOwner(
		ctx context.Context,
		userID models.UUID,
		postID models.UUID,
	) (*models.Post, error)
	UpdatePost(ctx context.Context, userID models.UUID, post *models.Post) error
	DeletePost(ctx context.Context, userID models.UUID, postID models.UUID) error
}

// CommentUsecaseは、CommentControllerが必要とする業務操作だけを定義する。
type CommentUsecase interface {
	ListCommentsForUser(
		ctx context.Context,
		userID models.UUID,
		postID models.UUID,
	) (services.CommentListRead, error)
	CreateComment(
		ctx context.Context,
		userID models.UUID,
		postID models.UUID,
		content string,
		parentID *models.UUID,
	) error
	UpdateComment(
		ctx context.Context,
		userID models.UUID,
		commentID models.UUID,
		content string,
	) error
	DeleteComment(ctx context.Context, userID models.UUID, commentID models.UUID) error
}

// LikeUsecaseは、LikeControllerが必要とする冪等なLike操作を定義する。
type LikeUsecase interface {
	LikePost(
		ctx context.Context,
		userID models.UUID,
		postID models.UUID,
	) (services.LikeActionResult, error)
	UnlikePost(
		ctx context.Context,
		userID models.UUID,
		postID models.UUID,
	) (services.LikeActionResult, error)
	LikeComment(
		ctx context.Context,
		userID models.UUID,
		commentID models.UUID,
	) (services.LikeActionResult, error)
	UnlikeComment(
		ctx context.Context,
		userID models.UUID,
		commentID models.UUID,
	) (services.LikeActionResult, error)
}
