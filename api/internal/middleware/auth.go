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
		rawToken, ok := sessionToken(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		userID, err := tokenIssuer.Parse(rawToken)
		if err != nil || userID == 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		// 後続レイヤーにはUser IDだけを渡し、認証情報そのものを持ち回らない。
		requestContext := context.WithValue(c.Request.Context(), userIDContextKey, userID)
		c.Request = c.Request.WithContext(requestContext)
		c.Next()
	}
}

func sessionToken(c *gin.Context) (string, bool) {
	if cookie, err := c.Request.Cookie(sessionCookieName()); err == nil {
		return strings.TrimSpace(cookie.Value), true
	}

	return "", false
}

func UserIDFromContext(ctx context.Context) (uint, bool) {
	userID, ok := ctx.Value(userIDContextKey).(uint)
	return userID, ok && userID > 0
}
