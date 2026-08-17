package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, map[string]string{
		"status":  "healthy",
		"service": "taawun",
		"version": "2.1.0",
	}, http.StatusOK)
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Fatal("JWT_SECRET environment variable is required (32+ random chars)")
	}
	jwtSecret = []byte(secret)

	if err := initDB(); err != nil {
		log.Fatal("Database initialization failed:", err)
	}
	defer db.Close()
	log.Println("Database connected")

	initTemplates()
	log.Println("Templates loaded")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	router := mux.NewRouter()

	// Static assets
	wd, _ := os.Getwd()
	fs := http.FileServer(http.Dir(filepath.Join(wd, "internal/web/static")))
	router.PathPrefix("/css/").Handler(fs)
	router.PathPrefix("/js/").Handler(fs)

	// Web pages
	router.HandleFunc("/", landingHandler).Methods("GET")
	router.HandleFunc("/login", loginPageHandler).Methods("GET")
	router.HandleFunc("/register", registerPageHandler).Methods("GET")

	authDash := authMiddleware(http.HandlerFunc(dashboardHandler))
	router.Handle("/dashboard", authDash).Methods("GET")

	authWs := authMiddleware(http.HandlerFunc(workspacesPageHandler))
	router.Handle("/workspaces", authWs).Methods("GET")

	authWsDetail := authMiddleware(http.HandlerFunc(workspaceDetailPageHandler))
	router.Handle("/workspaces/{id}", authWsDetail).Methods("GET")

	authAdmin := authMiddleware(requireAdmin(http.HandlerFunc(adminPageHandler)))
	router.Handle("/admin", authAdmin).Methods("GET")

	authLogout := authMiddleware(http.HandlerFunc(webLogoutHandler))
	router.Handle("/logout", authLogout).Methods("GET")

	// API: health + auth
	router.HandleFunc("/health", healthHandler).Methods("GET")
	router.HandleFunc("/api/v1/health", healthHandler).Methods("GET")
	router.HandleFunc("/api/v1/auth/register", registerHandler).Methods("POST")
	router.HandleFunc("/api/v1/auth/login", loginHandler).Methods("POST")

	apiLogout := authMiddleware(http.HandlerFunc(logoutHandler))
	router.Handle("/api/v1/auth/logout", apiLogout).Methods("POST")

	apiProfile := authMiddleware(http.HandlerFunc(profileHandler))
	router.Handle("/api/v1/profile", apiProfile).Methods("GET")

	// API: workspaces
	// Broken into variables to prevent terminal line-wrapping syntax errors
	createWsHandler := requirePlatformRole(RoleAdmin, RoleArchitect)(http.HandlerFunc(createWorkspaceHandler))
	router.Handle("/api/v1/workspaces", authMiddleware(createWsHandler)).Methods("POST")

	router.Handle("/api/v1/workspaces", authMiddleware(http.HandlerFunc(listWorkspacesHandler))).Methods("GET")
	router.Handle("/api/v1/workspaces/{id}", authMiddleware(http.HandlerFunc(getWorkspaceHandler))).Methods("GET")
	router.Handle("/api/v1/workspaces/{id}", authMiddleware(http.HandlerFunc(updateWorkspaceHandler))).Methods("PUT")
	router.Handle("/api/v1/workspaces/{id}", authMiddleware(http.HandlerFunc(deleteWorkspaceHandler))).Methods("DELETE")
	router.Handle("/api/v1/workspaces/{id}/members", authMiddleware(http.HandlerFunc(listMembersHandler))).Methods("GET")
	router.Handle("/api/v1/workspaces/{id}/members", authMiddleware(http.HandlerFunc(addMemberHandler))).Methods("POST")

	rmMemberHandler := authMiddleware(http.HandlerFunc(removeMemberHandler))
	router.Handle("/api/v1/workspaces/{id}/members/{userID}", rmMemberHandler).Methods("DELETE")

	// API: admin
	adminListUsers := authMiddleware(requireAdmin(http.HandlerFunc(listUsersHandler)))
	router.Handle("/api/v1/admin/users", adminListUsers).Methods("GET")

	adminSetRole := authMiddleware(requireAdmin(http.HandlerFunc(setUserRoleHandler)))
	router.Handle("/api/v1/admin/users/{id}/role", adminSetRole).Methods("PUT")

	// Real-time collaboration channel
	authSSE := authMiddleware(http.HandlerFunc(sseHandler))
	router.Handle("/_/sse", authSSE).Methods("GET")

	fmt.Printf("Taawun server starting on port %s\n", port)
	fmt.Printf("Default admin: admin@taawun.com / admin123\n")

	// Wrap router with security and CSRF middleware
	finalRouter := securityHeaders(csrfMiddleware(router))
	log.Fatal(http.ListenAndServe(":"+port, finalRouter))
}
