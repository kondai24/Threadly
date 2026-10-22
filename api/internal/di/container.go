package di

import (
	"Threadly/internal/domain/repositories"
	authinfra "Threadly/internal/infra/auth"
	"Threadly/internal/infra/database"
	dbrepository "Threadly/internal/infra/database/repositories"
	"Threadly/internal/infra/http/routes"
	"Threadly/internal/interface/controllers"
	"Threadly/internal/usecase"
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/dig"
	"gorm.io/gorm"
)

func NewContainer() (*dig.Container, error) {
	container := dig.New()

	constructors := []any{
		provideTracer,
		database.ConnectionDB,
		provideUserRepository,
		providePostRepository,
		provideCommentRepository,
		provideUnitOfWork,
		providePostLikeRepository,
		provideCommentLikeRepository,
		providePostLikeSummaryReader,
		provideCommentLikeSummaryReader,
		providePasswordHasher,
		provideTokenIssuer,
		usecase.NewAuthUsecase,
		usecase.NewLikeUsecase,
		usecase.NewPostUsecaseWithLikeReader,
		usecase.NewCommentUsecaseWithLikeReader,
		controllers.NewAuthController,
		controllers.NewPostController,
		controllers.NewCommentController,
		controllers.NewLikeController,
		provideHandlers,
		routes.SetupRouter,
	}

	for _, constructor := range constructors {
		if err := container.Provide(constructor); err != nil {
			return nil, fmt.Errorf("failed to register constructor %T: %w", constructor, err)
		}
	}

	return container, nil
}

// provideTracerは、実行時SDKを起動処理へ接続するまで、無送信のTracerを明示的に渡す。
func provideTracer() trace.Tracer {
	return noop.NewTracerProvider().Tracer("Threadly/api")
}

func providePostRepository(db *gorm.DB, tracer trace.Tracer) repositories.PostRepository {
	return dbrepository.NewPostRepository(db, tracer)
}

func provideUserRepository(db *gorm.DB, tracer trace.Tracer) repositories.UserRepository {
	return dbrepository.NewUserRepository(db, tracer)
}

func provideCommentRepository(db *gorm.DB, tracer trace.Tracer) repositories.CommentRepository {
	return dbrepository.NewCommentRepository(db, tracer)
}

func provideUnitOfWork(db *gorm.DB, tracer trace.Tracer) repositories.UnitOfWork {
	return dbrepository.NewUnitOfWork(db, tracer)
}

func providePostLikeRepository(db *gorm.DB, tracer trace.Tracer) repositories.PostLikeRepository {
	return dbrepository.NewPostLikeRepository(db, tracer)
}

func provideCommentLikeRepository(db *gorm.DB, tracer trace.Tracer) repositories.CommentLikeRepository {
	return dbrepository.NewCommentLikeRepository(db, tracer)
}

func providePostLikeSummaryReader(likeUsecase *usecase.LikeUsecase) usecase.PostLikeSummaryReader {
	return likeUsecase
}

func provideCommentLikeSummaryReader(likeUsecase *usecase.LikeUsecase) usecase.CommentLikeSummaryReader {
	return likeUsecase
}

func providePasswordHasher() usecase.PasswordHasher {
	return authinfra.NewArgon2idHasher()
}

func provideTokenIssuer() (usecase.TokenIssuer, error) {
	// JWT_SECRETが未設定・短すぎる場合は、デフォルト値にフォールバックせず
	// 起動を失敗させる。
	return authinfra.NewJWTIssuer(os.Getenv("JWT_SECRET"))
}

func provideHandlers(
	authController *controllers.AuthController,
	postController *controllers.PostController,
	commentController *controllers.CommentController,
	likeController *controllers.LikeController,
	tokenIssuer usecase.TokenIssuer,
	tracer trace.Tracer,
) routes.Handlers {
	return routes.Handlers{
		Auth:        authController,
		Post:        postController,
		Comment:     commentController,
		Like:        likeController,
		TokenIssuer: tokenIssuer,
		Tracer:      tracer,
	}
}

func ResolveRouter(container *dig.Container) (*gin.Engine, error) {
	var router *gin.Engine
	if err := container.Invoke(func(r *gin.Engine) {
		router = r
	}); err != nil {
		return nil, fmt.Errorf("failed to resolve router: %w", err)
	}

	return router, nil
}
