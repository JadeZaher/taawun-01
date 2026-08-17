package services

import (
	"strconv"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"taawun/pkg/models"
)

func TestAuthServiceRequiresStrongSecretAndValidatesTypedClaims(t *testing.T) {
	db, userRepo, _ := newSecurityTestRepositories(t)
	if _, err := NewAuthService(userRepo, []byte("too-short")); err == nil {
		t.Fatal("expected a short JWT secret to be rejected")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("a-long-test-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	user := &models.User{
		Username: "member",
		Email:    "member@example.com",
		Password: string(hash),
		Role:     models.RoleUser,
		Status:   models.StatusActive,
	}
	insertSecurityTestUser(t, db, user)

	secret := []byte("0123456789abcdef0123456789abcdef")
	service, err := NewAuthService(userRepo, secret)
	if err != nil {
		t.Fatal(err)
	}
	login, err := service.Login(" MEMBER@EXAMPLE.COM ", "a-long-test-password")
	if err != nil {
		t.Fatal(err)
	}
	authenticated, err := service.ValidateToken(login.Token)
	if err != nil {
		t.Fatal(err)
	}
	if authenticated.ID != user.ID {
		t.Fatalf("authenticated user %d, want %d", authenticated.ID, user.ID)
	}
	if authenticated.SessionVersion != 1 {
		t.Fatalf("session version = %d, want 1", authenticated.SessionVersion)
	}

	now := time.Now().UTC()
	claims := accessTokenClaims{
		UserID: user.ID, SessionVersion: authenticated.SessionVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    jwtIssuer,
			Subject:   strconv.Itoa(user.ID + 1),
			Audience:  jwt.ClaimStrings{jwtAudience},
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	malformed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ValidateToken(malformed); err == nil {
		t.Fatal("expected mismatched typed subject and user ID to be rejected")
	}

	if _, err := db.Exec(`UPDATE users SET status = ? WHERE id = ?`, models.StatusSuspended, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ValidateToken(login.Token); err == nil {
		t.Fatal("expected a token for a suspended user to be rejected")
	}
}

func TestPasswordUpdateRevokesExistingAccessTokens(t *testing.T) {
	db, userRepo, _ := newSecurityTestRepositories(t)
	userService := NewUserService(userRepo)
	created, err := userService.CreateUser(&models.RegisterRequest{
		Username: "member", Email: "member@example.com", Password: "initial-password",
	})
	if err != nil {
		t.Fatal(err)
	}
	authService, err := NewAuthService(userRepo, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	login, err := authService.Login(created.Email, "initial-password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := userService.UpdateUser(created.ID, &models.UpdateUserRequest{Password: "replacement-password"}); err != nil {
		t.Fatal(err)
	}
	if _, err := authService.ValidateToken(login.Token); err == nil {
		t.Fatal("expected password replacement to revoke the prior token")
	}
	if _, err := authService.Login(created.Email, "replacement-password"); err != nil {
		t.Fatalf("expected replacement password to authenticate: %v", err)
	}
	if _, err := userService.UpdateUser(created.ID, &models.UpdateUserRequest{Password: "too-short"}); err == nil {
		t.Fatal("expected password update to enforce registration password policy")
	}
	var version int64
	if err := db.QueryRow(`SELECT session_version FROM users WHERE id = ?`, created.ID).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 2 {
		t.Fatalf("stored session version = %d, want 2", version)
	}
}
