// Package mcp exposes Taawun's curated vibecoding control plane over MCP.
// See AGENTS.md for the authorization and execution boundaries.
package mcp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"taawun/pkg/artifacts"
	"taawun/pkg/ethics"
	"taawun/pkg/models"
)

const (
	serverName      = "taawun-vibecoding-control-plane"
	serverVersion   = "0.2.0"
	mcpScopeRead    = "taawun:read"
	mcpScopeBuild   = "taawun:build"
	mcpScopePublish = "taawun:publish"
)

var (
	closedWorld    = boolPointer(false)
	nonDestructive = boolPointer(false)
)

// OAuthScopeRequirements returns the HTTP step-up scope for every registered tool.
func OAuthScopeRequirements() map[string]string {
	return map[string]string{
		"taawun_list_templates":      mcpScopeRead,
		"taawun_list_primitives":     mcpScopeRead,
		"taawun_inspect_template":    mcpScopeRead,
		"taawun_inspect_primitive":   mcpScopeRead,
		"taawun_audit_compliance":    mcpScopeRead,
		"taawun_compose_card_bundle": mcpScopeBuild,
		"taawun_get_manifest":        mcpScopeRead,
		"taawun_get_card":            mcpScopeRead,
		"taawun_stage_card_bundle":   mcpScopePublish,
	}
}

// ArtifactStore is the immutable signed-bundle dependency used by lifecycle tools.
type ArtifactStore interface {
	Build(context.Context, artifacts.BuildRequest) (artifacts.BuildResult, error)
	Open(context.Context, string) (artifacts.BuildResult, error)
	ReadFile(context.Context, string, string) (artifacts.ArtifactFile, error)
}

// WorkspaceAuthorizer decides capabilities from the authenticated actor and persisted membership.
type WorkspaceAuthorizer interface {
	AuthorizeWorkspaceCapability(*models.User, int, models.WorkspaceCapability) (*models.Workspace, error)
}

// CurrentUserFunc obtains the user placed in the request context by bearer authentication.
type CurrentUserFunc func(context.Context) (*models.User, bool)

// OriginAuthorizer resolves the subset of requested origins approved for a workspace.
type OriginAuthorizer interface {
	AuthorizeOrigins(context.Context, *models.User, *models.Workspace, artifacts.OriginPolicy) (artifacts.OriginPolicy, error)
}

type lifecycleOriginAuthorizer interface {
	AuthorizeOriginsForLifecycle(context.Context, *models.User, *models.Workspace, artifacts.BundleLifecycle, artifacts.OriginPolicy) (artifacts.OriginPolicy, error)
}

// Publisher stages an already-built bundle to a domain approved by its signed manifest.
type Publisher interface {
	Stage(context.Context, *models.User, *models.Workspace, artifacts.BuildResult, string) (PublishResult, error)
}

// AuthorizationBoundary enforces the OAuth grant carried by the request context.
type AuthorizationBoundary interface {
	AuthorizeScope(context.Context, string) error
	AuthorizeWorkspaceGrant(context.Context, int) error
}

// ServerOptions supplies optional production integrations and deterministic time for tests.
type ServerOptions struct {
	OriginAuthorizer      OriginAuthorizer
	AuthorizationBoundary AuthorizationBoundary
	Publisher             Publisher
	Now                   func() time.Time
}

// MCPServer hosts typed, schema-validated tools through the official Go SDK.
type MCPServer struct {
	sdk          *mcpsdk.Server
	handler      http.Handler
	artifacts    ArtifactStore
	authorizer   WorkspaceAuthorizer
	currentUser  CurrentUserFunc
	ethicsEngine *ethics.HaramCheckEngine
	corpus       *ethics.ComplianceCorpus
	origins      OriginAuthorizer
	boundary     AuthorizationBoundary
	publisher    Publisher
	now          func() time.Time
}

// NewMCPServer creates the authenticated remote control plane.
func NewMCPServer(store ArtifactStore, authorizer WorkspaceAuthorizer, currentUser CurrentUserFunc, options ServerOptions) (*MCPServer, error) {
	if store == nil {
		return nil, errors.New("MCP artifact store is required")
	}
	if authorizer == nil {
		return nil, errors.New("MCP workspace authorizer is required")
	}
	if currentUser == nil {
		return nil, errors.New("MCP current-user resolver is required")
	}
	if options.OriginAuthorizer == nil {
		return nil, errors.New("MCP workspace origin authorizer is required")
	}
	if options.AuthorizationBoundary == nil {
		return nil, errors.New("MCP OAuth authorization boundary is required")
	}
	if options.Now == nil {
		options.Now = time.Now
	}

	sdkServer := mcpsdk.NewServer(
		&mcpsdk.Implementation{
			Name:    serverName,
			Title:   "Taawun Vibecoding Control Plane",
			Version: serverVersion,
		},
		&mcpsdk.ServerOptions{
			Capabilities: &mcpsdk.ServerCapabilities{},
			Instructions: "Discover curated Taawun templates, audit requirements, and compose signed declarative card bundles. Every workspace operation is authorized from the bearer-authenticated user. The server never executes arbitrary code or deploys containers. Compliance output is reference-only and requires qualified review.",
		},
	)
	server := &MCPServer{
		sdk:          sdkServer,
		artifacts:    store,
		authorizer:   authorizer,
		currentUser:  currentUser,
		ethicsEngine: ethics.NewHaramCheckEngine(),
		corpus:       ethics.NewSeedComplianceCorpus(),
		origins:      options.OriginAuthorizer,
		boundary:     options.AuthorizationBoundary,
		publisher:    options.Publisher,
		now:          options.Now,
	}
	server.registerTools()
	server.handler = mcpsdk.NewStreamableHTTPHandler(
		func(*http.Request) *mcpsdk.Server { return sdkServer },
		&mcpsdk.StreamableHTTPOptions{
			SessionTimeout: 30 * time.Minute,
		},
	)
	return server, nil
}

// Handler returns the stateful Streamable HTTP transport.
func (s *MCPServer) Handler() http.Handler {
	return s.handler
}

func (s *MCPServer) registerTools() {
	readOnly := func(title string) *mcpsdk.ToolAnnotations {
		return &mcpsdk.ToolAnnotations{
			Title:           title,
			ReadOnlyHint:    true,
			DestructiveHint: nonDestructive,
			IdempotentHint:  true,
			OpenWorldHint:   closedWorld,
		}
	}
	additive := func(title string, openWorld bool) *mcpsdk.ToolAnnotations {
		return &mcpsdk.ToolAnnotations{
			Title:           title,
			ReadOnlyHint:    false,
			DestructiveHint: nonDestructive,
			IdempotentHint:  true,
			OpenWorldHint:   boolPointer(openWorld),
		}
	}

	mcpsdk.AddTool(s.sdk, &mcpsdk.Tool{
		Name:        "taawun_list_templates",
		Title:       "List Taawun templates",
		Description: "Lists the curated, versioned templates that can be composed into signed Taawun card bundles. This does not build or execute anything.",
		Annotations: readOnly("List Taawun templates"),
	}, s.listTemplates)

	mcpsdk.AddTool(s.sdk, &mcpsdk.Tool{
		Name:        "taawun_list_primitives",
		Title:       "List Taawun primitives",
		Description: "Lists the declarative product primitives available to curated templates, including capabilities, routes, events, data classifications, and allowed server signals.",
		Annotations: readOnly("List Taawun primitives"),
	}, s.listPrimitives)

	mcpsdk.AddTool(s.sdk, &mcpsdk.Tool{
		Name:        "taawun_inspect_template",
		Title:       "Inspect a Taawun template",
		Description: "Returns one curated template contract and the complete contracts of its allowed primitives. It reports composition boundaries without generating files.",
		Annotations: readOnly("Inspect a Taawun template"),
	}, s.inspectTemplate)

	mcpsdk.AddTool(s.sdk, &mcpsdk.Tool{
		Name:        "taawun_inspect_primitive",
		Title:       "Inspect a Taawun primitive",
		Description: "Returns one declarative primitive contract. Primitive contracts describe renderer-owned UI and signals; they are not executable plugins.",
		Annotations: readOnly("Inspect a Taawun primitive"),
	}, s.inspectPrimitive)

	mcpsdk.AddTool(s.sdk, &mcpsdk.Tool{
		Name:        "taawun_audit_compliance",
		Title:       "Audit a build prompt",
		Description: "Audits a prompt inside an authorized workspace and injects reference-only compliance context for a selected Hanafi, Shafii, Maliki, or Hanbali baseline. The result is not a fatwa or scholar approval.",
		Annotations: readOnly("Audit a build prompt"),
	}, s.auditCompliance)

	mcpsdk.AddTool(s.sdk, &mcpsdk.Tool{
		Name:        "taawun_compose_card_bundle",
		Title:       "Compose a signed card bundle",
		Description: "Builds an immutable, content-addressed card bundle from one curated template. The server binds the signed bundle to the bearer-authenticated user and authorized workspace; arbitrary source code and container execution are not accepted.",
		Annotations: additive("Compose a signed card bundle", false),
	}, s.composeCardBundle)

	mcpsdk.AddTool(s.sdk, &mcpsdk.Tool{
		Name:        "taawun_get_manifest",
		Title:       "Get a signed card manifest",
		Description: "Verifies and returns a signed card manifest after re-authorizing access to the manifest's requested workspace. Server filesystem paths are never returned.",
		Annotations: readOnly("Get a signed card manifest"),
	}, s.getManifest)

	mcpsdk.AddTool(s.sdk, &mcpsdk.Tool{
		Name:        "taawun_get_card",
		Title:       "Get a card bundle file",
		Description: "Verifies a signed card bundle and returns one manifest-listed UTF-8 file after workspace authorization. Paths outside the immutable bundle are rejected.",
		Annotations: readOnly("Get a card bundle file"),
	}, s.getCard)

	if s.publisher != nil {
		mcpsdk.AddTool(s.sdk, &mcpsdk.Tool{
			Name:        "taawun_stage_card_bundle",
			Title:       "Stage a card bundle",
			Description: "Stages a verified bundle through the configured publisher, only to a domain already approved in its signed manifest. Requires workspace publish capability and never accepts deployment commands or container images.",
			Annotations: additive("Stage a card bundle", true),
		}, s.stageCardBundle)
	}
}

type EmptyInput struct{}

type TemplateListOutput struct {
	Templates           []artifacts.TemplateCatalogEntry `json:"templates"`
	PublishingAvailable bool                             `json:"publishingAvailable"`
}

type PrimitiveListOutput struct {
	Primitives []artifacts.ModuleDescriptor `json:"primitives"`
}

type InspectTemplateInput struct {
	TemplateID string `json:"templateId" jsonschema:"Exact ID returned by taawun_list_templates"`
}

type TemplateContractOutput struct {
	Template          artifacts.TemplateCatalogEntry `json:"template"`
	Primitives        []artifacts.ModuleDescriptor   `json:"primitives"`
	RenderModes       []artifacts.RenderMode         `json:"renderModes"`
	CompositionPolicy string                         `json:"compositionPolicy"`
}

type InspectPrimitiveInput struct {
	PrimitiveID string `json:"primitiveId" jsonschema:"Exact ID returned by taawun_list_primitives"`
}

type PrimitiveContractOutput struct {
	Primitive       artifacts.ModuleDescriptor `json:"primitive"`
	ExecutionPolicy string                     `json:"executionPolicy"`
}

type AuditComplianceInput struct {
	WorkspaceID int    `json:"workspaceId" jsonschema:"Authorized Taawun workspace ID"`
	Prompt      string `json:"prompt" jsonschema:"Product requirement or build prompt to audit"`
	Madhhab     string `json:"madhhab" jsonschema:"Jurisprudence baseline: hanafi, shafii, maliki, or hanbali"`
}

type AuditComplianceOutput struct {
	Audit             *ethics.ComplianceResult  `json:"audit"`
	Madhhab           ethics.Madhhab            `json:"madhhab"`
	GenerationContext string                    `json:"generationContext"`
	References        []ethics.ComplianceRecord `json:"references"`
	Disclaimer        string                    `json:"disclaimer"`
}

type ComposeCardBundleInput struct {
	WorkspaceID      int                    `json:"workspaceId" jsonschema:"Authorized workspace that will own the signed bundle"`
	AppName          string                 `json:"appName" jsonschema:"User-facing application name"`
	OrganizationName string                 `json:"organizationName" jsonschema:"User-facing community or organization name"`
	City             string                 `json:"city" jsonschema:"User-facing city or locality"`
	Madhhab          string                 `json:"madhhab" jsonschema:"Jurisprudence baseline: hanafi, shafii, maliki, or hanbali"`
	TemplateID       string                 `json:"templateId" jsonschema:"Curated template ID"`
	Theme            artifacts.ThemeRequest `json:"theme" jsonschema:"Curated theme token values"`
	Modules          []string               `json:"modules" jsonschema:"Primitive IDs allowed by the selected template"`
	AllowedOrigins   artifacts.OriginPolicy `json:"allowedOrigins" jsonschema:"Exact HTTPS origins for surfaces, embedders, connections, and resources"`
	TTLHours         int                    `json:"ttlHours" jsonschema:"Authorization lifetime in hours, from 1 through 2160"`
}

type CardBundleOutput struct {
	ArtifactID  string             `json:"artifactId"`
	ContentHash string             `json:"contentHash"`
	Manifest    artifacts.Manifest `json:"manifest"`
}

type GetManifestInput struct {
	WorkspaceID int    `json:"workspaceId" jsonschema:"Authorized workspace expected to own the artifact"`
	ContentHash string `json:"contentHash" jsonschema:"SHA-256 content address returned by taawun_compose_card_bundle"`
}

type ManifestOutput struct {
	Manifest            artifacts.Manifest `json:"manifest"`
	AuthorizationActive bool               `json:"authorizationActive"`
}

type GetCardInput struct {
	WorkspaceID int    `json:"workspaceId" jsonschema:"Authorized workspace expected to own the artifact"`
	ContentHash string `json:"contentHash" jsonschema:"SHA-256 content address returned by taawun_compose_card_bundle"`
	Path        string `json:"path" jsonschema:"Exact manifest-listed bundle path, such as standalone/index.html"`
}

type CardFileOutput struct {
	Path     string `json:"path"`
	Encoding string `json:"encoding"`
	SHA256   string `json:"sha256"`
	Contents string `json:"contents"`
}

type StageCardInput struct {
	WorkspaceID int    `json:"workspaceId" jsonschema:"Workspace with publish capability"`
	ContentHash string `json:"contentHash" jsonschema:"Verified signed bundle content address"`
	Domain      string `json:"domain" jsonschema:"Exact domain already approved by the signed manifest"`
}

// PublishResult is returned by an optional approved-domain publisher.
type PublishResult struct {
	Status  string `json:"status"`
	Domain  string `json:"domain"`
	Preview string `json:"preview,omitempty"`
}

func (s *MCPServer) listTemplates(ctx context.Context, _ *mcpsdk.CallToolRequest, _ EmptyInput) (*mcpsdk.CallToolResult, TemplateListOutput, error) {
	if err := s.boundary.AuthorizeScope(ctx, mcpScopeRead); err != nil {
		return nil, TemplateListOutput{}, err
	}
	if _, err := s.actor(ctx); err != nil {
		return nil, TemplateListOutput{}, err
	}
	templates := artifacts.ListTemplates()
	return textResult(fmt.Sprintf("%d curated templates are available; publishing integration available: %t.", len(templates), s.publisher != nil)), TemplateListOutput{
		Templates:           templates,
		PublishingAvailable: s.publisher != nil,
	}, nil
}

func (s *MCPServer) listPrimitives(ctx context.Context, _ *mcpsdk.CallToolRequest, _ EmptyInput) (*mcpsdk.CallToolResult, PrimitiveListOutput, error) {
	if err := s.boundary.AuthorizeScope(ctx, mcpScopeRead); err != nil {
		return nil, PrimitiveListOutput{}, err
	}
	if _, err := s.actor(ctx); err != nil {
		return nil, PrimitiveListOutput{}, err
	}
	primitives := artifacts.ListModules()
	return textResult(fmt.Sprintf("%d curated declarative primitives are available.", len(primitives))), PrimitiveListOutput{Primitives: primitives}, nil
}

func (s *MCPServer) inspectTemplate(ctx context.Context, _ *mcpsdk.CallToolRequest, input InspectTemplateInput) (*mcpsdk.CallToolResult, TemplateContractOutput, error) {
	if err := s.boundary.AuthorizeScope(ctx, mcpScopeRead); err != nil {
		return nil, TemplateContractOutput{}, err
	}
	if _, err := s.actor(ctx); err != nil {
		return nil, TemplateContractOutput{}, err
	}
	template, found := artifacts.GetTemplate(input.TemplateID)
	if !found {
		return nil, TemplateContractOutput{}, fmt.Errorf("unknown template %q; call taawun_list_templates for supported IDs", input.TemplateID)
	}
	primitives := make([]artifacts.ModuleDescriptor, 0, len(template.AllowedModules))
	for _, primitiveID := range template.AllowedModules {
		primitive, ok := artifacts.GetModule(primitiveID)
		if !ok {
			return nil, TemplateContractOutput{}, fmt.Errorf("template %q references unavailable primitive %q", input.TemplateID, primitiveID)
		}
		primitives = append(primitives, primitive)
	}
	return textResult(fmt.Sprintf("Template %s@%s allows %d curated primitives in standalone or embed mode.", template.Identity.ID, template.Identity.Version, len(primitives))), TemplateContractOutput{
		Template:          template,
		Primitives:        primitives,
		RenderModes:       []artifacts.RenderMode{artifacts.RenderModeStandalone, artifacts.RenderModeEmbed},
		CompositionPolicy: "Curated declarative composition only; no arbitrary code, package installation, shell command, or container execution.",
	}, nil
}

func (s *MCPServer) inspectPrimitive(ctx context.Context, _ *mcpsdk.CallToolRequest, input InspectPrimitiveInput) (*mcpsdk.CallToolResult, PrimitiveContractOutput, error) {
	if err := s.boundary.AuthorizeScope(ctx, mcpScopeRead); err != nil {
		return nil, PrimitiveContractOutput{}, err
	}
	if _, err := s.actor(ctx); err != nil {
		return nil, PrimitiveContractOutput{}, err
	}
	primitive, found := artifacts.GetModule(input.PrimitiveID)
	if !found {
		return nil, PrimitiveContractOutput{}, fmt.Errorf("unknown primitive %q; call taawun_list_primitives for supported IDs", input.PrimitiveID)
	}
	return textResult(fmt.Sprintf("Primitive %s@%s is declarative and exposes %d named capabilities.", primitive.ID, primitive.Version, len(primitive.Capabilities))), PrimitiveContractOutput{
		Primitive:       primitive,
		ExecutionPolicy: "Renderer-owned declarative contract; host communication is limited to the listed events and server signals.",
	}, nil
}

func (s *MCPServer) auditCompliance(ctx context.Context, _ *mcpsdk.CallToolRequest, input AuditComplianceInput) (*mcpsdk.CallToolResult, AuditComplianceOutput, error) {
	if err := s.boundary.AuthorizeScope(ctx, mcpScopeRead); err != nil {
		return nil, AuditComplianceOutput{}, err
	}
	if _, _, err := s.authorize(ctx, input.WorkspaceID, models.WorkspaceCapabilityAudit); err != nil {
		return nil, AuditComplianceOutput{}, err
	}
	prompt := strings.TrimSpace(input.Prompt)
	if prompt == "" {
		return nil, AuditComplianceOutput{}, errors.New("prompt is required")
	}
	madhhab, err := ethics.ParseMadhhab(input.Madhhab)
	if err != nil {
		return nil, AuditComplianceOutput{}, fmt.Errorf("invalid madhhab: %w", err)
	}
	audit, err := s.ethicsEngine.AuditPrompt(prompt)
	if err != nil {
		return nil, AuditComplianceOutput{}, errors.New("compliance audit failed")
	}
	generationContext, references, err := s.corpus.InjectContext(prompt, madhhab, 5)
	if err != nil {
		return nil, AuditComplianceOutput{}, fmt.Errorf("compliance reference retrieval failed: %w", err)
	}
	status := "passed"
	if !audit.Passed {
		status = "requires changes"
	}
	return textResult(fmt.Sprintf("Compliance audit %s against the %s baseline with %d seed references. Qualified review is still required.", status, madhhab, len(references))), AuditComplianceOutput{
		Audit:             audit,
		Madhhab:           madhhab,
		GenerationContext: generationContext,
		References:        references,
		Disclaimer:        "Reference-only seed material; not a fatwa or scholar approval. Obtain qualified review before reliance.",
	}, nil
}

func (s *MCPServer) composeCardBundle(ctx context.Context, _ *mcpsdk.CallToolRequest, input ComposeCardBundleInput) (*mcpsdk.CallToolResult, CardBundleOutput, error) {
	if err := s.boundary.AuthorizeScope(ctx, mcpScopeBuild); err != nil {
		return nil, CardBundleOutput{}, err
	}
	actor, workspace, err := s.authorize(ctx, input.WorkspaceID, models.WorkspaceCapabilityBuild)
	if err != nil {
		return nil, CardBundleOutput{}, err
	}
	if input.TTLHours < 1 || input.TTLHours > 90*24 {
		return nil, CardBundleOutput{}, errors.New("ttlHours must be between 1 and 2160")
	}
	madhhab, err := ethics.ParseMadhhab(input.Madhhab)
	if err != nil {
		return nil, CardBundleOutput{}, fmt.Errorf("invalid madhhab: %w", err)
	}
	requestedOrigins := cloneOriginPolicy(input.AllowedOrigins)
	var authorizedOrigins artifacts.OriginPolicy
	if lifecycleAuthorizer, ok := s.origins.(lifecycleOriginAuthorizer); ok {
		authorizedOrigins, err = lifecycleAuthorizer.AuthorizeOriginsForLifecycle(ctx, actor, workspace, artifacts.BundleLifecyclePreview, requestedOrigins)
	} else {
		authorizedOrigins, err = s.origins.AuthorizeOrigins(ctx, actor, workspace, requestedOrigins)
	}
	if err != nil {
		return nil, CardBundleOutput{}, fmt.Errorf("artifact origin authorization failed: %w", err)
	}
	request := artifacts.BuildRequest{
		WorkspaceID:      input.WorkspaceID,
		AppName:          input.AppName,
		OrganizationName: input.OrganizationName,
		City:             input.City,
		Madhhab:          madhhab,
		TemplateID:       input.TemplateID,
		Theme:            input.Theme,
		Modules:          append([]string(nil), input.Modules...),
		AllowedOrigins:   cloneOriginPolicy(authorizedOrigins),
		Subject: artifacts.SubjectBinding{
			ID:     fmt.Sprintf("taawun:user:%d", actor.ID),
			UserID: actor.ID,
		},
		ExpiresAt: s.now().UTC().Add(time.Duration(input.TTLHours) * time.Hour),
		Lifecycle: artifacts.BundleLifecyclePreview,
	}
	built, err := s.artifacts.Build(ctx, request)
	if err != nil {
		return nil, CardBundleOutput{}, fmt.Errorf("card bundle composition failed: %w", err)
	}
	if err := validateArtifactWorkspace(built, input.WorkspaceID); err != nil {
		return nil, CardBundleOutput{}, err
	}
	output := CardBundleOutput{ArtifactID: built.ArtifactID, ContentHash: built.ContentHash, Manifest: built.Manifest}
	return textResult(fmt.Sprintf("Built signed bundle %s at content hash %s for workspace %d; it expires %s.", built.ArtifactID, built.ContentHash, input.WorkspaceID, built.Manifest.Authorization.ExpiresAt.UTC().Format(time.RFC3339))), output, nil
}

func (s *MCPServer) getManifest(ctx context.Context, _ *mcpsdk.CallToolRequest, input GetManifestInput) (*mcpsdk.CallToolResult, ManifestOutput, error) {
	if err := s.boundary.AuthorizeScope(ctx, mcpScopeRead); err != nil {
		return nil, ManifestOutput{}, err
	}
	if _, _, err := s.authorize(ctx, input.WorkspaceID, models.WorkspaceCapabilityView); err != nil {
		return nil, ManifestOutput{}, err
	}
	opened, err := s.openAuthorizedArtifact(ctx, input.WorkspaceID, input.ContentHash)
	if err != nil {
		return nil, ManifestOutput{}, err
	}
	active := artifacts.CheckManifestExpiry(opened.Manifest, s.now()) == nil
	return textResult(fmt.Sprintf("Verified signed manifest %s for workspace %d with %d files; authorization active: %t.", opened.Manifest.ArtifactID, input.WorkspaceID, len(opened.Manifest.Files), active)), ManifestOutput{Manifest: opened.Manifest, AuthorizationActive: active}, nil
}

func (s *MCPServer) getCard(ctx context.Context, _ *mcpsdk.CallToolRequest, input GetCardInput) (*mcpsdk.CallToolResult, CardFileOutput, error) {
	if err := s.boundary.AuthorizeScope(ctx, mcpScopeRead); err != nil {
		return nil, CardFileOutput{}, err
	}
	if _, _, err := s.authorize(ctx, input.WorkspaceID, models.WorkspaceCapabilityView); err != nil {
		return nil, CardFileOutput{}, err
	}
	opened, err := s.openAuthorizedArtifact(ctx, input.WorkspaceID, input.ContentHash)
	if err != nil {
		return nil, CardFileOutput{}, err
	}
	if err := artifacts.CheckManifestExpiry(opened.Manifest, s.now()); err != nil {
		return nil, CardFileOutput{}, errors.New("artifact authorization has expired")
	}
	file, err := s.artifacts.ReadFile(ctx, input.ContentHash, input.Path)
	if err != nil {
		return nil, CardFileOutput{}, fmt.Errorf("card file unavailable: %w", err)
	}
	if !utf8.Valid(file.Contents) {
		return nil, CardFileOutput{}, errors.New("card file is not UTF-8 text and cannot be returned by this tool")
	}
	output := CardFileOutput{Path: file.Path, Encoding: "utf-8", SHA256: file.SHA256, Contents: string(file.Contents)}
	return textResult(fmt.Sprintf("Verified and retrieved %s (%d bytes) from workspace %d bundle %s.", file.Path, len(file.Contents), input.WorkspaceID, input.ContentHash)), output, nil
}

func (s *MCPServer) stageCardBundle(ctx context.Context, _ *mcpsdk.CallToolRequest, input StageCardInput) (*mcpsdk.CallToolResult, PublishResult, error) {
	if err := s.boundary.AuthorizeScope(ctx, mcpScopePublish); err != nil {
		return nil, PublishResult{}, err
	}
	actor, workspace, err := s.authorize(ctx, input.WorkspaceID, models.WorkspaceCapabilityPublish)
	if err != nil {
		return nil, PublishResult{}, err
	}
	opened, err := s.openAuthorizedArtifact(ctx, input.WorkspaceID, input.ContentHash)
	if err != nil {
		return nil, PublishResult{}, err
	}
	if err := artifacts.CheckManifestExpiry(opened.Manifest, s.now()); err != nil {
		return nil, PublishResult{}, errors.New("artifact authorization has expired")
	}
	if !containsExact(opened.Manifest.Authorization.ApprovedDomains, input.Domain) {
		return nil, PublishResult{}, errors.New("domain is not approved by the signed artifact manifest")
	}
	result, err := s.publisher.Stage(ctx, actor, workspace, opened, input.Domain)
	if err != nil {
		return nil, PublishResult{}, fmt.Errorf("bundle staging failed: %w", err)
	}
	return textResult(fmt.Sprintf("Bundle %s staged to approved domain %s with status %s.", input.ContentHash, result.Domain, result.Status)), result, nil
}

func (s *MCPServer) actor(ctx context.Context) (*models.User, error) {
	actor, ok := s.currentUser(ctx)
	if !ok || actor == nil || actor.ID <= 0 {
		return nil, errors.New("authenticated user context is required")
	}
	return actor, nil
}

func (s *MCPServer) authorize(ctx context.Context, workspaceID int, capability models.WorkspaceCapability) (*models.User, *models.Workspace, error) {
	actor, err := s.actor(ctx)
	if err != nil {
		return nil, nil, err
	}
	if workspaceID <= 0 {
		return nil, nil, errors.New("workspaceId must be positive")
	}
	if err := s.boundary.AuthorizeWorkspaceGrant(ctx, workspaceID); err != nil {
		return nil, nil, err
	}
	workspace, err := s.authorizer.AuthorizeWorkspaceCapability(actor, workspaceID, capability)
	if err != nil || workspace == nil || workspace.ID != workspaceID {
		return nil, nil, fmt.Errorf("workspace is unavailable to the authenticated user for %s", capability)
	}
	return actor, workspace, nil
}

func (s *MCPServer) openAuthorizedArtifact(ctx context.Context, workspaceID int, contentHash string) (artifacts.BuildResult, error) {
	opened, err := s.artifacts.Open(ctx, contentHash)
	if err != nil {
		return artifacts.BuildResult{}, fmt.Errorf("artifact unavailable or invalid: %w", err)
	}
	if err := validateArtifactWorkspace(opened, workspaceID); err != nil {
		return artifacts.BuildResult{}, err
	}
	return opened, nil
}

func validateArtifactWorkspace(result artifacts.BuildResult, workspaceID int) error {
	if result.Manifest.WorkspaceID != workspaceID || result.Manifest.Authorization.Subject.WorkspaceID != workspaceID {
		return errors.New("artifact is not bound to the authorized workspace")
	}
	if result.ContentHash == "" || result.ContentHash != result.Manifest.ContentHash || result.ArtifactID == "" || result.ArtifactID != result.Manifest.ArtifactID {
		return errors.New("artifact identity does not match its verified manifest")
	}
	return nil
}

func cloneOriginPolicy(policy artifacts.OriginPolicy) artifacts.OriginPolicy {
	return artifacts.OriginPolicy{
		Surfaces:    append([]string(nil), policy.Surfaces...),
		Embedders:   append([]string(nil), policy.Embedders...),
		Connections: append([]string(nil), policy.Connections...),
		Resources:   append([]string(nil), policy.Resources...),
	}
}

func containsExact(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func textResult(text string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: text}}}
}

func boolPointer(value bool) *bool {
	return &value
}

// StaticOriginAuthorizer is the conservative exact-allowlist resolver used until durable workspace domain claims land.
type StaticOriginAuthorizer struct {
	allowed map[string]struct{}
}

// NewStaticOriginAuthorizer validates and copies an exact origin allowlist.
func NewStaticOriginAuthorizer(allowedOrigins []string) (*StaticOriginAuthorizer, error) {
	if len(allowedOrigins) == 0 {
		return nil, errors.New("at least one approved artifact origin is required")
	}
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || strings.Contains(parsed.Hostname(), "*") {
			return nil, fmt.Errorf("invalid approved artifact origin %q", origin)
		}
		if parsed.Scheme != "https" && !(parsed.Scheme == "http" && isLoopbackOriginHost(parsed.Hostname())) {
			return nil, fmt.Errorf("approved artifact origin %q must use HTTPS outside local development", origin)
		}
		if _, duplicate := allowed[origin]; duplicate {
			return nil, fmt.Errorf("duplicate approved artifact origin %q", origin)
		}
		allowed[origin] = struct{}{}
	}
	return &StaticOriginAuthorizer{allowed: allowed}, nil
}

// AuthorizeOrigins requires every requested origin to be in the configured exact set.
func (a *StaticOriginAuthorizer) AuthorizeOrigins(_ context.Context, _ *models.User, _ *models.Workspace, requested artifacts.OriginPolicy) (artifacts.OriginPolicy, error) {
	if a == nil || len(a.allowed) == 0 {
		return artifacts.OriginPolicy{}, errors.New("artifact origins are not configured")
	}
	checks := []struct {
		field   string
		origins []string
	}{
		{field: "surfaces", origins: requested.Surfaces},
		{field: "embedders", origins: requested.Embedders},
		{field: "connections", origins: requested.Connections},
		{field: "resources", origins: requested.Resources},
	}
	for _, check := range checks {
		for _, origin := range check.origins {
			if _, approved := a.allowed[origin]; !approved {
				return artifacts.OriginPolicy{}, fmt.Errorf("%s origin %q is not approved for signed artifacts", check.field, origin)
			}
		}
	}
	return cloneOriginPolicy(requested), nil
}

func isLoopbackOriginHost(host string) bool {
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}
