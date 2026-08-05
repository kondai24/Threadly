package auth

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

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

func (i *JWTIssuer) Issue(userID uint) (string, error) {
	if userID == 0 {
		return "", errors.New("user id must be positive")
	}

	now := i.now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   strconv.FormatUint(uint64(userID), 10),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(i.lifetime)),
	})
	signedToken, err := token.SignedString(i.secret)
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}
	return signedToken, nil
}

func (i *JWTIssuer) Parse(rawToken string) (uint, error) {
	if strings.TrimSpace(rawToken) == "" {
		return 0, services.ErrInvalidToken
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
	)
	if err != nil || !token.Valid {
		return 0, services.ErrInvalidToken
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || claims.Subject == "" {
		return 0, services.ErrInvalidToken
	}
	userID, err := strconv.ParseUint(claims.Subject, 10, 64)
	if err != nil || userID == 0 || userID > uint64(^uint(0)) {
		return 0, services.ErrInvalidToken
	}
	return uint(userID), nil
}
