package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"taawun/pkg/artifacts"
	"taawun/pkg/models"
)

type testUserContextKey struct{}

type fakeArtifactStore struct {
	mu          sync.Mutex
	lastBuild   artifacts.BuildRequest
	buildCalls  int
	openResult  artifacts.BuildResult
	file        artifacts.ArtifactFile
	buildErr    error
	openErr     error
	readFileErr error
}

func (s *fakeArtifactStore) Build(_ context.Context, request artifacts.BuildRequest) (artifacts.BuildResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastBuild = request
	s.buildCalls++
	if s.buildErr != nil {
		return artifacts.BuildResult{}, s.buildErr
	}
	result := s.openResult
	result.Manifest.WorkspaceID = request.WorkspaceID
	result.Manifest.Authorization.Subject = artifacts.BundleSubject{
		ID: request.Subject.ID, UserID: request.Subject.UserID, WorkspaceID: request.WorkspaceID,
	}
	result.Manifest.Authorization.ExpiresAt = request.ExpiresAt
	return result, nil
}

func (s *fakeArtifactStore) Open(context.Context, string) (artifacts.BuildResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.openErr != nil {
		return artifacts.BuildResult{}, s.openErr
	}
	return s.openResult, nil
}

func (s *fakeArtifactStore) ReadFile(context.Context, string, string) (artifacts.ArtifactFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.readFileErr != nil {
		return artifacts.ArtifactFile{}, s.readFileErr
	}
	return s.file, nil
}

type fakeWorkspaceAuthorizer struct {
	workspace  *models.Workspace
	denied     bool
	capability models.WorkspaceCapability
}

type previewLifecycleAuthorizer struct {
	lifecycle artifacts.BundleLifecycle
}

func (a *previewLifecycleAuthorizer) AuthorizeOrigins(_ context.Context, _ *models.User, _ *models.Workspace, _ artifacts.OriginPolicy) (artifacts.OriginPolicy, error) {
	return artifacts.OriginPolicy{}, errors.New("published origin path must not authorize preview composition")
}

func (a *previewLifecycleAuthorizer) AuthorizeOriginsForLifecycle(_ context.Context, _ *models.User, _ *models.Workspace, lifecycle artifacts.BundleLifecycle, requested artifacts.OriginPolicy) (artifacts.OriginPolicy, error) {
	a.lifecycle = lifecycle
	return requested, nil
}

func TestComposeUsesLifecycleScopedPreviewOrigins(t *testing.T) {
	hash := strings.Repeat("f", 64)
	store := &fakeArtifactStore{openResult: artifacts.BuildResult{ArtifactID: "art-preview", ContentHash: hash,
		Manifest: artifacts.Manifest{ArtifactID: "art-preview", ContentHash: hash}}}
	origins := &previewLifecycleAuthorizer{}
	server, err := NewMCPServer(store, &fakeWorkspaceAuthorizer{workspace: &models.Workspace{ID: 42}}, testCurrentUser,
		ServerOptions{OriginAuthorizer: origins, AuthorizationBoundary: allowAllAuthorizationBoundary{}})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(context.Background(), testUserContextKey{}, &models.User{ID: 7, Role: models.RoleUser})
	_, _, err = server.composeCardBundle(ctx, nil, ComposeCardBundleInput{WorkspaceID: 42, AppName: "Preview",
		OrganizationName: "Community", City: "Denver", Madhhab: "hanafi", TemplateID: artifacts.TemplateCommunityIftar,
		Modules: []string{artifacts.ModuleRegistration}, AllowedOrigins: artifacts.OriginPolicy{Surfaces: []string{"http://localhost:8080"}}, TTLHours: 1})
	if err != nil {
		t.Fatalf("composeCardBundle() error = %v", err)
	}
	if origins.lifecycle != artifacts.BundleLifecyclePreview {
		t.Fatalf("origin lifecycle = %q, want preview", origins.lifecycle)
	}
}

func TestComposeRejectsReservedComponentAuthorityBeforeBuild(t *testing.T) {
	store := &fakeArtifactStore{}
	server, err := NewMCPServer(store, &fakeWorkspaceAuthorizer{workspace: &models.Workspace{ID: 42}}, testCurrentUser,
		ServerOptions{OriginAuthorizer: &previewLifecycleAuthorizer{}, AuthorizationBoundary: allowAllAuthorizationBoundary{}})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(context.Background(), testUserContextKey{}, &models.User{ID: 7, Role: models.RoleUser})
	_, _, err = server.composeCardBundle(ctx, nil, ComposeCardBundleInput{WorkspaceID: 42, AppName: "Preview",
		OrganizationName: "Community", City: "Denver", Madhhab: "hanafi", TemplateID: artifacts.TemplateCommunityIftar,
		Modules: []string{artifacts.ModuleRegistration}, Components: []MCPComponentInput{{ID: artifacts.ModuleRegistration, Type: artifacts.ModuleRegistration,
			Data: map[string]any{"title": "Register", "summary": "Join", "workspaceId": 99}}},
		AllowedOrigins: artifacts.OriginPolicy{Surfaces: []string{"http://localhost:8080"}}, TTLHours: 1})
	var validation *artifacts.ComponentValidationError
	if !errors.As(err, &validation) || validation.Reason != "reserved_key" {
		t.Fatalf("component validation = %#v err=%v", validation, err)
	}
	if strings.Contains(err.Error(), "workspaceId") {
		t.Fatalf("MCP error reflected reserved key: %v", err)
	}
	store.mu.Lock()
	buildCalls := store.buildCalls
	store.mu.Unlock()
	if buildCalls != 0 {
		t.Fatalf("build called %d times for invalid component", buildCalls)
	}
}

func TestComposeRejectsExplicitEmptyComponentsAndUntrustedMetadata(t *testing.T) {
	store := &fakeArtifactStore{}
	server, err := NewMCPServer(store, &fakeWorkspaceAuthorizer{workspace: &models.Workspace{ID: 42}}, testCurrentUser,
		ServerOptions{OriginAuthorizer: &previewLifecycleAuthorizer{}, AuthorizationBoundary: allowAllAuthorizationBoundary{}})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(context.Background(), testUserContextKey{}, &models.User{ID: 7, Role: models.RoleUser})
	base := ComposeCardBundleInput{WorkspaceID: 42, AppName: "Preview", OrganizationName: "Community", City: "Denver", Madhhab: "hanafi",
		TemplateID: artifacts.TemplateCommunityIftar, Modules: []string{artifacts.ModuleRegistration},
		AllowedOrigins: artifacts.OriginPolicy{Surfaces: []string{"http://localhost:8080"}}, TTLHours: 1}
	base.Components = []MCPComponentInput{}
	_, _, err = server.composeCardBundle(ctx, nil, base)
	var validation *artifacts.ComponentValidationError
	if !errors.As(err, &validation) || validation.Reason != "components_required" {
		t.Fatalf("explicit empty MCP components = %#v err=%v", validation, err)
	}

	secretID := strings.Repeat("secret-token-", 40)
	base.Components = []MCPComponentInput{{ID: secretID, Type: artifacts.ModuleRegistration, Data: map[string]any{"title": "Register", "summary": "Join"}}}
	_, _, err = server.composeCardBundle(ctx, nil, base)
	if !errors.As(err, &validation) || strings.Contains(err.Error(), "secret-token") || len(err.Error()) > 256 {
		t.Fatalf("unsafe MCP component metadata = %#v err=%v", validation, err)
	}
	store.mu.Lock()
	buildCalls := store.buildCalls
	store.mu.Unlock()
	if buildCalls != 0 {
		t.Fatalf("build called %d times for rejected MCP components", buildCalls)
	}
}

func TestStreamableHTTPPreservesComponentDocumentLexemesForValidation(t *testing.T) {
	artifactID := "artifact_unicode_test"
	hash := strings.Repeat("a", 64)
	store := &fakeArtifactStore{openResult: artifacts.BuildResult{
		ArtifactID: artifactID, ContentHash: hash,
		Manifest: artifacts.Manifest{ArtifactID: artifactID, ContentHash: hash},
	}}
	server, err := NewMCPServer(store, &fakeWorkspaceAuthorizer{workspace: &models.Workspace{ID: 42, Status: models.WorkspaceStatusActive}}, testCurrentUser,
		ServerOptions{OriginAuthorizer: mustStaticOriginAuthorizer(t, "https://app.example"), AuthorizationBoundary: allowAllAuthorizationBoundary{}})
	if err != nil {
		t.Fatal(err)
	}
	session := connectTestClient(t, server.Handler(), "https://app.example")
	argumentTemplate := `{"workspaceId":42,"appName":"Preview","organizationName":"Community","city":"Denver","madhhab":"hanafi","templateId":"community-iftar","theme":{},"modules":["iftar-registration"],"components":[{"id":"iftar-registration","type":"iftar-registration","data":DOCUMENT}],"allowedOrigins":{"surfaces":["https://app.example"]},"ttlHours":1}`
	for _, test := range []struct {
		name     string
		document string
		reason   string
	}{
		{name: "duplicate key", document: `{"title":"Register","summary":"Join","note":1,"note":2}`, reason: "duplicate_key"},
		{name: "exponent", document: `{"title":"Register","summary":"Join","amount":1e0}`, reason: "non_canonical_number"},
		{name: "trailing zero", document: `{"title":"Register","summary":"Join","amount":1.0}`, reason: "non_canonical_number"},
		{name: "root high surrogate", document: `{"title":"Register","summary":"\ud800"}`, reason: "invalid_unicode_scalar"},
		{name: "nested low surrogate", document: `{"title":"Register","summary":"Join","details":{"note":"\udc00"}}`, reason: "invalid_unicode_scalar"},
	} {
		t.Run(test.name, func(t *testing.T) {
			arguments := json.RawMessage(strings.Replace(argumentTemplate, "DOCUMENT", test.document, 1))
			result, err := session.CallTool(t.Context(), &mcpsdk.CallToolParams{Name: "taawun_compose_card_bundle", Arguments: arguments})
			if err != nil {
				t.Fatalf("raw protocol call: %v", err)
			}
			if !result.IsError || !toolTextContains(result, test.reason) {
				t.Fatalf("raw document result = %#v, want %s", result, test.reason)
			}
		})
	}
	store.mu.Lock()
	buildCalls := store.buildCalls
	store.mu.Unlock()
	if buildCalls != 0 {
		t.Fatalf("artifact build called %d times for lexically invalid MCP documents", buildCalls)
	}

	arguments := json.RawMessage(strings.Replace(argumentTemplate, "DOCUMENT", `{"title":"Register","summary":"\ud83d\ude00","details":{"note":"🚀"}}`, 1))
	result, err := session.CallTool(t.Context(), &mcpsdk.CallToolParams{Name: "taawun_compose_card_bundle", Arguments: arguments})
	if err != nil || result.IsError {
		t.Fatalf("valid Unicode scalar pair result = %#v err=%v", result, err)
	}
	store.mu.Lock()
	lastBuild := store.lastBuild
	buildCalls = store.buildCalls
	store.mu.Unlock()
	if buildCalls != 1 || len(lastBuild.Components) != 1 || string(lastBuild.Components[0].Data) != `{"details":{"note":"🚀"},"summary":"😀","title":"Register"}` {
		t.Fatalf("valid Unicode MCP build = calls:%d components:%#v", buildCalls, lastBuild.Components)
	}
}

type allowAllAuthorizationBoundary struct{}

func (allowAllAuthorizationBoundary) AuthorizeScope(context.Context, string) error       { return nil }
func (allowAllAuthorizationBoundary) AuthorizeWorkspaceGrant(context.Context, int) error { return nil }

type denyScopesAuthorizationBoundary struct{}

func (denyScopesAuthorizationBoundary) AuthorizeScope(context.Context, string) error {
	return errors.New("missing OAuth scope")
}

func (denyScopesAuthorizationBoundary) AuthorizeWorkspaceGrant(context.Context, int) error {
	return nil
}

func TestEveryToolFailsClosedWithoutItsOAuthScope(t *testing.T) {
	server := &MCPServer{boundary: denyScopesAuthorizationBoundary{}}
	tests := []struct {
		name string
		call func() error
	}{
		{"list templates", func() error { _, _, err := server.listTemplates(t.Context(), nil, EmptyInput{}); return err }},
		{"list primitives", func() error { _, _, err := server.listPrimitives(t.Context(), nil, EmptyInput{}); return err }},
		{"inspect template", func() error {
			_, _, err := server.inspectTemplate(t.Context(), nil, InspectTemplateInput{})
			return err
		}},
		{"inspect primitive", func() error {
			_, _, err := server.inspectPrimitive(t.Context(), nil, InspectPrimitiveInput{})
			return err
		}},
		{"audit", func() error {
			_, _, err := server.auditCompliance(t.Context(), nil, AuditComplianceInput{})
			return err
		}},
		{"compose", func() error {
			_, _, err := server.composeCardBundle(t.Context(), nil, ComposeCardBundleInput{})
			return err
		}},
		{"manifest", func() error { _, _, err := server.getManifest(t.Context(), nil, GetManifestInput{}); return err }},
		{"card", func() error { _, _, err := server.getCard(t.Context(), nil, GetCardInput{}); return err }},
		{"stage", func() error { _, _, err := server.stageCardBundle(t.Context(), nil, StageCardInput{}); return err }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err == nil || !strings.Contains(err.Error(), "OAuth scope") {
				t.Fatalf("scope error = %v", err)
			}
		})
	}
}

func (a *fakeWorkspaceAuthorizer) AuthorizeWorkspaceCapability(_ *models.User, id int, capability models.WorkspaceCapability) (*models.Workspace, error) {
	a.capability = capability
	if a.denied || a.workspace == nil || a.workspace.ID != id {
		return nil, errors.New("forbidden")
	}
	return a.workspace, nil
}

type authTransport struct {
	base   http.RoundTripper
	token  string
	origin string
}

func (t authTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.Header.Set("Authorization", "Bearer "+t.token)
	if t.origin != "" {
		clone.Header.Set("Origin", t.origin)
	}
	return t.base.RoundTrip(clone)
}

func TestStreamableHTTPAdvertisesTypedControlPlaneAndRunsLifecycle(t *testing.T) {
	now := time.Date(2026, 8, 16, 22, 0, 0, 0, time.UTC)
	hash := strings.Repeat("a", 64)
	store := &fakeArtifactStore{
		openResult: artifacts.BuildResult{
			ArtifactID:  "art_test",
			ContentHash: hash,
			Manifest: artifacts.Manifest{
				ArtifactID:  "art_test",
				ContentHash: hash,
				WorkspaceID: 42,
				Authorization: artifacts.BundleAuthorization{
					Subject:   artifacts.BundleSubject{ID: "taawun:user:7", UserID: 7, WorkspaceID: 42},
					ExpiresAt: now.Add(24 * time.Hour),
					Lifecycle: artifacts.BundleLifecyclePreview,
				},
				Files: []artifacts.FileDigest{{Path: "standalone/index.html", SHA256: strings.Repeat("b", 64), Bytes: 18}},
			},
		},
		file: artifacts.ArtifactFile{Path: "standalone/index.html", Contents: []byte("<main>Taawun</main>"), SHA256: strings.Repeat("b", 64)},
	}
	authorizer := &fakeWorkspaceAuthorizer{workspace: &models.Workspace{ID: 42, Status: models.WorkspaceStatusActive}}
	server, err := NewMCPServer(store, authorizer, testCurrentUser, ServerOptions{
		OriginAuthorizer:      mustStaticOriginAuthorizer(t, "https://app.example"),
		AuthorizationBoundary: allowAllAuthorizationBoundary{},
		Now:                   func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}

	httpServer := newProtectedTestServer(t, server.Handler(), "https://app.example")
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "1.0.0"}, &mcpsdk.ClientOptions{Capabilities: &mcpsdk.ClientCapabilities{}})
	session, err := client.Connect(t.Context(), &mcpsdk.StreamableClientTransport{
		Endpoint:             httpServer.URL + "/mcp",
		HTTPClient:           &http.Client{Transport: authTransport{base: http.DefaultTransport, token: "valid", origin: "https://app.example"}},
		MaxRetries:           -1,
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatalf("connect remote MCP: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	initialized := session.InitializeResult()
	if initialized == nil || initialized.Capabilities == nil || initialized.Capabilities.Tools == nil {
		t.Fatalf("initialize capabilities = %#v, want tools", initialized)
	}
	if initialized.Capabilities.Resources != nil || len(initialized.Capabilities.Extensions) != 0 {
		t.Fatalf("unexpected resource or MCP Apps capability: %#v", initialized.Capabilities)
	}

	listed, err := session.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	wantNames := []string{
		"taawun_list_templates",
		"taawun_list_primitives",
		"taawun_inspect_template",
		"taawun_inspect_primitive",
		"taawun_audit_compliance",
		"taawun_compose_card_bundle",
		"taawun_get_manifest",
		"taawun_get_card",
	}
	if len(listed.Tools) != len(wantNames) {
		t.Fatalf("tool count = %d, want %d", len(listed.Tools), len(wantNames))
	}
	toolsByName := make(map[string]*mcpsdk.Tool, len(listed.Tools))
	for _, tool := range listed.Tools {
		toolsByName[tool.Name] = tool
		if tool.InputSchema == nil || tool.OutputSchema == nil {
			t.Fatalf("tool %s is missing typed schema", tool.Name)
		}
		if tool.Annotations == nil || tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint {
			t.Fatalf("tool %s has inaccurate destructive annotation: %#v", tool.Name, tool.Annotations)
		}
		if _, hasUI := tool.Meta["ui"]; hasUI {
			t.Fatalf("tool %s unexpectedly advertises UI metadata", tool.Name)
		}
	}
	for _, name := range wantNames {
		if toolsByName[name] == nil {
			t.Errorf("missing tool %s", name)
		}
	}
	for _, forbidden := range []string{"deploy_container", "init_p2p_relay", "run_ethics_check"} {
		if toolsByName[forbidden] != nil {
			t.Errorf("legacy or unsafe tool %s must not be advertised", forbidden)
		}
	}
	if !toolsByName["taawun_list_templates"].Annotations.ReadOnlyHint || toolsByName["taawun_compose_card_bundle"].Annotations.ReadOnlyHint {
		t.Fatal("read/write annotations do not match tool behavior")
	}
	if !schemaContainsObjectProperty(toolsByName["taawun_compose_card_bundle"].InputSchema, "data") {
		t.Fatalf("compose schema no longer advertises component data as an object: %#v", toolsByName["taawun_compose_card_bundle"].InputSchema)
	}

	assertSuccessfulTool(t, session, "taawun_list_templates", map[string]any{})
	assertSuccessfulTool(t, session, "taawun_inspect_template", map[string]any{"templateId": artifacts.TemplateCommunityIftar})
	assertSuccessfulTool(t, session, "taawun_audit_compliance", map[string]any{
		"workspaceId": 42, "prompt": "Build an interest-free donation campaign with explicit settlement terms.", "madhhab": "hanafi",
	})
	buildResult := assertSuccessfulTool(t, session, "taawun_compose_card_bundle", map[string]any{
		"workspaceId":      42,
		"appName":          "Community Iftar",
		"organizationName": "Masjid Example",
		"city":             "Denver",
		"madhhab":          "hanafi",
		"templateId":       artifacts.TemplateCommunityIftar,
		"theme":            map[string]any{"accentColor": "#166534"},
		"modules":          []string{artifacts.ModuleRegistration},
		"components": []map[string]any{{"id": artifacts.ModuleRegistration, "type": artifacts.ModuleRegistration,
			"data": map[string]any{"title": "Register", "summary": "Join tonight", "audience": []any{"families", 3}}}},
		"allowedOrigins": map[string]any{"surfaces": []string{"https://app.example"}},
		"ttlHours":       24,
	})
	if buildResult.StructuredContent == nil {
		t.Fatal("compose tool must return structured content")
	}
	store.mu.Lock()
	lastBuild := store.lastBuild
	store.mu.Unlock()
	if lastBuild.Subject.UserID != 7 || lastBuild.Subject.ID != "taawun:user:7" || lastBuild.WorkspaceID != 42 {
		t.Fatalf("build identity binding = %#v, want authenticated user 7 in workspace 42", lastBuild)
	}
	if !lastBuild.ExpiresAt.Equal(now.Add(24 * time.Hour)) {
		t.Fatalf("expiresAt = %s", lastBuild.ExpiresAt)
	}
	if lastBuild.Lifecycle != artifacts.BundleLifecyclePreview {
		t.Fatalf("lifecycle = %s, want preview", lastBuild.Lifecycle)
	}
	if len(lastBuild.Components) != 1 || string(lastBuild.Components[0].Data) != `{"audience":["families",3],"summary":"Join tonight","title":"Register"}` {
		t.Fatalf("MCP component payload = %#v", lastBuild.Components)
	}
	assertSuccessfulTool(t, session, "taawun_get_manifest", map[string]any{"workspaceId": 42, "contentHash": hash})
	card := assertSuccessfulTool(t, session, "taawun_get_card", map[string]any{"workspaceId": 42, "contentHash": hash, "path": "standalone/index.html"})
	encoded, _ := json.Marshal(card.StructuredContent)
	var cardOutput CardFileOutput
	if err := json.Unmarshal(encoded, &cardOutput); err != nil {
		t.Fatalf("decode card output: %v", err)
	}
	if cardOutput.Contents != "<main>Taawun</main>" {
		t.Fatalf("card output = %#v", cardOutput)
	}
}

func TestWorkspaceAuthorizationAndArtifactBindingAreEnforced(t *testing.T) {
	hash := strings.Repeat("c", 64)
	store := &fakeArtifactStore{openResult: artifacts.BuildResult{
		ArtifactID: "art_other", ContentHash: hash,
		Manifest: artifacts.Manifest{
			ArtifactID: "art_other", ContentHash: hash, WorkspaceID: 99,
			Authorization: artifacts.BundleAuthorization{Subject: artifacts.BundleSubject{WorkspaceID: 99}},
		},
	}}
	authorizer := &fakeWorkspaceAuthorizer{workspace: &models.Workspace{ID: 42, Status: models.WorkspaceStatusActive}, denied: true}
	server, err := NewMCPServer(store, authorizer, testCurrentUser, ServerOptions{OriginAuthorizer: mustStaticOriginAuthorizer(t, "https://app.example"), AuthorizationBoundary: allowAllAuthorizationBoundary{}})
	if err != nil {
		t.Fatal(err)
	}
	session := connectTestClient(t, server.Handler(), "https://app.example")

	denied, err := session.CallTool(t.Context(), &mcpsdk.CallToolParams{Name: "taawun_compose_card_bundle", Arguments: map[string]any{
		"workspaceId": 42, "appName": "Denied", "organizationName": "Denied", "city": "Denver", "madhhab": "hanafi",
		"templateId": artifacts.TemplateCommunityIftar, "theme": map[string]any{}, "modules": []string{artifacts.ModuleRegistration},
		"allowedOrigins": map[string]any{"surfaces": []string{"https://app.example"}}, "ttlHours": 1,
	}})
	if err != nil {
		t.Fatalf("denied tool call should return a tool result: %v", err)
	}
	if !denied.IsError || authorizer.capability != models.WorkspaceCapabilityBuild {
		t.Fatalf("denied result = %#v, capability = %s", denied, authorizer.capability)
	}
	store.mu.Lock()
	buildCalls := store.buildCalls
	store.mu.Unlock()
	if buildCalls != 0 {
		t.Fatalf("artifact build called %d times despite denied workspace", buildCalls)
	}

	authorizer.denied = false
	mismatch, err := session.CallTool(t.Context(), &mcpsdk.CallToolParams{Name: "taawun_get_manifest", Arguments: map[string]any{"workspaceId": 42, "contentHash": hash}})
	if err != nil {
		t.Fatalf("mismatch tool call: %v", err)
	}
	if !mismatch.IsError || !toolTextContains(mismatch, "not bound") {
		t.Fatalf("cross-workspace artifact result = %#v", mismatch)
	}

	unapproved, err := session.CallTool(t.Context(), &mcpsdk.CallToolParams{Name: "taawun_compose_card_bundle", Arguments: map[string]any{
		"workspaceId": 42, "appName": "Origin Test", "organizationName": "Masjid Example", "city": "Denver", "madhhab": "hanafi",
		"templateId": artifacts.TemplateCommunityIftar, "theme": map[string]any{}, "modules": []string{artifacts.ModuleRegistration},
		"allowedOrigins": map[string]any{"surfaces": []string{"https://evil.example"}}, "ttlHours": 1,
	}})
	if err != nil {
		t.Fatalf("unapproved origin tool call: %v", err)
	}
	if !unapproved.IsError || !toolTextContains(unapproved, "not approved") {
		t.Fatalf("unapproved origin result = %#v", unapproved)
	}
	store.mu.Lock()
	buildCalls = store.buildCalls
	store.mu.Unlock()
	if buildCalls != 0 {
		t.Fatalf("artifact build called %d times despite unapproved origin", buildCalls)
	}
}

func TestExactHTTPProtectionAndBearerBoundary(t *testing.T) {
	store := &fakeArtifactStore{}
	authorizer := &fakeWorkspaceAuthorizer{workspace: &models.Workspace{ID: 42}}
	server, err := NewMCPServer(store, authorizer, testCurrentUser, ServerOptions{OriginAuthorizer: mustStaticOriginAuthorizer(t, "https://allowed.example"), AuthorizationBoundary: allowAllAuthorizationBoundary{}})
	if err != nil {
		t.Fatal(err)
	}

	next := bearerTestMiddleware(server.Handler())
	protected := ProtectExactHTTP([]string{"https://allowed.example"}, []string{"mcp.example"}, next)
	tests := []struct {
		name       string
		host       string
		origin     string
		fetchSite  string
		token      string
		wantStatus int
	}{
		{name: "missing bearer", host: "mcp.example", origin: "https://allowed.example", wantStatus: http.StatusUnauthorized},
		{name: "wrong origin", host: "mcp.example", origin: "https://evil.example", token: "valid", wantStatus: http.StatusForbidden},
		{name: "wrong host", host: "evil.example", origin: "https://allowed.example", token: "valid", wantStatus: http.StatusForbidden},
		{name: "cross site without origin", host: "mcp.example", fetchSite: "cross-site", token: "valid", wantStatus: http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "http://mcp.example/mcp", strings.NewReader(`{}`))
			request.Host = test.host
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			if test.fetchSite != "" {
				request.Header.Set("Sec-Fetch-Site", test.fetchSite)
			}
			if test.token != "" {
				request.Header.Set("Authorization", "Bearer "+test.token)
			}
			response := httptest.NewRecorder()
			protected.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.wantStatus, response.Body.String())
			}
		})
	}

	// A non-browser client with bearer authentication and no Origin must pass the HTTP security boundary.
	request := httptest.NewRequest(http.MethodPost, "http://mcp.example/mcp", strings.NewReader(`{}`))
	request.Host = "mcp.example"
	request.Header.Set("Authorization", "Bearer valid")
	response := httptest.NewRecorder()
	protected.ServeHTTP(response, request)
	if response.Code == http.StatusUnauthorized || response.Code == http.StatusForbidden {
		t.Fatalf("non-browser request was blocked by boundary: %d %s", response.Code, response.Body.String())
	}
}

func connectTestClient(t *testing.T, handler http.Handler, origin string) *mcpsdk.ClientSession {
	t.Helper()
	httpServer := newProtectedTestServer(t, handler, origin)
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "1.0.0"}, &mcpsdk.ClientOptions{Capabilities: &mcpsdk.ClientCapabilities{}})
	session, err := client.Connect(t.Context(), &mcpsdk.StreamableClientTransport{
		Endpoint:             httpServer.URL + "/mcp",
		HTTPClient:           &http.Client{Transport: authTransport{base: http.DefaultTransport, token: "valid", origin: origin}},
		MaxRetries:           -1,
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatalf("connect remote MCP: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func newProtectedTestServer(t *testing.T, handler http.Handler, origin string) *httptest.Server {
	t.Helper()
	server := httptest.NewUnstartedServer(nil)
	host := server.Listener.Addr().String()
	server.Config.Handler = ProtectExactHTTP([]string{origin}, []string{host}, bearerTestMiddleware(handler))
	server.Start()
	t.Cleanup(server.Close)
	return server
}

func bearerTestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer valid" {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		user := &models.User{ID: 7, Username: "builder", Status: models.StatusActive}
		ctx := context.WithValue(request.Context(), testUserContextKey{}, user)
		next.ServeHTTP(w, request.WithContext(ctx))
	})
}

func testCurrentUser(ctx context.Context) (*models.User, bool) {
	user, ok := ctx.Value(testUserContextKey{}).(*models.User)
	return user, ok && user != nil
}

func mustStaticOriginAuthorizer(t *testing.T, origins ...string) OriginAuthorizer {
	t.Helper()
	authorizer, err := NewStaticOriginAuthorizer(origins)
	if err != nil {
		t.Fatal(err)
	}
	return authorizer
}

func assertSuccessfulTool(t *testing.T, session *mcpsdk.ClientSession, name string, arguments map[string]any) *mcpsdk.CallToolResult {
	t.Helper()
	result, err := session.CallTool(t.Context(), &mcpsdk.CallToolParams{Name: name, Arguments: arguments})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	if result.IsError {
		t.Fatalf("call %s returned tool error: %#v", name, result.Content)
	}
	if result.StructuredContent == nil || len(result.Content) == 0 {
		t.Fatalf("call %s missing structured or text fallback: %#v", name, result)
	}
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	if !ok || strings.TrimSpace(text.Text) == "" {
		t.Fatalf("call %s has invalid text fallback: %#v", name, result.Content)
	}
	return result
}

func toolTextContains(result *mcpsdk.CallToolResult, substring string) bool {
	for _, content := range result.Content {
		if text, ok := content.(*mcpsdk.TextContent); ok && strings.Contains(text.Text, substring) {
			return true
		}
	}
	return false
}

func schemaContainsObjectProperty(value any, property string) bool {
	switch typed := value.(type) {
	case map[string]any:
		if properties, ok := typed["properties"].(map[string]any); ok {
			if candidate, ok := properties[property].(map[string]any); ok && candidate["type"] == "object" {
				return true
			}
		}
		for _, child := range typed {
			if schemaContainsObjectProperty(child, property) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if schemaContainsObjectProperty(child, property) {
				return true
			}
		}
	}
	return false
}

func TestProtectedServerURLUsesExactHost(t *testing.T) {
	// Guard against accidentally changing httptest setup to derive the allowlist from an Origin.
	testServer := httptest.NewUnstartedServer(http.NotFoundHandler())
	host := testServer.Listener.Addr().String()
	parsed, err := url.Parse("http://" + host)
	if err != nil || parsed.Host != host {
		t.Fatalf("test host %q is not canonical: %v", host, err)
	}
	testServer.Close()
}
