package middleware

import (
	"net/http"
	"os"
	"strconv"
	"time"
)

// __Host- prefix requires Secure, Path=/, and no Domain attribute.
const SessionCookieName = "__Host-threadly-session"

const sessionCookieMaxAge = int(time.Hour / time.Second)

func NewSessionCookie(token string) *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   sessionCookieMaxAge,
		HttpOnly: true,
		Secure:   sessionCookieSecure(),
		SameSite: http.SameSiteLaxMode,
	}
}

func NewExpiredSessionCookie() *http.Cookie {
	cookie := NewSessionCookie("")
	cookie.MaxAge = -1
	cookie.Expires = time.Unix(1, 0)
	return cookie
}

func sessionCookieSecure() bool {
	value, ok := os.LookupEnv("COOKIE_SECURE")
	if !ok {
		return true
	}

	secure, err := strconv.ParseBool(value)
	if err != nil {
		return true
	}
	return secure
}
