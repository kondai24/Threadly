package services

import (
	"context"
	"errors"
	"fmt"

	"Threadly/internal/domain/models"
	"Threadly/internal/domain/repositories"
)

var (
	ErrInvalidCredentials    = errors.New("invalid credentials")
	ErrUsernameAlreadyExists = errors.New("username already exists")
	ErrInvalidToken          = errors.New("invalid token")
	// ErrPasswordMismatchは、PasswordHasher.Compareがpassword不一致時に返すエラーである。
	ErrPasswordMismatch = errors.New("password mismatch")
)

type PasswordHasher interface {
	Hash(password string) (string, error)
	// Compareはpassword不一致時だけErrPasswordMismatchを返し、不正なhashや
	// 内部障害は別エラーで返す。
	Compare(encodedHash string, password string) error
}

// TokenIssuerは、Usecaseと認証adapterの間でToken発行・検証の契約を定義する。
type TokenIssuer interface {
	Issue(userID models.UUID) (string, error)
	Parse(rawToken string) (models.UUID, error)
}

// AuthServiceは、認証情報の検証・Userの永続化・セッション発行を組み立てる
// Usecaseである。
// HTTPのstatusやCookie操作はControllerへ、DB操作はUserRepositoryへ委譲する。
type AuthService struct {
	repo   repositories.UserRepository
	hasher PasswordHasher
	tokens TokenIssuer
}

// NewAuthServiceは、認証Usecaseを依存性注入で構築する。
func NewAuthService(
	repo repositories.UserRepository,
	hasher PasswordHasher,
	tokens TokenIssuer,
) *AuthService {
	return &AuthService{
		repo:   repo,
		hasher: hasher,
		tokens: tokens,
	}
}

func (s *AuthService) Register(
	ctx context.Context,
	username string,
	password string,
) (*models.User, string, error) {
	if err := models.ValidateUsername(username); err != nil {
		return nil, "", err
	}
	if err := models.ValidatePassword(password); err != nil {
		return nil, "", err
	}

	passwordHash, err := s.hasher.Hash(password)
	if err != nil {
		return nil, "", fmt.Errorf("hash password: %w", err)
	}

	user := &models.User{
		Username:     username,
		PasswordHash: passwordHash,
	}
	if err := s.repo.Create(ctx, user); err != nil {
		if errors.Is(err, repositories.ErrUsernameAlreadyExists) {
			return nil, "", ErrUsernameAlreadyExists
		}
		return nil, "", fmt.Errorf("create user: %w", err)
	}

	token, err := s.tokens.Issue(user.ID)
	if err != nil {
		return nil, "", fmt.Errorf("issue access token: %w", err)
	}
	return user, token, nil
}

func (s *AuthService) Login(
	ctx context.Context,
	username string,
	password string,
) (*models.User, string, error) {
	if err := models.ValidateUsername(username); err != nil {
		return nil, "", err
	}
	if err := models.ValidatePassword(password); err != nil {
		return nil, "", err
	}

	user, err := s.repo.FindByUsername(ctx, username)
	if errors.Is(err, repositories.ErrUserNotFound) {
		// username不存在とpassword不一致を同じエラーにして、Userの存在を推測されにくくする。
		return nil, "", ErrInvalidCredentials
	}
	if err != nil {
		return nil, "", fmt.Errorf("find user for login: %w", err)
	}
	if err := s.hasher.Compare(user.PasswordHash, password); err != nil {
		if errors.Is(err, ErrPasswordMismatch) {
			// password不一致はusername不存在時と同じ応答にして、Userの存在を推測されにくくする。
			return nil, "", ErrInvalidCredentials
		}
		return nil, "", fmt.Errorf("compare password hash: %w", err)
	}

	token, err := s.tokens.Issue(user.ID)
	if err != nil {
		return nil, "", fmt.Errorf("issue access token: %w", err)
	}
	return user, token, nil
}

func (s *AuthService) GetMe(ctx context.Context, userID models.UUID) (*models.User, error) {
	user, err := s.repo.FindByID(ctx, userID)
	if errors.Is(err, repositories.ErrUserNotFound) {
		// RepositoryのNotFoundをUsecaseの契約へ変換し、Controllerへ永続化層を漏らさない。
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find current user: %w", err)
	}
	return user, nil
}
