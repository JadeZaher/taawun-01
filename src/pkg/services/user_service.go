package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"taawun/pkg/models"
	"taawun/pkg/repositories"
)

var (
	ErrUserNotFound             = errors.New("user not found")
	ErrOwnedWorkspacesRemaining = errors.New("owned workspaces remaining")
	ErrAccountDeletionFailed    = errors.New("account deletion failed")
)

type UserService struct {
	repo *repositories.UserRepository
}

func NewUserService(repo *repositories.UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) CreateUser(req *models.RegisterRequest) (*models.User, error) {
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	// Validate input
	if err := s.validateRegisterRequest(req); err != nil {
		return nil, err
	}

	// Check if user exists
	existing, err := s.repo.GetByEmail(req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to check user existence: %v", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("user with email %s already exists", req.Email)
	}

	existing, err = s.repo.GetByUsername(req.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to check user existence: %v", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("user with username %s already exists", req.Username)
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %v", err)
	}

	user := &models.User{
		Username: req.Username,
		Email:    req.Email,
		Password: string(hashedPassword),
		Role:     models.RoleUser,
		Status:   models.StatusActive,
	}

	if err := s.repo.Create(user); err != nil {
		return nil, fmt.Errorf("failed to create user: %v", err)
	}

	return user, nil
}

func (s *UserService) GetUser(id int) (*models.User, error) {
	user, err := s.repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %v", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}
	return user, nil
}

func (s *UserService) GetUserByEmail(email string) (*models.User, error) {
	user, err := s.repo.GetByEmail(email)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %v", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}
	return user, nil
}

func (s *UserService) GetAllUsers() ([]*models.User, error) {
	users, err := s.repo.GetAll()
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %v", err)
	}
	return users, nil
}

func (s *UserService) UpdateUser(id int, req *models.UpdateUserRequest) (*models.User, error) {
	user, err := s.repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %v", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	if req.Username != "" {
		user.Username = strings.TrimSpace(req.Username)
	}
	if req.Email != "" {
		user.Email = strings.ToLower(strings.TrimSpace(req.Email))
	}
	rotateSessions := false
	if req.Password != "" {
		if err := validatePassword(req.Password); err != nil {
			return nil, err
		}
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %v", err)
		}
		user.Password = string(hashedPassword)
		rotateSessions = true
	}

	if rotateSessions {
		if err := s.repo.UpdateAndRotateSessions(user); err != nil {
			return nil, fmt.Errorf("failed to update user: %v", err)
		}
	} else if err := s.repo.Update(user); err != nil {
		return nil, fmt.Errorf("failed to update user: %v", err)
	}

	return user, nil
}

func (s *UserService) DeleteUser(id int) error {
	return s.DeleteUserContext(context.Background(), id)
}

func (s *UserService) DeleteUserContext(ctx context.Context, id int) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil {
		return fmt.Errorf("%w: lifecycle transaction unavailable", ErrAccountDeletionFailed)
	}
	random := make([]byte, 24)
	if _, err := rand.Read(random); err != nil {
		return fmt.Errorf("%w: prepare account tombstone", ErrAccountDeletionFailed)
	}
	suffix := hex.EncodeToString(random[:12])
	hashedPassword, err := bcrypt.GenerateFromPassword(random, bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("%w: prepare account credentials", ErrAccountDeletionFailed)
	}
	if err := s.repo.DeactivateAndAnonymizeContext(ctx, id, "deleted-"+suffix, "deleted-"+suffix+"@deleted.invalid", string(hashedPassword), time.Now().UTC()); err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			return ErrUserNotFound
		}
		if errors.Is(err, repositories.ErrOwnedWorkspacesRemaining) {
			return ErrOwnedWorkspacesRemaining
		}
		if errors.Is(err, repositories.ErrLifecycleUnavailable) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("%w: lifecycle transaction unavailable", ErrAccountDeletionFailed)
		}
		return fmt.Errorf("%w: %v", ErrAccountDeletionFailed, err)
	}
	return nil
}

func (s *UserService) validateRegisterRequest(req *models.RegisterRequest) error {
	if req.Username == "" {
		return fmt.Errorf("username is required")
	}
	if len(req.Username) < 3 {
		return fmt.Errorf("username must be at least 3 characters")
	}

	if req.Email == "" {
		return fmt.Errorf("email is required")
	}
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(req.Email) {
		return fmt.Errorf("invalid email format")
	}

	return validatePassword(req.Password)
}

func validatePassword(password string) error {
	if password == "" {
		return fmt.Errorf("password is required")
	}
	if len(password) < 12 {
		return fmt.Errorf("password must be at least 12 characters")
	}
	return nil
}
