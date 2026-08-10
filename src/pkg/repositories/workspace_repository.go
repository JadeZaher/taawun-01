package repositories

import (
	"database/sql"
	"fmt"
	"time"

	"taawun/pkg/models"
)

type WorkspaceRepository struct {
	db *sql.DB
}

func NewWorkspaceRepository(db *sql.DB) *WorkspaceRepository {
	return &WorkspaceRepository{db: db}
}

func (r *WorkspaceRepository) Create(workspace *models.Workspace) error {
	query := `INSERT INTO workspaces (name, description, owner_id, status) 
		VALUES (?, ?, ?, ?) RETURNING id, created_at, updated_at`
	
	err := r.db.QueryRow(query, workspace.Name, workspace.Description, workspace.OwnerID, workspace.Status).
		Scan(&workspace.ID, &workspace.CreatedAt, &workspace.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create workspace: %v", err)
	}
	return nil
}

func (r *WorkspaceRepository) GetByID(id int) (*models.Workspace, error) {
	query := `SELECT id, name, description, owner_id, status, created_at, updated_at 
		FROM workspaces WHERE id = ?`
	
	var workspace models.Workspace
	err := r.db.QueryRow(query, id).Scan(
		&workspace.ID, &workspace.Name, &workspace.Description,
		&workspace.OwnerID, &workspace.Status, &workspace.CreatedAt, &workspace.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace: %v", err)
	}
	return &workspace, nil
}

func (r *WorkspaceRepository) GetAll() ([]*models.Workspace, error) {
	query := `SELECT id, name, description, owner_id, status, created_at, updated_at 
		FROM workspaces ORDER BY id DESC`
	
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspaces: %v", err)
	}
	defer rows.Close()

	var workspaces []*models.Workspace
	for rows.Next() {
		var workspace models.Workspace
		err := rows.Scan(
			&workspace.ID, &workspace.Name, &workspace.Description,
			&workspace.OwnerID, &workspace.Status, &workspace.CreatedAt, &workspace.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan workspace: %v", err)
		}
		workspaces = append(workspaces, &workspace)
	}
	return workspaces, nil
}

func (r *WorkspaceRepository) GetByOwner(ownerID int) ([]*models.Workspace, error) {
	query := `SELECT id, name, description, owner_id, status, created_at, updated_at 
		FROM workspaces WHERE owner_id = ? ORDER BY id DESC`
	
	rows, err := r.db.Query(query, ownerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspaces by owner: %v", err)
	}
	defer rows.Close()

	var workspaces []*models.Workspace
	for rows.Next() {
		var workspace models.Workspace
		err := rows.Scan(
			&workspace.ID, &workspace.Name, &workspace.Description,
			&workspace.OwnerID, &workspace.Status, &workspace.CreatedAt, &workspace.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan workspace: %v", err)
		}
		workspaces = append(workspaces, &workspace)
	}
	return workspaces, nil
}

func (r *WorkspaceRepository) GetByUser(userID int) ([]*models.Workspace, error) {
	query := `SELECT w.id, w.name, w.description, w.owner_id, w.status, w.created_at, w.updated_at 
		FROM workspaces w
		INNER JOIN workspace_users wu ON w.id = wu.workspace_id
		WHERE wu.user_id = ?
		ORDER BY w.id DESC`
	
	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspaces by user: %v", err)
	}
	defer rows.Close()

	var workspaces []*models.Workspace
	for rows.Next() {
		var workspace models.Workspace
		err := rows.Scan(
			&workspace.ID, &workspace.Name, &workspace.Description,
			&workspace.OwnerID, &workspace.Status, &workspace.CreatedAt, &workspace.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan workspace: %v", err)
		}
		workspaces = append(workspaces, &workspace)
	}
	return workspaces, nil
}

func (r *WorkspaceRepository) Update(workspace *models.Workspace) error {
	query := `UPDATE workspaces SET name = ?, description = ?, status = ?, updated_at = ? 
		WHERE id = ?`
	
	result, err := r.db.Exec(query, workspace.Name, workspace.Description, workspace.Status, time.Now(), workspace.ID)
	if err != nil {
		return fmt.Errorf("failed to update workspace: %v", err)
	}
	
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}
	if rows == 0 {
		return fmt.Errorf("workspace not found")
	}
	return nil
}

func (r *WorkspaceRepository) Delete(id int) error {
	query := `DELETE FROM workspaces WHERE id = ?`
	
	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete workspace: %v", err)
	}
	
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}
	if rows == 0 {
		return fmt.Errorf("workspace not found")
	}
	return nil
}

func (r *WorkspaceRepository) AddUser(workspaceID, userID int, role string) error {
	query := `INSERT INTO workspace_users (workspace_id, user_id, role) VALUES (?, ?, ?)`
	
	_, err := r.db.Exec(query, workspaceID, userID, role)
	if err != nil {
		return fmt.Errorf("failed to add user to workspace: %v", err)
	}
	return nil
}

func (r *WorkspaceRepository) RemoveUser(workspaceID, userID int) error {
	query := `DELETE FROM workspace_users WHERE workspace_id = ? AND user_id = ?`
	
	result, err := r.db.Exec(query, workspaceID, userID)
	if err != nil {
		return fmt.Errorf("failed to remove user from workspace: %v", err)
	}
	
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}
	if rows == 0 {
		return fmt.Errorf("user not found in workspace")
	}
	return nil
}

func (r *WorkspaceRepository) GetUserRole(workspaceID, userID int) (string, error) {
	var role string
	query := `SELECT role FROM workspace_users WHERE workspace_id = ? AND user_id = ?`
	err := r.db.QueryRow(query, workspaceID, userID).Scan(&role)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get user role: %v", err)
	}
	return role, nil
}

func (r *WorkspaceRepository) Count() (int, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM workspaces").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count workspaces: %v", err)
	}
	return count, nil
}

func (r *WorkspaceRepository) CountActive() (int, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM workspaces WHERE status = 'active'").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count active workspaces: %v", err)
	}
	return count, nil
}

func (r *WorkspaceRepository) CountByStatus(status string) (int, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM workspaces WHERE status = ?", status).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count workspaces by status: %v", err)
	}
	return count, nil
}