package di

import (
	"Threadly/internal/domain/repositories"
	"Threadly/internal/infra/database"
	dbrepository "Threadly/internal/infra/database/repositories"
	"Threadly/internal/infra/http/routes"
	"Threadly/internal/interface/controllers"
	"Threadly/internal/usecase/services"
	"fmt"

	"github.com/gin-gonic/gin"
	"go.uber.org/dig"
	"gorm.io/gorm"
)

func NewContainer() (*dig.Container, error) {
	container := dig.New()

	constructors := []any{
		database.ConnectionDB,
		providePostRepository,
		services.NewPostService,
		controllers.NewPostController,
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

func provideHandlers(postController *controllers.PostController) routes.Handlers {
	return routes.Handlers{
		Post: postController,
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
