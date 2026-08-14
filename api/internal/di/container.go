package di

import (
	"Threadly/internal/domain/repositories"
	authinfra "Threadly/internal/infra/auth"
	"Threadly/internal/infra/database"
	dbrepository "Threadly/internal/infra/database/repositories"
	"Threadly/internal/infra/http/routes"
	"Threadly/internal/interface/controllers"
	"Threadly/internal/usecase/services"
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
	"go.uber.org/dig"
	"gorm.io/gorm"
)

func NewContainer() (*dig.Container, error) {
	container := dig.New()

	constructors := []any{
		database.ConnectionDB,
		provideUserRepository,
		providePostRepository,
		provideCommentRepository,
		providePasswordHasher,
		provideTokenIssuer,
		services.NewAuthService,
		services.NewPostService,
		services.NewCommentService,
		controllers.NewAuthController,
		controllers.NewPostController,
		controllers.NewCommentController,
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

func providePostRepository(db *gorm.DB) repositories.PostRepository {
	return dbrepository.NewPostRepository(db)
}

func provideUserRepository(db *gorm.DB) repositories.UserRepository {
	return dbrepository.NewUserRepository(db)
}

func provideCommentRepository(db *gorm.DB) repositories.CommentRepository {
	return dbrepository.NewCommentRepository(db)
}

func providePasswordHasher() services.PasswordHasher {
	return authinfra.NewArgon2idHasher()
}

func provideTokenIssuer() (services.TokenIssuer, error) {
	// JWT_SECRETが未設定・短すぎる場合は、デフォルト値にフォールバックせず起動を失敗させる。
	return authinfra.NewJWTIssuer(os.Getenv("JWT_SECRET"))
}

func provideHandlers(
	authController *controllers.AuthController,
	postController *controllers.PostController,
	commentController *controllers.CommentController,
	tokenIssuer services.TokenIssuer,
) routes.Handlers {
	return routes.Handlers{
		Auth:        authController,
		Post:        postController,
		Comment:     commentController,
		TokenIssuer: tokenIssuer,
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
