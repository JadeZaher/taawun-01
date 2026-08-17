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
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	cors "github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	"taawun/pkg/artifacts"
	"taawun/pkg/bazaar"
	"taawun/pkg/conductor"
	"taawun/pkg/database"
	"taawun/pkg/domains"
	"taawun/pkg/ethics"
	"taawun/pkg/financial"
	"taawun/pkg/handlers"
	"taawun/pkg/mcp"
	"taawun/pkg/models"
	"taawun/pkg/oauth"
	"taawun/pkg/primitives"
	"taawun/pkg/repositories"
	"taawun/pkg/services"
	"taawun/pkg/shura"
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

	// Initialize Taawun engine primitives and the authenticated composition services.
	p2pHub, err := configuredRelayHub()
	if err != nil {
		log.Fatalf("Failed to configure P2P relay: %v", err)
	}
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
	ethicsEngine := ethics.NewHaramCheckEngine()
	complianceCorpus := ethics.NewSeedComplianceCorpus()
	conductorRepository, err := conductor.NewRepository(db)
	if err != nil {
		log.Fatalf("Failed to initialize Conductor storage: %v", err)
	}
	complianceAuditor, err := conductor.NewReferenceComplianceAuditor(ethicsEngine, complianceCorpus)
	if err != nil {
		log.Fatalf("Failed to configure Conductor compliance audit: %v", err)
	}
	conductorService, err := conductor.NewService(conductorRepository, workspaceService, conductor.ActorSubjectResolver{}, conductor.CuratedCompositionValidator{}, complianceAuditor, artifactBuilder, domainService, domainService)
	if err != nil {
		log.Fatalf("Failed to configure Conductor composition service: %v", err)
	}
	compositionHandler, err := handlers.NewCompositionHTTPHandler(conductorService, artifactBuilder, handlers.CurrentUser, appOrigins)
	if err != nil {
		log.Fatalf("Failed to configure central builder API: %v", err)
	}
	relayTicketHandler, err := handlers.NewRelayTicketHTTPHandler(p2pHub, artifactBuilder, workspaceService, handlers.CurrentUser)
	if err != nil {
		log.Fatalf("Failed to configure relay ticket API: %v", err)
	}
	shuraRepository, err := shura.NewRepository(db)
	if err != nil {
		log.Fatalf("Failed to initialize Shura storage: %v", err)
	}
	shuraIssuer, shuraRegistry, err := configuredShuraSigning(publicBaseURL)
	if err != nil {
		log.Fatalf("Failed to configure Shura signing: %v", err)
	}
	shuraVerifier, err := shura.NewCapabilityVerifier(shuraRegistry, shuraRepository, workspaceService)
	if err != nil {
		log.Fatalf("Failed to configure Shura verification: %v", err)
	}
	shuraService, err := shura.NewServiceWithMembership(shuraRepository, shuraIssuer, shuraVerifier, workspaceService)
	if err != nil {
		log.Fatalf("Failed to configure Shura service: %v", err)
	}
	shuraHandler, err := shura.NewHTTPHandler(shuraService, func(request *http.Request) (*models.User, error) {
		user, ok := handlers.CurrentUser(request.Context())
		if !ok {
			return nil, fmt.Errorf("authentication required")
		}
		return user, nil
	})
	if err != nil {
		log.Fatalf("Failed to configure Shura HTTP API: %v", err)
	}
	financialPath := configuredFinancialDatabasePath()
	financialService, err := financial.NewSQLiteAzoaSandbox(financialPath)
	if err != nil {
		log.Fatalf("Failed to configure AZOA sandbox: %v", err)
	}
	defer financialService.Close()
	financialHandler, err := handlers.NewFinancialHTTPHandler(financialService, workspaceService, shuraService, handlers.CurrentUser)
	if err != nil {
		log.Fatalf("Failed to configure financial HTTP API: %v", err)
	}
	bazaarService, err := bazaar.NewService(db, bazaar.Dependencies{
		Workspaces: workspaceService, Artifacts: artifactBuilder, Publications: domainService,
		Compliance: referenceBazaarComplianceReviewer{corpus: complianceCorpus},
		Shura:      bazaarShuraDecisionResolver{service: shuraService},
		Financial:  financialService, Gharar: ethics.NewAntiGhararValidator(),
	})
	if err != nil {
		log.Fatalf("Failed to configure Bazaar service: %v", err)
	}
	bazaarHandler, err := bazaar.NewHTTPHandler(bazaarService, handlers.CurrentUser)
	if err != nil {
		log.Fatalf("Failed to configure Bazaar HTTP API: %v", err)
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

	// Remote MCP accepts standard Streamable HTTP methods and re-authorizes every workspace tool.
	r.Handle("/mcp", mcp.ProtectExactHTTP(appOrigins, mcpHosts, oauthService.Middleware(oauthService.MCPToolScopeMiddleware(mcp.OAuthScopeRequirements(), mcpServer.Handler()))))

	// Shura uses first-party identity only for invitation and capability issuance; proposal actions use signed Shura capabilities.
	shuraAPI := http.StripPrefix("/api/shura", shuraHandler)
	r.Handle("/api/shura/.well-known/jwks.json", shuraAPI).Methods(http.MethodGet)
	r.Handle("/api/shura/v1/capabilities", authHandler.AuthMiddleware(shuraAPI)).Methods(http.MethodPost)
	r.Handle("/api/shura/v1/capabilities/revoke", authHandler.AuthMiddleware(shuraAPI)).Methods(http.MethodPost)
	r.Handle("/api/shura/v1/invitations", authHandler.AuthMiddleware(shuraAPI)).Methods(http.MethodPost)
	r.Handle("/api/shura/v1/invitations/accept", authHandler.AuthMiddleware(shuraAPI)).Methods(http.MethodPost)
	r.Handle("/api/shura/v1/invitations/{invitation_id}/revoke", authHandler.AuthMiddleware(shuraAPI)).Methods(http.MethodPost)
	r.Handle("/api/shura/v1/workspaces/{workspace_id}/proposals", shuraAPI).Methods(http.MethodPost)
	r.Handle("/api/shura/v1/proposals/{proposal_id}", shuraAPI).Methods(http.MethodGet)
	r.Handle("/api/shura/v1/proposals/{proposal_id}/deliberation", shuraAPI).Methods(http.MethodPost)
	r.Handle("/api/shura/v1/proposals/{proposal_id}/votes", shuraAPI).Methods(http.MethodPost)
	r.Handle("/api/shura/v1/proposals/{proposal_id}/decision", shuraAPI).Methods(http.MethodPost)
	r.Handle("/api/shura/v1/proposals/{proposal_id}/cancel", shuraAPI).Methods(http.MethodPost)
	r.Handle("/api/shura/v1/proposals/{proposal_id}/audit", shuraAPI).Methods(http.MethodGet)

	if err := bazaar.RegisterRoutes(r, authHandler.AuthMiddleware, bazaarHandler); err != nil {
		log.Fatalf("Failed to mount Bazaar HTTP API: %v", err)
	}

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
				{"path": "/api/artifacts/preview", "method": "POST", "desc": "Create an authenticated signed staging preview"},
				{"path": "/api/conductor/tracks/{track_id}", "method": "GET", "desc": "Inspect a workspace-authorized composition track"},
				{"path": "/api/conductor/tracks/{track_id}/publication", "method": "POST", "desc": "Request a verified-domain publication"},
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
	api.HandleFunc("/workspaces/{workspace_id}/relay-sessions", relayTicketHandler.Issue).Methods(http.MethodPost)

	// Central builder routes derive the principal from authentication and workspace authority from Conductor.
	api.HandleFunc("/templates", compositionHandler.Templates).Methods(http.MethodGet)
	api.HandleFunc("/modules", compositionHandler.Modules).Methods(http.MethodGet)
	api.HandleFunc("/artifacts/preview", compositionHandler.Preview).Methods(http.MethodPost)
	api.HandleFunc("/conductor/tracks/{track_id}", compositionHandler.GetTrack).Methods(http.MethodGet)
	api.HandleFunc("/conductor/tracks/{track_id}/events", compositionHandler.Events).Methods(http.MethodGet)
	api.HandleFunc("/conductor/tracks/{track_id}/resume", compositionHandler.Resume).Methods(http.MethodPost)
	api.HandleFunc("/conductor/tracks/{track_id}/publication", compositionHandler.RequestPublication).Methods(http.MethodPost)
	api.HandleFunc("/conductor/tracks/{track_id}/activate", compositionHandler.ActivatePublication).Methods(http.MethodPost)
	api.HandleFunc("/conductor/tracks/{track_id}/preview/files/{path:.*}", compositionHandler.PreviewFile).Methods(http.MethodGet, http.MethodHead)
	api.HandleFunc("/financial/flows", financialHandler.Flows).Methods(http.MethodGet)
	api.HandleFunc("/financial/quests", financialHandler.CreateQuest).Methods(http.MethodPost)
	api.HandleFunc("/financial/quests/{quest_id}", financialHandler.GetQuest).Methods(http.MethodGet)
	api.HandleFunc("/financial/quests/{quest_id}/events", financialHandler.Events).Methods(http.MethodGet)
	api.HandleFunc("/financial/quests/{quest_id}/approve", financialHandler.Approve).Methods(http.MethodPost)
	api.HandleFunc("/financial/quests/{quest_id}/execute", financialHandler.Execute).Methods(http.MethodPost)
	api.HandleFunc("/financial/quests/{quest_id}/reconcile", financialHandler.Reconcile).Methods(http.MethodPost)
	api.HandleFunc("/financial/quests/{quest_id}/cancel", financialHandler.Cancel).Methods(http.MethodPost)

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

func configuredShuraSigning(publicBaseURL string) (*shura.CapabilityIssuer, *shura.StaticKeyRegistry, error) {
	issuerURL := strings.TrimSpace(os.Getenv("TAWUN_SHURA_ISSUER"))
	if issuerURL == "" {
		parsed, err := url.Parse(publicBaseURL)
		if err == nil && parsed.Scheme == "https" {
			issuerURL = publicBaseURL
		} else {
			issuerURL = "https://taawun.local"
		}
	}
	keyID := strings.TrimSpace(os.Getenv("TAWUN_SHURA_SIGNING_KEY_ID"))
	if keyID == "" {
		keyID = strings.TrimSpace(os.Getenv("TAWUN_ARTIFACT_SIGNING_KEY_ID"))
	}
	encodedKey := strings.TrimSpace(os.Getenv("TAWUN_SHURA_SIGNING_PRIVATE_KEY"))
	if encodedKey == "" {
		encodedKey = strings.TrimSpace(os.Getenv("TAWUN_ARTIFACT_SIGNING_PRIVATE_KEY"))
	}
	if keyID == "" || encodedKey == "" {
		return nil, nil, fmt.Errorf("TAWUN_SHURA_SIGNING_KEY_ID and TAWUN_SHURA_SIGNING_PRIVATE_KEY are required (artifact signing values may supply the development fallback)")
	}
	privateKey, err := decodeEd25519PrivateKey(encodedKey)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid Shura signing key: %w", err)
	}
	issuer, err := shura.NewCapabilityIssuer(issuerURL, keyID, privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("configure Shura issuer: %w", err)
	}
	registry, err := shura.NewStaticKeyRegistry(issuerURL, keyID, issuer.PublicKey())
	if err != nil {
		return nil, nil, fmt.Errorf("configure Shura key registry: %w", err)
	}
	return issuer, registry, nil
}

func configuredFinancialDatabasePath() string {
	if value := strings.TrimSpace(os.Getenv("TAWUN_FINANCIAL_DB_PATH")); value != "" {
		return value
	}
	if databasePath := strings.TrimSpace(os.Getenv("APP_DB_PATH")); databasePath != "" {
		return filepath.Join(filepath.Dir(databasePath), "taawun-azoa.db")
	}
	return filepath.Join("data", "taawun-azoa.db")
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
