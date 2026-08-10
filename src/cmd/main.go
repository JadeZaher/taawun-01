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

"taawun/pkg/database"
"taawun/pkg/handlers"
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

// Setup router
r := mux.NewRouter()

// Middleware
r.Use(cors.CORS(
cors.AllowedOrigins([]string{"*"}),
cors.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
cors.AllowedHeaders([]string{"Content-Type", "Authorization"}),
))

// Public routes
r.HandleFunc("/api/register", authHandler.Register).Methods("POST")
r.HandleFunc("/api/login", authHandler.Login).Methods("POST")
r.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
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

// Web UI (Embedded directly into the executable via the web package)
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
log.Println("Taawun server starting on :8080")
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