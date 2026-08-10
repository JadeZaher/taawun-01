package repositories

import (
	"database/sql"
	"fmt"
	"time"

	"taawun/pkg/models"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(user *models.User) error {
	query := `INSERT INTO users (username, email, password, role, status) 
		VALUES (?, ?, ?, ?, ?) RETURNING id, created_at, updated_at`
	
	err := r.db.QueryRow(query, user.Username, user.Email, user.Password, user.Role, user.Status).
		Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create user: %v", err)
	}
	return nil
}

func (r *UserRepository) GetByID(id int) (*models.User, error) {
	query := `SELECT id, username, email, password, role, status, created_at, updated_at 
		FROM users WHERE id = ?`
	
	var user models.User
	err := r.db.QueryRow(query, id).Scan(
		&user.ID, &user.Username, &user.Email, &user.Password,
		&user.Role, &user.Status, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %v", err)
	}
	return &user, nil
}

func (r *UserRepository) GetByEmail(email string) (*models.User, error) {
	query := `SELECT id, username, email, password, role, status, created_at, updated_at 
		FROM users WHERE email = ?`
	
	var user models.User
	err := r.db.QueryRow(query, email).Scan(
		&user.ID, &user.Username, &user.Email, &user.Password,
		&user.Role, &user.Status, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by email: %v", err)
	}
	return &user, nil
}

func (r *UserRepository) GetByUsername(username string) (*models.User, error) {
	query := `SELECT id, username, email, password, role, status, created_at, updated_at 
		FROM users WHERE username = ?`
	
	var user models.User
	err := r.db.QueryRow(query, username).Scan(
		&user.ID, &user.Username, &user.Email, &user.Password,
		&user.Role, &user.Status, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by username: %v", err)
	}
	return &user, nil
}

func (r *UserRepository) GetAll() ([]*models.User, error) {
	query := `SELECT id, username, email, password, role, status, created_at, updated_at 
		FROM users ORDER BY id DESC`
	
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %v", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		var user models.User
		err := rows.Scan(
			&user.ID, &user.Username, &user.Email, &user.Password,
			&user.Role, &user.Status, &user.CreatedAt, &user.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %v", err)
		}
		users = append(users, &user)
	}
	return users, nil
}

func (r *UserRepository) Update(user *models.User) error {
	query := `UPDATE users SET username = ?, email = ?, password = ?, role = ?, status = ?, updated_at = ? 
		WHERE id = ?`
	
	result, err := r.db.Exec(query, user.Username, user.Email, user.Password, user.Role, user.Status, time.Now(), user.ID)
	if err != nil {
		return fmt.Errorf("failed to update user: %v", err)
	}
	
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}
	if rows == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (r *UserRepository) Delete(id int) error {
	query := `DELETE FROM users WHERE id = ?`
	
	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %v", err)
	}
	
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}
	if rows == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (r *UserRepository) UpdateRole(id int, role string) error {
	query := `UPDATE users SET role = ?, updated_at = ? WHERE id = ?`
	
	result, err := r.db.Exec(query, role, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update user role: %v", err)
	}
	
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}
	if rows == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (r *UserRepository) UpdateStatus(id int, status string) error {
	query := `UPDATE users SET status = ?, updated_at = ? WHERE id = ?`
	
	result, err := r.db.Exec(query, status, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update user status: %v", err)
	}
	
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}
	if rows == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (r *UserRepository) Count() (int, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %v", err)
	}
	return count, nil
}

func (r *UserRepository) CountActive() (int, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM users WHERE status = 'active'").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count active users: %v", err)
	}
	return count, nil
}

func (r *UserRepository) CountByRole(role string) (int, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM users WHERE role = ?", role).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count users by role: %v", err)
	}
	return count, nil
}

func (r *UserRepository) GetWorkspaceUsers(workspaceID int) ([]*models.User, error) {
	query := `SELECT u.id, u.username, u.email, u.password, u.role, u.status, u.created_at, u.updated_at 
		FROM users u
		INNER JOIN workspace_users wu ON u.id = wu.user_id
		WHERE wu.workspace_id = ?`
	
	rows, err := r.db.Query(query, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace users: %v", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		var user models.User
		err := rows.Scan(
			&user.ID, &user.Username, &user.Email, &user.Password,
			&user.Role, &user.Status, &user.CreatedAt, &user.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %v", err)
		}
		users = append(users, &user)
	}
	return users, nil
}