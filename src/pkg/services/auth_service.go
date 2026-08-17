package services

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"taawun/pkg/models"
	"taawun/pkg/repositories"
)

const (
	jwtIssuer   = "taawun"
	jwtAudience = "taawun-web"
	jwtLifetime = 12 * time.Hour
)

type AuthService struct {
	userRepo  *repositories.UserRepository
	jwtSecret []byte
}

type accessTokenClaims struct {
	UserID int `json:"user_id"`
	jwt.RegisteredClaims
}

func NewAuthService(userRepo *repositories.UserRepository, jwtSecret []byte) (*AuthService, error) {
	if len(jwtSecret) < 32 {
		return nil, fmt.Errorf("JWT secret must contain at least 32 bytes")
	}
	secretCopy := append([]byte(nil), jwtSecret...)
	return &AuthService{userRepo: userRepo, jwtSecret: secretCopy}, nil
}

func (s *AuthService) Login(email, password string) (*models.LoginResponse, error) {
	user, err := s.Authenticate(email, password)
	if err != nil {
		return nil, err
	}

	token, err := s.generateToken(user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %v", err)
	}

	return &models.LoginResponse{
		Token: token,
		User:  *user,
	}, nil
}

// Authenticate verifies platform credentials without minting a first-party JWT.
func (s *AuthService) Authenticate(email, password string) (*models.User, error) {
	user, err := s.userRepo.GetByEmail(strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %v", err)
	}
	if user == nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	if user.Status != models.StatusActive {
		return nil, fmt.Errorf("account is not active")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	return user, nil
}

func (s *AuthService) ValidateToken(tokenString string) (*models.User, error) {
	claims := &accessTokenClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.jwtSecret, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(jwtIssuer),
		jwt.WithAudience(jwtAudience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
	)

	if err != nil || !token.Valid || claims.UserID <= 0 || claims.Subject != strconv.Itoa(claims.UserID) {
		return nil, fmt.Errorf("invalid token")
	}

	user, err := s.userRepo.GetByID(claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %v", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}
	if user.Status != models.StatusActive {
		return nil, fmt.Errorf("account is not active")
	}
	return user, nil
}

func (s *AuthService) generateToken(user *models.User) (string, error) {
	now := time.Now().UTC()
	claims := accessTokenClaims{
		UserID: user.ID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    jwtIssuer,
			Subject:   strconv.Itoa(user.ID),
			Audience:  jwt.ClaimStrings{jwtAudience},
			ExpiresAt: jwt.NewNumericDate(now.Add(jwtLifetime)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}
