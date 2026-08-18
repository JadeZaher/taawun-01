// Package artifacts builds immutable bundles from curated Taawun templates.
// See AGENTS.md for the trust and state boundaries.
package artifacts

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"taawun/pkg/ethics"
)

const (
	// TemplateCommunityIftar is a focused example preset in the curated catalog.
	TemplateCommunityIftar = "community-iftar"
	// TemplateCommunityWorkspace exposes the complete modular community workspace.
	TemplateCommunityWorkspace = "community-workspace"
	// TemplateBazaarCooperative focuses the catalog on ethical marketplace workflows.
	TemplateBazaarCooperative = "bazaar-cooperative"
	// Template versions are bumped for any output-affecting template change.
	TemplateCommunityIftarVersion     = "2.0.0"
	TemplateCommunityWorkspaceVersion = "2.0.0"
	TemplateBazaarCooperativeVersion  = "2.0.0"
	defaultAccentColor                = "#166534"
)

var (
	// ErrInvalidTemplate identifies an unsupported curated template.
	ErrInvalidTemplate = errors.New("unsupported artifact template")
	// ErrInvalidBuildRequest identifies request data that cannot safely populate a template.
	ErrInvalidBuildRequest = errors.New("invalid artifact build request")
)

// BuildRequest is transport-neutral so an HTTP or MCP handler can decode it directly.
type BuildRequest struct {
	WorkspaceID      int                 `json:"workspaceId"`
	AppName          string              `json:"appName"`
	OrganizationName string              `json:"organizationName"`
	City             string              `json:"city"`
	Madhhab          ethics.Madhhab      `json:"madhhab"`
	TemplateID       string              `json:"templateId"`
	Theme            ThemeRequest        `json:"theme"`
	Modules          []string            `json:"modules,omitempty"`
	Components       []ComponentInstance `json:"components,omitempty"`
	AllowedOrigins   OriginPolicy        `json:"allowedOrigins"`
	Subject          SubjectBinding      `json:"subject"`
	ExpiresAt        time.Time           `json:"expiresAt"`
	Lifecycle        BundleLifecycle     `json:"lifecycle"`
}

// BundleLifecycle distinguishes short previews from published card bundles.
type BundleLifecycle string

const (
	BundleLifecyclePreview   BundleLifecycle = "preview"
	BundleLifecyclePublished BundleLifecycle = "published"
)

// SubjectBinding binds a generated card bundle to one upstream-authorized principal.
type SubjectBinding struct {
	ID     string `json:"id"`
	UserID int    `json:"userId"`
}

// ThemeRequest contains curated values that map only to namespaced CSS tokens.
type ThemeRequest struct {
	AccentColor string `json:"accentColor,omitempty"`
}

// OriginPolicy explicitly separates embedding, network, and passive-resource trust.
type OriginPolicy struct {
	Surfaces    []string `json:"surfaces"`
	Embedders   []string `json:"embedders,omitempty"`
	Connections []string `json:"connections,omitempty"`
	Resources   []string `json:"resources,omitempty"`
}

// DecodeBuildRequest strictly decodes one JSON request for an HTTP or MCP handler.
func DecodeBuildRequest(reader io.Reader) (BuildRequest, error) {
	if reader == nil {
		return BuildRequest{}, fmt.Errorf("%w: request body is required", ErrInvalidBuildRequest)
	}
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var request BuildRequest
	if err := decoder.Decode(&request); err != nil {
		return BuildRequest{}, fmt.Errorf("%w: decode JSON: %v", ErrInvalidBuildRequest, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return BuildRequest{}, fmt.Errorf("%w: request body must contain one JSON object", ErrInvalidBuildRequest)
	}
	request, err := ResolveBuildRequest(request)
	if err != nil {
		return BuildRequest{}, err
	}
	if err := ValidateRequest(request); err != nil {
		return BuildRequest{}, err
	}
	return request, nil
}

// ValidateRequest applies the curated-template boundary used by Builder.Build.
func ValidateRequest(request BuildRequest) error {
	if _, supported := templateCatalog[request.TemplateID]; !supported {
		return fmt.Errorf("%w: %q", ErrInvalidTemplate, request.TemplateID)
	}
	if request.WorkspaceID <= 0 {
		return fmt.Errorf("%w: workspaceId must be positive", ErrInvalidBuildRequest)
	}
	if err := validateDisplayText("appName", request.AppName, 80); err != nil {
		return err
	}
	if err := validateDisplayText("organizationName", request.OrganizationName, 120); err != nil {
		return err
	}
	if err := validateDisplayText("city", request.City, 80); err != nil {
		return err
	}
	if !request.Madhhab.Valid() {
		return fmt.Errorf("%w: madhhab %q is unsupported", ErrInvalidBuildRequest, request.Madhhab)
	}
	if !validSubjectID(request.Subject.ID) || request.Subject.UserID <= 0 {
		return fmt.Errorf("%w: subject id and positive userId are required", ErrInvalidBuildRequest)
	}
	if request.ExpiresAt.IsZero() {
		return fmt.Errorf("%w: expiresAt is required", ErrInvalidBuildRequest)
	}
	if request.Lifecycle != BundleLifecyclePreview && request.Lifecycle != BundleLifecyclePublished {
		return fmt.Errorf("%w: lifecycle must be preview or published", ErrInvalidBuildRequest)
	}
	if request.Theme.AccentColor != "" && !validHexColor(request.Theme.AccentColor) {
		return fmt.Errorf("%w: accentColor must be empty or a six-digit CSS hex color", ErrInvalidBuildRequest)
	}
	if _, err := CanonicalizeSuppliedComponents(request.TemplateID, request.Modules, request.Components); err != nil {
		return err
	}
	if len(request.AllowedOrigins.Surfaces) == 0 {
		return fmt.Errorf("%w: at least one approved surface origin is required", ErrInvalidBuildRequest)
	}
	if err := validateOriginList("allowedOrigins.surfaces", request.AllowedOrigins.Surfaces); err != nil {
		return err
	}
	if err := validateOriginList("allowedOrigins.embedders", request.AllowedOrigins.Embedders); err != nil {
		return err
	}
	if err := validateOriginList("allowedOrigins.connections", request.AllowedOrigins.Connections); err != nil {
		return err
	}
	if err := validateOriginList("allowedOrigins.resources", request.AllowedOrigins.Resources); err != nil {
		return err
	}
	return nil
}

func validSubjectID(value string) bool {
	if value == "" || len(value) > 128 || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if !((character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || strings.ContainsRune(":._-@", character)) {
			return false
		}
	}
	return true
}

func validateDisplayText(field, value string, maxRunes int) error {
	if value == "" || strings.TrimSpace(value) != value {
		return fmt.Errorf("%w: %s is required and must not have surrounding whitespace", ErrInvalidBuildRequest, field)
	}
	if !utf8.ValidString(value) {
		return fmt.Errorf("%w: %s must be valid UTF-8", ErrInvalidBuildRequest, field)
	}
	if utf8.RuneCountInString(value) > maxRunes {
		return fmt.Errorf("%w: %s exceeds %d characters", ErrInvalidBuildRequest, field, maxRunes)
	}
	if strings.Contains(value, "..") || strings.ContainsAny(value, `/\\<>`) {
		return fmt.Errorf("%w: %s contains unsafe path or markup characters", ErrInvalidBuildRequest, field)
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return fmt.Errorf("%w: %s contains control characters", ErrInvalidBuildRequest, field)
		}
	}
	return nil
}

func validHexColor(value string) bool {
	if len(value) != 7 || value[0] != '#' {
		return false
	}
	for _, character := range value[1:] {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') || (character >= 'A' && character <= 'F')) {
			return false
		}
	}
	return true
}

func validateModules(templateID string, modules []string) error {
	if len(modules) == 0 {
		return fmt.Errorf("%w: at least one module is required", ErrInvalidBuildRequest)
	}
	seen := make(map[string]struct{}, len(modules))
	template := templateCatalog[templateID]
	allowed := make(map[string]struct{}, len(template.AllowedModules))
	for _, moduleID := range template.AllowedModules {
		allowed[moduleID] = struct{}{}
	}
	for _, moduleID := range modules {
		if _, supported := moduleCatalog[moduleID]; !supported {
			return fmt.Errorf("%w: module %q is not in the curated catalog", ErrInvalidBuildRequest, moduleID)
		}
		if _, supported := allowed[moduleID]; !supported {
			return fmt.Errorf("%w: module %q is not supported by template %s", ErrInvalidBuildRequest, moduleID, templateID)
		}
		if _, duplicate := seen[moduleID]; duplicate {
			return fmt.Errorf("%w: duplicate module %q", ErrInvalidBuildRequest, moduleID)
		}
		seen[moduleID] = struct{}{}
	}
	return nil
}

func validateOriginList(field string, origins []string) error {
	seen := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		if origin == "" || strings.TrimSpace(origin) != origin {
			return fmt.Errorf("%w: %s contains an empty or padded origin", ErrInvalidBuildRequest, field)
		}
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
			return fmt.Errorf("%w: %s contains invalid origin %q", ErrInvalidBuildRequest, field, origin)
		}
		if parsed.Path == "/" || origin != parsed.Scheme+"://"+parsed.Host {
			return fmt.Errorf("%w: %s origin %q must be canonical and have no trailing slash", ErrInvalidBuildRequest, field, origin)
		}
		if parsed.Scheme != "https" && !(parsed.Scheme == "http" && localDevelopmentHost(parsed.Hostname())) {
			return fmt.Errorf("%w: %s origin %q must use HTTPS outside local development", ErrInvalidBuildRequest, field, origin)
		}
		if !validOriginHost(parsed.Hostname()) {
			return fmt.Errorf("%w: %s origin %q has an invalid or wildcard host", ErrInvalidBuildRequest, field, origin)
		}
		key := strings.ToLower(origin)
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("%w: %s contains duplicate origin %q", ErrInvalidBuildRequest, field, origin)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validOriginHost(host string) bool {
	if host == "" || strings.Contains(host, "*") || net.ParseIP(host) != nil {
		return host != "" && !strings.Contains(host, "*")
	}
	if len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if !((character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || character == '-') {
				return false
			}
		}
	}
	return true
}

func localDevelopmentHost(host string) bool {
	return strings.EqualFold(host, "localhost") || host == "127.0.0.1" || host == "::1"
}

func normalizedRequest(request BuildRequest) BuildRequest {
	copy := request
	copy.Theme.AccentColor = resolvedAccentColor(request.Theme.AccentColor)
	copy.Modules = append([]string(nil), request.Modules...)
	sort.Strings(copy.Modules)
	copy.Components = cloneComponents(request.Components)
	sortComponents(copy.Components)
	copy.AllowedOrigins.Surfaces = normalizedOrigins(request.AllowedOrigins.Surfaces)
	copy.AllowedOrigins.Embedders = normalizedOrigins(request.AllowedOrigins.Embedders)
	copy.AllowedOrigins.Connections = normalizedOrigins(request.AllowedOrigins.Connections)
	copy.AllowedOrigins.Resources = normalizedOrigins(request.AllowedOrigins.Resources)
	return copy
}

func normalizedOrigins(origins []string) []string {
	copy := append([]string(nil), origins...)
	sort.Strings(copy)
	return copy
}

func resolvedAccentColor(value string) string {
	if value == "" {
		return defaultAccentColor
	}
	return strings.ToUpper(value)
}
