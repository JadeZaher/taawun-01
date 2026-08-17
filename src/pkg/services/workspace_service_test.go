package services

import (
	"errors"
	"testing"

	"taawun/pkg/models"
)

func TestWorkspaceServiceEnforcesMembershipAndManagementRoles(t *testing.T) {
	db, userRepo, workspaceRepo := newSecurityTestRepositories(t)
	users := []*models.User{
		{Username: "owner", Email: "owner@example.com", Password: "x", Role: models.RoleUser, Status: models.StatusActive},
		{Username: "workspace-admin", Email: "workspace-admin@example.com", Password: "x", Role: models.RoleUser, Status: models.StatusActive},
		{Username: "member", Email: "member2@example.com", Password: "x", Role: models.RoleUser, Status: models.StatusActive},
		{Username: "outsider", Email: "outsider@example.com", Password: "x", Role: models.RoleUser, Status: models.StatusActive},
		{Username: "platform-admin", Email: "platform-admin@example.com", Password: "x", Role: models.RoleAdmin, Status: models.StatusActive},
		{Username: "viewer", Email: "viewer@example.com", Password: "x", Role: models.RoleUser, Status: models.StatusActive},
	}
	for _, user := range users {
		insertSecurityTestUser(t, db, user)
	}

	service := NewWorkspaceService(workspaceRepo, userRepo)
	workspace, err := service.CreateWorkspace(users[0], &models.CreateWorkspaceRequest{Name: "Relief"})
	if err != nil {
		t.Fatal(err)
	}
	if err := workspaceRepo.AddUser(workspace.ID, users[1].ID, models.WorkspaceRoleAdmin); err != nil {
		t.Fatal(err)
	}
	if err := workspaceRepo.AddUser(workspace.ID, users[2].ID, models.WorkspaceRoleMember); err != nil {
		t.Fatal(err)
	}
	if err := workspaceRepo.AddUser(workspace.ID, users[5].ID, models.WorkspaceRoleViewer); err != nil {
		t.Fatal(err)
	}

	if _, err := service.GetWorkspace(users[2], workspace.ID); err != nil {
		t.Fatalf("member should be able to read workspace: %v", err)
	}
	if _, err := service.GetWorkspace(users[3], workspace.ID); !errors.Is(err, ErrWorkspaceForbidden) {
		t.Fatalf("outsider read error = %v, want forbidden", err)
	}
	if _, err := service.UpdateWorkspace(users[2], workspace.ID, &models.UpdateWorkspaceRequest{Name: "Denied"}); !errors.Is(err, ErrWorkspaceForbidden) {
		t.Fatalf("member update error = %v, want forbidden", err)
	}
	if _, err := service.UpdateWorkspace(users[1], workspace.ID, &models.UpdateWorkspaceRequest{Name: "Managed"}); err != nil {
		t.Fatalf("workspace admin should be able to update: %v", err)
	}
	if err := service.DeleteWorkspace(users[1], workspace.ID); !errors.Is(err, ErrWorkspaceForbidden) {
		t.Fatalf("workspace admin delete error = %v, want owner-only denial", err)
	}
	if err := service.AddUserToWorkspace(users[0], workspace.ID, users[3].ID, models.WorkspaceRoleOwner); err == nil {
		t.Fatal("expected assignment of a second owner to be rejected")
	}
	if err := service.AddUserToWorkspace(users[1], workspace.ID, users[3].ID, ""); err != nil {
		t.Fatalf("workspace admin should be able to add a default member: %v", err)
	}
	role, err := workspaceRepo.GetUserRole(workspace.ID, users[3].ID)
	if err != nil || role != models.WorkspaceRoleMember {
		t.Fatalf("added role = %q, err = %v", role, err)
	}
	if _, err := service.GetWorkspace(users[4], workspace.ID); err != nil {
		t.Fatalf("platform admin should be able to read workspace: %v", err)
	}
	if _, err := service.AuthorizeWorkspaceCapability(users[5], workspace.ID, models.WorkspaceCapabilityAudit); err != nil {
		t.Fatalf("viewer should have audit capability: %v", err)
	}
	if _, err := service.AuthorizeWorkspaceCapability(users[5], workspace.ID, models.WorkspaceCapabilityBuild); !errors.Is(err, ErrWorkspaceForbidden) {
		t.Fatalf("viewer build error = %v, want forbidden", err)
	}
	if _, err := service.AuthorizeWorkspaceCapability(users[2], workspace.ID, models.WorkspaceCapabilityBuild); err != nil {
		t.Fatalf("member should have build capability: %v", err)
	}
	if _, err := service.AuthorizeWorkspaceCapability(users[1], workspace.ID, models.WorkspaceCapabilityPublish); err != nil {
		t.Fatalf("workspace admin should have publish capability: %v", err)
	}
}
