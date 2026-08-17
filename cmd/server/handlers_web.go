package main

import (
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/gorilla/mux"
)

var templates map[string]*template.Template

var templatePages = []string{
	"index.html", "login.html", "register.html",
	"dashboard.html", "workspaces.html", "workspace_detail.html", "admin.html",
}

func initTemplates() {
	templates = make(map[string]*template.Template)
	dir := "internal/web/templates"
	for _, page := range templatePages {
		templates[page] = template.Must(template.ParseFiles(
			filepath.Join(dir, "layout.html"),
			filepath.Join(dir, page),
		))
	}
}

func renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	tmpl, ok := templates[name]
	if !ok {
		http.Error(w, "template not found: "+name, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// baseData always includes the nav keys so layout.html never sees a missing map key.
func baseData(r *http.Request, title string) map[string]interface{} {
	data := map[string]interface{}{"PageTitle": title, "UserID": "", "UserName": "", "UserRole": ""}
	if v, ok := r.Context().Value(ctxUserID).(string); ok {
		data["UserID"] = v
		data["UserName"], _ = r.Context().Value(ctxUserEmail).(string)
		data["UserRole"], _ = r.Context().Value(ctxUserRole).(string)
	}
	return data
}

func landingHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "index.html", baseData(r, "Welcome"))
}
func loginPageHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "login.html", baseData(r, "Login"))
}
func registerPageHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "register.html", baseData(r, "Register"))
}
func dashboardHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "dashboard.html", baseData(r, "Dashboard"))
}
func workspacesPageHandler(w http.ResponseWriter, r *http.Request) {
	data := baseData(r, "Workspaces")
	role, _ := r.Context().Value(ctxUserRole).(string)
	data["CanCreate"] = canCreateWorkspace(role)
	renderTemplate(w, "workspaces.html", data)
}
func workspaceDetailPageHandler(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(ctxUserID).(string)
	platformRole, _ := r.Context().Value(ctxUserRole).(string)
	id := mux.Vars(r)["id"]
	ownerID, wsRole, err := wsAccess(id, userID)
	if err != nil || (wsRole == "" && ownerID != userID && platformRole != RoleAdmin) {
		http.Redirect(w, r, "/workspaces", http.StatusSeeOther)
		return
	}
	data := baseData(r, "Workspace")
	data["WorkspaceID"] = id
	canManage := platformRole == RoleAdmin || ownerID == userID || wsRole == RoleArchitect
	data["CanEdit"] = canManage || wsRole == RoleMaintainer
	data["CanManage"] = canManage
	renderTemplate(w, "workspace_detail.html", data)
}
func adminPageHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "admin.html", baseData(r, "Admin Panel"))
}
func webLogoutHandler(w http.ResponseWriter, r *http.Request) {
	if tokenID, ok := r.Context().Value(ctxTokenID).(string); ok {
		db.Exec(`INSERT OR IGNORE INTO revoked_tokens (token_id, expires_at) VALUES (?, datetime('now', '+1 day'))`, tokenID)
	}
	clearAuthCookies(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
