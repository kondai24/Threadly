package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"Threadly/internal/domain/models"
	"Threadly/internal/usecase/services"

	"github.com/golang-jwt/jwt/v5"
)

const (
	jwtLifetime      = time.Hour
	minimumSecretLen = 32
)

type JWTIssuer struct {
	secret   []byte
	now      func() time.Time
	lifetime time.Duration
}

func NewJWTIssuer(secret string) (*JWTIssuer, error) {
	if len([]byte(secret)) < minimumSecretLen {
		return nil, errors.New("jwt secret must be at least 32 bytes")
	}
	return &JWTIssuer{
		secret:   []byte(secret),
		now:      time.Now,
		lifetime: jwtLifetime,
	}, nil
}

func (i *JWTIssuer) Issue(userID models.UUID) (string, error) {
	if userID == "" {
		return "", errors.New("user id must not be empty")
	}

	now := i.now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   string(userID),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(i.lifetime)),
	})
	signedToken, err := token.SignedString(i.secret)
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}
	return signedToken, nil
}

func (i *JWTIssuer) Parse(rawToken string) (models.UUID, error) {
	if strings.TrimSpace(rawToken) == "" {
		return "", services.ErrInvalidToken
	}

	// 署名方式をHS256に限定し、tokenヘッダーのalgを無条件に信用しない。
	token, err := jwt.ParseWithClaims(
		rawToken,
		&jwt.RegisteredClaims{},
		func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, services.ErrInvalidToken
			}
			return i.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithTimeFunc(i.now),
	)
	if err != nil || !token.Valid {
		return "", services.ErrInvalidToken
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || claims.Subject == "" {
		return "", services.ErrInvalidToken
	}
	userID, err := models.ParseUUID(claims.Subject)
	if err != nil || userID == "" {
		return "", services.ErrInvalidToken
	}
	return userID, nil
}
