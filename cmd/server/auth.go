package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type ctxKey string

const (
	ctxUserID    ctxKey = "user_id"
	ctxUserEmail ctxKey = "user_email"
	ctxUserRole  ctxKey = "user_role"
	ctxTokenID   ctxKey = "token_id"
)

var jwtSecret []byte

type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func generateToken(user User) (string, error) {
	claims := Claims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        randomHex(16),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(jwtSecret)
}

func validateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, fmt.Errorf("invalid token")
}

func cookieSecure() bool { return os.Getenv("COOKIE_SECURE") == "true" }

func setAuthCookies(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name: "token", Value: token, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: cookieSecure(),
		MaxAge: int((24 * time.Hour).Seconds()),
	})
	http.SetCookie(w, &http.Cookie{
		Name: "csrf_token", Value: randomHex(16), Path: "/",
		SameSite: http.SameSiteLaxMode, Secure: cookieSecure(),
		MaxAge: int((24 * time.Hour).Seconds()),
	})
}

func clearAuthCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: "token", Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
	http.SetCookie(w, &http.Cookie{Name: "csrf_token", Value: "", Path: "/", MaxAge: -1})
}

// CSRF: double-submit cookie. State-changing API calls must echo the
// csrf_token cookie in the X-CSRF-Token header. Login/register are pre-session.
func csrfMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		if p := r.URL.Path; p == "/api/v1/auth/login" || p == "/api/v1/auth/register" {
			next.ServeHTTP(w, r)
			return
		}
		header := r.Header.Get("X-CSRF-Token")
		cookie, err := r.Cookie("csrf_token")
		if header == "" || err != nil || header != cookie.Value {
			jsonError(w, "CSRF token mismatch", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			if cookie, err := r.Cookie("token"); err == nil {
				authHeader = "Bearer " + cookie.Value
			} else {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		claims, err := validateToken(parts[1])
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		var revoked int
		db.QueryRow("SELECT COUNT(*) FROM revoked_tokens WHERE token_id = ?", claims.ID).Scan(&revoked)
		if revoked > 0 {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		ctx := context.WithValue(r.Context(), ctxUserID, claims.UserID)
		ctx = context.WithValue(ctx, ctxUserEmail, claims.Email)
		ctx = context.WithValue(ctx, ctxUserRole, claims.Role)
		ctx = context.WithValue(ctx, ctxTokenID, claims.ID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Context().Value(ctxUserRole) != RoleAdmin {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requirePlatformRole gates endpoints by platform role (RBAC).
func requirePlatformRole(roles ...string) func(http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, _ := r.Context().Value(ctxUserRole).(string)
			if !allowed[role] {
				jsonError(w, "Insufficient role: requires "+strings.Join(roles, " or "), http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
