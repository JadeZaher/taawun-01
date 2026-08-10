package services

import (
	"fmt"
	"time"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"taawun/pkg/models"
	"taawun/pkg/repositories"
)

type AuthService struct {
	userRepo *repositories.UserRepository
	jwtSecret []byte
}

func NewAuthService(userRepo *repositories.UserRepository) *AuthService {
	return &AuthService{
		userRepo: userRepo,
		jwtSecret: []byte("your-secret-key-change-in-production"),
	}
}

func (s *AuthService) Login(email, password string) (*models.LoginResponse, error) {
	user, err := s.userRepo.GetByEmail(email)
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

	token, err := s.generateToken(user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %v", err)
	}

	return &models.LoginResponse{
		Token: token,
		User:  *user,
	}, nil
}

func (s *AuthService) ValidateToken(tokenString string) (*models.User, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %v", err)
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		userIDFloat, ok := claims["user_id"].(float64)
		if !ok {
			return nil, fmt.Errorf("invalid user_id in token")
		}
		userID := int(userIDFloat)

		user, err := s.userRepo.GetByID(userID)
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

	return nil, fmt.Errorf("invalid token")
}

func (s *AuthService) generateToken(user *models.User) (string, error) {
	claims := jwt.MapClaims{
		"user_id":    user.ID,
		"username":   user.Username,
		"email":      user.Email,
		"role":       user.Role,
		"exp":        time.Now().Add(time.Hour * 24).Unix(),
		"iat":        time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}