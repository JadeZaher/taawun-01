package handlers

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gorilla/mux"

	"taawun/pkg/database"
	"taawun/pkg/models"
	"taawun/pkg/repositories"
	"taawun/pkg/services"
)

func TestDeleteUserDoesNotExposePersistenceFailure(t *testing.T) {
	t.Setenv("APP_DB_PATH", filepath.Join(t.TempDir(), "user-delete.db"))
	t.Setenv("TAWUN_BOOTSTRAP_ADMIN_USERNAME", "")
	t.Setenv("TAWUN_BOOTSTRAP_ADMIN_EMAIL", "")
	t.Setenv("TAWUN_BOOTSTRAP_ADMIN_PASSWORD", "")
	db, err := database.InitDB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := database.SQLDB(db)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	user := &models.User{Username: "delete-failure", Email: "delete-failure@example.com", Password: "unused", Role: models.RoleUser, Status: models.StatusActive}
	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.Exec(`CREATE TRIGGER fail_account_anonymization BEFORE UPDATE ON users BEGIN SELECT RAISE(ABORT, 'raw FOREIGN KEY constraint detail'); END`); err != nil {
		t.Fatal(err)
	}

	handler := NewUserHandler(services.NewUserService(repositories.NewUserRepository(db)))
	request := httptest.NewRequest(http.MethodDelete, "/api/users/"+strconv.Itoa(user.ID), nil)
	request = mux.SetURLVars(request, map[string]string{"id": strconv.Itoa(user.ID)})
	request = withCurrentUser(request, user)
	response := httptest.NewRecorder()
	handler.DeleteUser(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	body := response.Body.String()
	if !strings.Contains(body, `"code":"account_deletion_failed"`) {
		t.Fatalf("response body = %s", body)
	}
	if strings.Contains(body, "FOREIGN KEY") || strings.Contains(body, "constraint") {
		t.Fatalf("response exposed persistence details: %s", body)
	}
}
