package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	cors "github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	"taawun/pkg/conductor"
	"taawun/pkg/database"
	"taawun/pkg/ethics"
	"taawun/pkg/handlers"
	"taawun/pkg/mcp"
	"taawun/pkg/primitives"
	"taawun/pkg/repositories"
	"taawun/pkg/services"
	"taawun/web"
)

func main() {
	// Initialize database
	db, err := database.InitDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize repositories
	userRepo := repositories.NewUserRepository(db)
	workspaceRepo := repositories.NewWorkspaceRepository(db)
	notificationRepo := repositories.NewNotificationRepository(db)

	// Initialize services
	userService := services.NewUserService(userRepo)
	authService := services.NewAuthService(userRepo)
	workspaceService := services.NewWorkspaceService(workspaceRepo, userRepo)
	notificationService := services.NewNotificationService(notificationRepo, userRepo)
	adminService := services.NewAdminService(userRepo, workspaceRepo, notificationRepo)

	// Initialize handlers
	userHandler := handlers.NewUserHandler(userService)
	authHandler := handlers.NewAuthHandler(authService, userService)
	workspaceHandler := handlers.NewWorkspaceHandler(workspaceService)
	notificationHandler := handlers.NewNotificationHandler(notificationService)
	adminHandler := handlers.NewAdminHandler(adminService)
	dashboardHandler := handlers.NewDashboardHandler(userService, workspaceService, notificationService)

	// Initialize Taawun Engine Primitives & Conductor Track
	p2pHub := primitives.NewP2PRelayHub()
	ltapService := primitives.NewLTAPStorageService("./data")
	mcpServer := mcp.NewMCPServer(p2pHub)
	ethicsEngine := ethics.NewHaramCheckEngine()
	conductorTrack := conductor.NewConductorTrack(p2pHub, ltapService)

	// Setup router
	r := mux.NewRouter()

	// Middleware
	r.Use(cors.CORS(
		cors.AllowedOrigins([]string{"*"}),
		cors.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
		cors.AllowedHeaders([]string{"Content-Type", "Authorization"}),
	))

	// Public Health & Info route
	r.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "ok",
			"engine":    "Taawun IaC Control Plane & Conductor Track",
			"version":   "1.0.0",
			"mcp":       "enabled",
			"p2p":       "enabled",
			"conductor": "active",
		})
	}).Methods("GET")

	// Public Auth routes
	r.HandleFunc("/api/register", authHandler.Register).Methods("POST")
	r.HandleFunc("/api/login", authHandler.Login).Methods("POST")

	// MCP JSON-RPC 2.0 endpoint (Vibecoding Agent interface)
	r.HandleFunc("/api/mcp", mcpServer.HandleRPC).Methods("POST")

	// P2P WebRTC Signaling & WebSocket Relay endpoint (Local-first browser artifacts)
	r.HandleFunc("/api/p2p/stream", p2pHub.HandleP2PStream)

	// Taqwa Compliance Audit endpoint
	r.HandleFunc("/api/ethics/audit", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Prompt string `json:"prompt"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		result, err := ethicsEngine.AuditPrompt(req.Prompt)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}).Methods("POST")

	// Conductor Track Endpoints
	r.HandleFunc("/api/conductor/track", func(w http.ResponseWriter, r *http.Request) {
		var req conductor.TrackRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid track request body", http.StatusBadRequest)
			return
		}
		res, err := conductorTrack.ExecuteTrack(r.Context(), &req)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(res)
			return
		}
		json.NewEncoder(w).Encode(res)
	}).Methods("POST")

	r.HandleFunc("/api/conductor/track/{id}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		trackID := vars["id"]
		res, ok := conductorTrack.GetTrackResult(trackID)
		w.Header().Set("Content-Type", "application/json")
		if !ok {
			http.Error(w, "Track execution record not found", http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(res)
	}).Methods("GET")

	// Conductor Open Platform Specification & Developer Ergonomics Endpoint
	r.HandleFunc("/api/conductor/spec", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"openapi": "3.0.0",
			"info": map[string]string{
				"title":       "Taawun Platform & Conductor Track API",
				"version":     "1.0.0",
				"description": "Orchestration Control Plane for AI Vibecoding, Local-First P2P Relays, Appwrite-like Primitives, and Taqwa Ethics Guardrails.",
			},
			"endpoints": []map[string]string{
				{"path": "/api/health", "method": "GET", "desc": "Platform health & feature status"},
				{"path": "/api/mcp", "method": "POST", "desc": "Model Context Protocol JSON-RPC 2.0 interface"},
				{"path": "/api/p2p/stream", "method": "GET (WebSocket)", "desc": "WebRTC signaling & Web P2P relay hub"},
				{"path": "/api/ethics/audit", "method": "POST", "desc": "Taqwa ethics & Anti-Gharar audit engine"},
				{"path": "/api/conductor/track", "method": "POST", "desc": "Execute full end-to-end conductor track"},
				{"path": "/api/conductor/track/{id}", "method": "GET", "desc": "Retrieve conductor track step trajectory"},
				{"path": "/api/conductor/spec", "method": "GET", "desc": "Platform specification & ergonomics doc"},
			},
		})
	}).Methods("GET")

	// Protected routes
	api := r.PathPrefix("/api").Subrouter()
	api.Use(authHandler.AuthMiddleware)

	// User routes
	api.HandleFunc("/users", userHandler.GetUsers).Methods("GET")
	api.HandleFunc("/users/{id}", userHandler.GetUser).Methods("GET")
	api.HandleFunc("/users/{id}", userHandler.UpdateUser).Methods("PUT")
	api.HandleFunc("/users/{id}", userHandler.DeleteUser).Methods("DELETE")
	api.HandleFunc("/profile", userHandler.GetProfile).Methods("GET")

	// Workspace routes
	api.HandleFunc("/workspaces", workspaceHandler.CreateWorkspace).Methods("POST")
	api.HandleFunc("/workspaces", workspaceHandler.GetWorkspaces).Methods("GET")
	api.HandleFunc("/workspaces/{id}", workspaceHandler.GetWorkspace).Methods("GET")
	api.HandleFunc("/workspaces/{id}", workspaceHandler.UpdateWorkspace).Methods("PUT")
	api.HandleFunc("/workspaces/{id}", workspaceHandler.DeleteWorkspace).Methods("DELETE")
	api.HandleFunc("/workspaces/{id}/users", workspaceHandler.AddUserToWorkspace).Methods("POST")
	api.HandleFunc("/workspaces/{id}/users/{user_id}", workspaceHandler.RemoveUserFromWorkspace).Methods("DELETE")
	api.HandleFunc("/workspaces/{id}/initialize", workspaceHandler.InitializeWorkspace).Methods("POST")

	// Notification routes
	api.HandleFunc("/notifications", notificationHandler.GetNotifications).Methods("GET")
	api.HandleFunc("/notifications/{id}", notificationHandler.MarkAsRead).Methods("PUT")
	api.HandleFunc("/notifications/read-all", notificationHandler.MarkAllAsRead).Methods("PUT")

	// Admin routes
	api.HandleFunc("/admin/users", adminHandler.GetAllUsers).Methods("GET")
	api.HandleFunc("/admin/users/{id}/role", adminHandler.UpdateUserRole).Methods("PUT")
	api.HandleFunc("/admin/users/{id}/status", adminHandler.UpdateUserStatus).Methods("PUT")
	api.HandleFunc("/admin/workspaces", adminHandler.GetAllWorkspaces).Methods("GET")
	api.HandleFunc("/admin/workspaces/{id}", adminHandler.DeleteWorkspace).Methods("DELETE")
	api.HandleFunc("/admin/statistics", adminHandler.GetStatistics).Methods("GET")

	// Dashboard routes
	api.HandleFunc("/dashboard/stats", dashboardHandler.GetDashboardStats).Methods("GET")
	api.HandleFunc("/dashboard/recent", dashboardHandler.GetRecentActivity).Methods("GET")

	// Web UI (Embedded directly into executable via web package)
	r.PathPrefix("/").Handler(http.FileServer(http.FS(web.FS)))

	// Start server
	srv := &http.Server{
		Handler:      r,
		Addr:         ":8080",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	// Graceful shutdown
	go func() {
		log.Println("Taawun IaC & Conductor Track Server starting on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down Taawun server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Taawun server exited")
}