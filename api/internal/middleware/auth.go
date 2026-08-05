package middleware

import (
	"context"
	"net/http"
	"strings"

	"Threadly/internal/usecase/services"

	"github.com/gin-gonic/gin"
)

// 独自型を使い、他パッケージのcontext keyとの衝突を防ぐ。
type contextKey uint8

const userIDContextKey contextKey = iota

func RequireAuth(tokenIssuer services.TokenIssuer) gin.HandlerFunc {
	return func(c *gin.Context) {
		scheme, rawToken, ok := strings.Cut(c.GetHeader("Authorization"), " ")
		if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(rawToken) == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		userID, err := tokenIssuer.Parse(strings.TrimSpace(rawToken))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		// 後続レイヤーにはUser IDだけを渡し、認証情報そのものを持ち回らない。
		requestContext := context.WithValue(c.Request.Context(), userIDContextKey, userID)
		c.Request = c.Request.WithContext(requestContext)
		c.Next()
	}
}

func UserIDFromContext(ctx context.Context) (uint, bool) {
	userID, ok := ctx.Value(userIDContextKey).(uint)
	return userID, ok && userID > 0
}
