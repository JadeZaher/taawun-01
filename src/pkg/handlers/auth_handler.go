package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"taawun/pkg/models"
	"taawun/pkg/services"
)

const (
	maximumPublicAuthBodyBytes = 16 << 10
	publicAuthWindow           = 15 * time.Minute
	maximumPublicAuthClients   = 4096
	registrationAttemptLimit   = 5
	loginAttemptLimit          = 10
)

type AuthHandler struct {
	authService   *services.AuthService
	userService   *services.UserService
	publicLimiter *authRateLimiter
}

func NewAuthHandler(authService *services.AuthService, userService *services.UserService) *AuthHandler {
	return &AuthHandler{
		authService:   authService,
		userService:   userService,
		publicLimiter: newAuthRateLimiter(publicAuthWindow, maximumPublicAuthClients),
	}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if !h.allowPublicAttempt(w, r, "register", registrationAttemptLimit) {
		return
	}
	var req models.RegisterRequest
	if !decodeBoundedJSON(w, r, &req, maximumPublicAuthBodyBytes) {
		return
	}

	user, err := h.userService.CreateUser(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if !h.allowPublicAttempt(w, r, "login", loginAttemptLimit) {
		return
	}
	var req models.LoginRequest
	if !decodeBoundedJSON(w, r, &req, maximumPublicAuthBodyBytes) {
		return
	}

	response, err := h.authService.Login(req.Email, req.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *AuthHandler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		parts := strings.Fields(authHeader)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
			return
		}

		token := parts[1]
		if len(token) > 8192 {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}
		user, err := h.authService.ValidateToken(token)
		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		ctx := WithCurrentUser(r.Context(), user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type contextKey string

const userContextKey contextKey = "taawun-user"

// WithCurrentUser binds a verified platform identity to a request context.
func WithCurrentUser(ctx context.Context, user *models.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

func CurrentUser(ctx context.Context) (*models.User, bool) {
	user, ok := ctx.Value(userContextKey).(*models.User)
	return user, ok && user != nil
}

func (h *AuthHandler) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := CurrentUser(r.Context())
		if !ok {
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}
		if user.Role != models.RoleAdmin {
			http.Error(w, "Administrator access required", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *AuthHandler) allowPublicAttempt(w http.ResponseWriter, r *http.Request, operation string, limit int) bool {
	if h.publicLimiter == nil {
		h.publicLimiter = newAuthRateLimiter(publicAuthWindow, maximumPublicAuthClients)
	}
	allowed, retryAfter := h.publicLimiter.Allow(operation, requestSource(r), limit)
	if allowed {
		return true
	}
	seconds := int(retryAfter.Round(time.Second) / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	http.Error(w, "Too many authentication attempts. Please try again later.", http.StatusTooManyRequests)
	return false
}

// decodeBoundedJSON accepts one strict JSON object with a small endpoint-specific limit.
func decodeBoundedJSON(w http.ResponseWriter, r *http.Request, target any, maximumBytes int64) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return false
	}
	if r.ContentLength > maximumBytes {
		http.Error(w, "Request body is too large", http.StatusRequestEntityTooLarge)
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maximumBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "Request body is too large", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
		}
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return false
	}
	return true
}

type authRateLimitEntry struct {
	started  time.Time
	attempts int
}

// authRateLimiter deliberately bounds both the request window and its in-memory accounting.
type authRateLimiter struct {
	mu         sync.Mutex
	entries    map[string]authRateLimitEntry
	window     time.Duration
	maxEntries int
	now        func() time.Time
}

func newAuthRateLimiter(window time.Duration, maxEntries int) *authRateLimiter {
	return &authRateLimiter{entries: make(map[string]authRateLimitEntry), window: window, maxEntries: maxEntries, now: time.Now}
}

func (l *authRateLimiter) Allow(operation, source string, maximumAttempts int) (bool, time.Duration) {
	now := l.now().UTC()
	key := operation + "\x00" + source
	l.mu.Lock()
	defer l.mu.Unlock()
	for candidate, entry := range l.entries {
		if !entry.started.Add(l.window).After(now) {
			delete(l.entries, candidate)
		}
	}
	entry, exists := l.entries[key]
	if !exists {
		if len(l.entries) >= l.maxEntries {
			return false, l.window
		}
		l.entries[key] = authRateLimitEntry{started: now, attempts: 1}
		return true, 0
	}
	if entry.attempts >= maximumAttempts {
		return false, entry.started.Add(l.window).Sub(now)
	}
	entry.attempts++
	l.entries[key] = entry
	return true, 0
}

func requestSource(r *http.Request) string {
	remote := strings.TrimSpace(r.RemoteAddr)
	if host, _, err := net.SplitHostPort(remote); err == nil && host != "" {
		return host
	}
	if remote != "" {
		return remote
	}
	return "unknown"
}
