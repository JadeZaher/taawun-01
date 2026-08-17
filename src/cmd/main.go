package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	cors "github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	"taawun/pkg/artifacts"
	"taawun/pkg/conductor"
	"taawun/pkg/database"
	"taawun/pkg/domains"
	"taawun/pkg/ethics"
	"taawun/pkg/handlers"
	"taawun/pkg/mcp"
	"taawun/pkg/oauth"
	"taawun/pkg/primitives"
	"taawun/pkg/repositories"
	"taawun/pkg/services"
	"taawun/web"
)

func main() {
	port, err := configuredPort()
	if err != nil {
		log.Fatalf("Invalid server configuration: %v", err)
	}
	appOrigins, err := configuredAppOrigins(port)
	if err != nil {
		log.Fatalf("Invalid CORS configuration: %v", err)
	}
	mcpHosts, err := configuredMCPHosts(appOrigins)
	if err != nil {
		log.Fatalf("Invalid MCP host configuration: %v", err)
	}
	jwtSecret, err := configuredJWTSecret()
	if err != nil {
		log.Fatalf("Invalid authentication configuration: %v", err)
	}
	publicBaseURL, err := configuredPublicBaseURL(port, appOrigins)
	if err != nil {
		log.Fatalf("Invalid public URL configuration: %v", err)
	}

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
	authService, err := services.NewAuthService(userRepo, jwtSecret)
	if err != nil {
		log.Fatalf("Failed to configure authentication: %v", err)
	}
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
	p2pHub, err := configuredRelayHub()
	if err != nil {
		log.Fatalf("Failed to configure P2P relay: %v", err)
	}
	ltapService := primitives.NewLTAPStorageService("./data")
	artifactBuilder, err := configuredArtifactBuilder()
	if err != nil {
		log.Fatalf("Failed to configure signed artifact builder: %v", err)
	}
	domainService, err := domains.NewService(db, workspaceService, domains.Options{Artifacts: artifactBuilder, PreviewOrigins: appOrigins})
	if err != nil {
		log.Fatalf("Failed to configure verified domains: %v", err)
	}
	domainHandler, err := domains.NewHTTPHandler(domainService, handlers.CurrentUser)
	if err != nil {
		log.Fatalf("Failed to configure verified-domain HTTP API: %v", err)
	}
	oauthRepository, err := oauth.NewRepository(db)
	if err != nil {
		log.Fatalf("Failed to initialize OAuth storage: %v", err)
	}
	oauthService, err := oauth.NewService(oauthRepository, authService, userRepo, workspaceService, handlers.WithCurrentUser, oauth.Config{
		Issuer: publicBaseURL, Resource: publicBaseURL + "/mcp",
	})
	if err != nil {
		log.Fatalf("Failed to configure OAuth authorization server: %v", err)
	}
	oauthHandler := oauth.NewHTTPHandler(oauthService)
	mcpServer, err := mcp.NewMCPServer(artifactBuilder, workspaceService, handlers.CurrentUser, mcp.ServerOptions{OriginAuthorizer: domainService, AuthorizationBoundary: oauthService})
	if err != nil {
		log.Fatalf("Failed to configure MCP control plane: %v", err)
	}
	ethicsEngine := ethics.NewHaramCheckEngine()
	conductorTrack := conductor.NewConductorTrack(p2pHub, ltapService)

	// Setup router
	r := mux.NewRouter()

	// CORS wraps the router so preflight requests are handled before method matching.
	corsHandler := cors.CORS(
		cors.AllowedOrigins(appOrigins),
		cors.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
		cors.AllowedHeaders([]string{"Content-Type", "Authorization"}),
		cors.MaxAge(600),
	)(r)

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

	// OAuth discovery, public-client registration, and browser consent endpoints.
	r.HandleFunc("/.well-known/oauth-protected-resource", oauthHandler.ProtectedResourceMetadata).Methods("GET")
	r.HandleFunc("/.well-known/oauth-protected-resource/mcp", oauthHandler.ProtectedResourceMetadata).Methods("GET")
	r.HandleFunc("/.well-known/oauth-authorization-server", oauthHandler.AuthorizationServerMetadata).Methods("GET")
	r.HandleFunc("/oauth/register", oauthHandler.Register).Methods("POST")
	r.HandleFunc("/oauth/authorize", oauthHandler.Authorize).Methods("GET")
	r.HandleFunc("/oauth/login", oauthHandler.Login).Methods("POST")
	r.HandleFunc("/oauth/consent", oauthHandler.Consent).Methods("POST")
	r.HandleFunc("/oauth/token", oauthHandler.Token).Methods("POST")
	r.HandleFunc("/oauth/revoke", oauthHandler.Revoke).Methods("POST")

	adminOnly := func(handler http.Handler) http.Handler {
		return authHandler.AuthMiddleware(authHandler.RequireAdmin(handler))
	}

	// Remote MCP accepts standard Streamable HTTP methods and re-authorizes every workspace tool.
	r.Handle("/mcp", mcp.ProtectExactHTTP(appOrigins, mcpHosts, oauthService.Middleware(oauthService.MCPToolScopeMiddleware(mcp.OAuthScopeRequirements(), mcpServer.Handler()))))

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
	r.Handle("/api/conductor/track", adminOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	}))).Methods("POST")

	r.Handle("/api/conductor/track/{id}", adminOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		trackID := vars["id"]
		res, ok := conductorTrack.GetTrackResult(trackID)
		w.Header().Set("Content-Type", "application/json")
		if !ok {
			http.Error(w, "Track execution record not found", http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(res)
	}))).Methods("GET")

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
				{"path": "/mcp", "method": "POST/GET/DELETE", "desc": "Authenticated MCP Streamable HTTP control plane with workspace-scoped tools"},
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
	api.HandleFunc("/workspaces/{id}/domains", domainHandler.Claim).Methods("POST")
	api.HandleFunc("/workspaces/{id}/domains", domainHandler.List).Methods("GET")
	api.HandleFunc("/workspaces/{id}/domains/{claim_id}", domainHandler.Inspect).Methods("GET")
	api.HandleFunc("/workspaces/{id}/domains/{claim_id}", domainHandler.Revoke).Methods("DELETE")
	api.HandleFunc("/workspaces/{id}/domains/{claim_id}/verify", domainHandler.Verify).Methods("POST")
	api.HandleFunc("/workspaces/{id}/domains/{claim_id}/publications", domainHandler.Publish).Methods("POST")
	api.HandleFunc("/workspaces/{id}/domains/{claim_id}/publications", domainHandler.PublicationHistory).Methods("GET")
	api.HandleFunc("/workspaces/{id}/domains/{claim_id}/publications/{publication_id}/activate", domainHandler.Activate).Methods("POST")

	// Notification routes
	api.HandleFunc("/notifications", notificationHandler.GetNotifications).Methods("GET")
	api.HandleFunc("/notifications/{id}", notificationHandler.MarkAsRead).Methods("PUT")
	api.HandleFunc("/notifications/read-all", notificationHandler.MarkAllAsRead).Methods("PUT")

	// Admin routes
	adminAPI := api.PathPrefix("/admin").Subrouter()
	adminAPI.Use(authHandler.RequireAdmin)
	adminAPI.HandleFunc("/users", adminHandler.GetAllUsers).Methods("GET")
	adminAPI.HandleFunc("/users/{id}/role", adminHandler.UpdateUserRole).Methods("PUT")
	adminAPI.HandleFunc("/users/{id}/status", adminHandler.UpdateUserStatus).Methods("PUT")
	adminAPI.HandleFunc("/workspaces", adminHandler.GetAllWorkspaces).Methods("GET")
	adminAPI.HandleFunc("/workspaces/{id}", adminHandler.DeleteWorkspace).Methods("DELETE")
	adminAPI.HandleFunc("/statistics", adminHandler.GetStatistics).Methods("GET")

	// Dashboard routes
	api.HandleFunc("/dashboard/stats", dashboardHandler.GetDashboardStats).Methods("GET")
	api.HandleFunc("/dashboard/recent", dashboardHandler.GetRecentActivity).Methods("GET")

	// Public cards resolve by Host; only configured control hosts fall back to the builder UI.
	publicCards, err := domains.NewPublicHandler(domainService, appOrigins, http.FileServer(http.FS(web.FS)))
	if err != nil {
		log.Fatalf("Failed to configure public signed-card delivery: %v", err)
	}
	r.PathPrefix("/").Handler(publicCards)

	// Start server
	srv := &http.Server{
		Handler:           withBoundedWriteDeadline(corsHandler, 30*time.Second, "/mcp", "/api/p2p/stream"),
		Addr:              ":" + port,
		WriteTimeout:      0,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	// Graceful shutdown
	go func() {
		log.Printf("Taawun IaC & Conductor Track Server starting on :%s", port)
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

func configuredRelayHub() (*primitives.P2PRelayHub, error) {
	secret := []byte(os.Getenv("RELAY_SHARED_SECRET"))
	if len(secret) < 32 {
		return nil, fmt.Errorf("RELAY_SHARED_SECRET must contain at least 32 bytes")
	}
	origins := splitCSV(os.Getenv("RELAY_ALLOWED_ORIGINS"))
	if len(origins) == 0 {
		return nil, fmt.Errorf("RELAY_ALLOWED_ORIGINS must list at least one browser origin")
	}
	return primitives.NewP2PRelayHubWithConfig(primitives.RelayConfig{SharedSecret: secret, AllowedOrigins: origins})
}

func configuredJWTSecret() ([]byte, error) {
	secret := []byte(os.Getenv("TAWUN_JWT_SECRET"))
	if len(secret) < 32 {
		return nil, fmt.Errorf("TAWUN_JWT_SECRET must contain at least 32 bytes")
	}
	return secret, nil
}

func configuredPort() (string, error) {
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		return "8080", nil
	}
	value, err := strconv.Atoi(port)
	if err != nil || value < 1 || value > 65535 {
		return "", fmt.Errorf("PORT must be an integer from 1 through 65535")
	}
	return strconv.Itoa(value), nil
}

func configuredAppOrigins(port string) ([]string, error) {
	origins := splitCSV(os.Getenv("TAWUN_ALLOWED_ORIGINS"))
	if len(origins) == 0 {
		return []string{"http://localhost:" + port}, nil
	}
	for _, origin := range origins {
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "" {
			return nil, fmt.Errorf("TAWUN_ALLOWED_ORIGINS contains invalid origin %q", origin)
		}
		if parsed.Scheme != "https" && !(parsed.Scheme == "http" && isLoopbackHost(parsed.Hostname())) {
			return nil, fmt.Errorf("TAWUN_ALLOWED_ORIGINS requires HTTPS except for loopback origins: %q", origin)
		}
	}
	return origins, nil
}

func configuredPublicBaseURL(port string, appOrigins []string) (string, error) {
	value := strings.TrimRight(strings.TrimSpace(os.Getenv("TAWUN_PUBLIC_BASE_URL")), "/")
	if value == "" {
		if railwayDomain := strings.TrimSpace(os.Getenv("RAILWAY_PUBLIC_DOMAIN")); railwayDomain != "" {
			value = "https://" + railwayDomain
		} else if len(appOrigins) > 0 {
			value = strings.TrimRight(appOrigins[0], "/")
		} else {
			value = "http://localhost:" + port
		}
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("TAWUN_PUBLIC_BASE_URL must be one origin without a path")
	}
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && isLoopbackHost(parsed.Hostname())) {
		return "", fmt.Errorf("TAWUN_PUBLIC_BASE_URL requires HTTPS except on loopback")
	}
	return value, nil
}

func configuredMCPHosts(appOrigins []string) ([]string, error) {
	hosts := splitCSV(os.Getenv("TAWUN_MCP_ALLOWED_HOSTS"))
	if len(hosts) == 0 {
		if railwayDomain := strings.TrimSpace(os.Getenv("RAILWAY_PUBLIC_DOMAIN")); railwayDomain != "" {
			hosts = []string{railwayDomain}
		} else {
			seen := make(map[string]struct{}, len(appOrigins))
			for _, origin := range appOrigins {
				parsed, err := url.Parse(origin)
				if err != nil || parsed.Host == "" {
					return nil, fmt.Errorf("cannot derive MCP host from origin %q", origin)
				}
				if _, exists := seen[parsed.Host]; !exists {
					hosts = append(hosts, parsed.Host)
					seen[parsed.Host] = struct{}{}
				}
			}
		}
	}
	if len(hosts) == 0 {
		return nil, fmt.Errorf("TAWUN_MCP_ALLOWED_HOSTS must list at least one exact host")
	}
	seen := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		if host == "" || strings.Contains(host, "*") || strings.TrimSpace(host) != host {
			return nil, fmt.Errorf("TAWUN_MCP_ALLOWED_HOSTS contains invalid host %q", host)
		}
		parsed, err := url.Parse("https://" + host)
		if err != nil || parsed.Host != host || parsed.Hostname() == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return nil, fmt.Errorf("TAWUN_MCP_ALLOWED_HOSTS contains invalid host %q", host)
		}
		key := strings.ToLower(host)
		if _, duplicate := seen[key]; duplicate {
			return nil, fmt.Errorf("TAWUN_MCP_ALLOWED_HOSTS contains duplicate host %q", host)
		}
		seen[key] = struct{}{}
	}
	return hosts, nil
}

func configuredArtifactBuilder() (*artifacts.Builder, error) {
	root := strings.TrimSpace(os.Getenv("TAWUN_ARTIFACT_ROOT"))
	if root == "" {
		root = "./data/artifacts"
	}
	keyID := strings.TrimSpace(os.Getenv("TAWUN_ARTIFACT_SIGNING_KEY_ID"))
	if keyID == "" {
		return nil, fmt.Errorf("TAWUN_ARTIFACT_SIGNING_KEY_ID is required")
	}
	encodedKey := strings.TrimSpace(os.Getenv("TAWUN_ARTIFACT_SIGNING_PRIVATE_KEY"))
	if encodedKey == "" {
		return nil, fmt.Errorf("TAWUN_ARTIFACT_SIGNING_PRIVATE_KEY is required")
	}
	privateKey, err := decodeEd25519PrivateKey(encodedKey)
	if err != nil {
		return nil, err
	}
	publicKey, ok := privateKey.Public().(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("artifact signing key has no Ed25519 public key")
	}
	return artifacts.NewSignedBuilder(root, artifacts.SigningConfig{
		KeyID:       keyID,
		PrivateKey:  privateKey,
		TrustedKeys: map[string]ed25519.PublicKey{keyID: publicKey},
	})
}

func decodeEd25519PrivateKey(value string) (ed25519.PrivateKey, error) {
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	for _, encoding := range encodings {
		decoded, err := encoding.DecodeString(value)
		if err == nil && len(decoded) == ed25519.PrivateKeySize {
			return ed25519.PrivateKey(append([]byte(nil), decoded...)), nil
		}
	}
	return nil, fmt.Errorf("TAWUN_ARTIFACT_SIGNING_PRIVATE_KEY must be a base64-encoded %d-byte Ed25519 private key", ed25519.PrivateKeySize)
}

func withBoundedWriteDeadline(next http.Handler, timeout time.Duration, unboundedPaths ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, path := range unboundedPaths {
			if r.URL.Path == path || strings.HasPrefix(r.URL.Path, path+"/") {
				next.ServeHTTP(w, r)
				return
			}
		}
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(timeout))
		next.ServeHTTP(w, r)
	})
}

func isLoopbackHost(host string) bool {
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

func splitCSV(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}
