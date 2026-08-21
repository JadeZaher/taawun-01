package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"reflect"
	"sort"
	"strings"
	"time"

	"taawun/pkg/artifacts"
)

const (
	artifactAPIBase          = "/api/artifacts"
	defaultArtifactBodyLimit = 64 << 10
)

// ArtifactAuthority is the trusted subject and verified-origin policy supplied upstream.
type ArtifactAuthority struct {
	WorkspaceID    int
	Subject        artifacts.SubjectBinding
	AllowedOrigins artifacts.OriginPolicy
}

// ArtifactAuthorityResolver reads authority established by upstream middleware.
type ArtifactAuthorityResolver func(*http.Request) (ArtifactAuthority, bool)

type artifactStore interface {
	Build(context.Context, artifacts.BuildRequest) (artifacts.BuildResult, error)
	Open(context.Context, string) (artifacts.BuildResult, error)
	ReadFile(context.Context, string, string) (artifacts.ArtifactFile, error)
	ReadVerifiedFile(context.Context, artifacts.BuildResult, string) (artifacts.ArtifactFile, error)
}

// ArtifactHTTPHandler exposes scoped build and immutable artifact read operations.
type ArtifactHTTPHandler struct {
	store            artifactStore
	authority        ArtifactAuthorityResolver
	maximumBodyBytes int64
	now              func() time.Time
}

// NewArtifactHTTPHandler constructs an adapter around an injected stable-signer builder.
func NewArtifactHTTPHandler(builder *artifacts.Builder, authority ArtifactAuthorityResolver) (*ArtifactHTTPHandler, error) {
	return NewArtifactHTTPHandlerWithClock(builder, authority, time.Now)
}

// NewArtifactHTTPHandlerWithClock binds signed file reads to the server clock.
func NewArtifactHTTPHandlerWithClock(builder *artifacts.Builder, authority ArtifactAuthorityResolver, now func() time.Time) (*ArtifactHTTPHandler, error) {
	if builder == nil || !builder.ProductionReady() {
		return nil, errors.New("production artifact builder with stable signer is required")
	}
	if authority == nil || now == nil {
		return nil, errors.New("artifact authority resolver and server clock are required")
	}
	return &ArtifactHTTPHandler{
		store:            builder,
		authority:        authority,
		maximumBodyBytes: defaultArtifactBodyLimit,
		now:              now,
	}, nil
}

// ServeHTTP routes the self-contained artifact API without imposing an application router.
func (h *ArtifactHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.store == nil || h.authority == nil || h.now == nil {
		writeArtifactError(w, http.StatusInternalServerError, "artifact_handler_unavailable", "Artifact service is unavailable.")
		return
	}
	if r.URL.RawQuery != "" {
		writeArtifactError(w, http.StatusBadRequest, "query_not_allowed", "Query parameters are not accepted by this endpoint.")
		return
	}
	authority, ok := h.authority(r)
	if !ok || authority.WorkspaceID <= 0 || authority.Subject.ID == "" || authority.Subject.UserID <= 0 {
		writeArtifactError(w, http.StatusForbidden, "workspace_scope_required", "A trusted workspace scope is required.")
		return
	}
	authority.AllowedOrigins = normalizedAuthorityOrigins(authority.AllowedOrigins)

	if r.URL.Path == artifactAPIBase {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeArtifactError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use POST to build an artifact.")
			return
		}
		h.build(w, r, authority)
		return
	}
	if !strings.HasPrefix(r.URL.Path, artifactAPIBase+"/") {
		writeArtifactError(w, http.StatusNotFound, "route_not_found", "Artifact endpoint not found.")
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", http.MethodGet+", "+http.MethodHead)
		writeArtifactError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use GET or HEAD to read an artifact.")
		return
	}
	h.read(w, r, authority)
}

func (h *ArtifactHTTPHandler) build(w http.ResponseWriter, r *http.Request, authority ArtifactAuthority) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeArtifactError(w, http.StatusUnsupportedMediaType, "content_type_required", "Content-Type must be application/json.")
		return
	}
	if r.ContentLength > h.maximumBodyBytes {
		writeArtifactError(w, http.StatusRequestEntityTooLarge, "request_too_large", "Artifact build request is too large.")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, h.maximumBodyBytes+1))
	if err != nil {
		writeArtifactError(w, http.StatusBadRequest, "invalid_request", "Could not read the artifact build request.")
		return
	}
	if int64(len(body)) > h.maximumBodyBytes {
		writeArtifactError(w, http.StatusRequestEntityTooLarge, "request_too_large", "Artifact build request is too large.")
		return
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var request artifacts.BuildRequest
	if err := decoder.Decode(&request); err != nil {
		writeArtifactError(w, http.StatusBadRequest, "invalid_build_request", "Artifact build request is invalid.")
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeArtifactError(w, http.StatusBadRequest, "invalid_build_request", "Artifact build request must contain one JSON object.")
		return
	}
	if request.WorkspaceID != authority.WorkspaceID {
		writeArtifactError(w, http.StatusForbidden, "workspace_mismatch", "Build request is outside the authorized workspace.")
		return
	}
	if request.Subject != (artifacts.SubjectBinding{}) && request.Subject != authority.Subject {
		writeArtifactError(w, http.StatusForbidden, "subject_mismatch", "Build request subject is outside the authorized principal.")
		return
	}
	if originPolicySupplied(request.AllowedOrigins) && !reflect.DeepEqual(request.AllowedOrigins, authority.AllowedOrigins) {
		writeArtifactError(w, http.StatusForbidden, "origin_policy_mismatch", "Build request contains an unverified origin policy.")
		return
	}
	request.Subject = authority.Subject
	request.AllowedOrigins = authority.AllowedOrigins
	request, err = artifacts.ResolveBuildRequest(request)
	if err != nil {
		writeArtifactValidationError(w, err)
		return
	}
	if err := artifacts.ValidateRequest(request); err != nil {
		writeArtifactValidationError(w, err)
		return
	}
	result, err := h.store.Build(r.Context(), request)
	if err != nil {
		h.writeStoreError(w, err)
		return
	}
	metadata := artifactMetadataFrom(result.Manifest)
	w.Header().Set("Location", metadata.URL)
	w.Header().Set("Cache-Control", "no-store")
	writeArtifactJSON(w, http.StatusCreated, artifactBuildResponse{Artifact: metadata})
}

func (h *ArtifactHTTPHandler) read(w http.ResponseWriter, r *http.Request, authority ArtifactAuthority) {
	requestNow := h.now().UTC()
	remainder := strings.TrimPrefix(r.URL.Path, artifactAPIBase+"/")
	segments := strings.Split(remainder, "/")
	if len(segments) == 0 || segments[0] == "" {
		writeArtifactError(w, http.StatusNotFound, "artifact_not_found", "Artifact not found.")
		return
	}
	contentHash := segments[0]
	result, err := h.store.Open(r.Context(), contentHash)
	if err != nil {
		h.writeStoreError(w, err)
		return
	}
	if result.Manifest.WorkspaceID != authority.WorkspaceID || result.Manifest.Authorization.Subject.ID != authority.Subject.ID || result.Manifest.Authorization.Subject.UserID != authority.Subject.UserID || !reflect.DeepEqual(result.Manifest.Authorization.AllowedOrigins, authority.AllowedOrigins) {
		writeArtifactError(w, http.StatusNotFound, "artifact_not_found", "Artifact not found.")
		return
	}

	switch {
	case len(segments) == 1:
		writeImmutableJSON(w, r, result.ContentHash, artifactMetadataFrom(result.Manifest))
	case len(segments) == 2 && segments[1] == "manifest.json":
		writeImmutableJSON(w, r, result.ContentHash, result.Manifest)
	case len(segments) >= 3 && segments[1] == "files":
		if err := artifacts.CheckManifestExpiry(result.Manifest, requestNow); err != nil {
			h.writeStoreError(w, err)
			return
		}
		relativePath := strings.Join(segments[2:], "/")
		if path.Clean(relativePath) != relativePath || relativePath == "." || strings.Contains(relativePath, "\\") {
			writeArtifactError(w, http.StatusNotFound, "artifact_file_not_found", "Artifact file not found.")
			return
		}
		file, err := h.store.ReadVerifiedFile(r.Context(), result, relativePath)
		if err != nil {
			h.writeStoreError(w, err)
			return
		}
		writeImmutableFile(w, r, file, result.Manifest)
	default:
		writeArtifactError(w, http.StatusNotFound, "route_not_found", "Artifact endpoint not found.")
	}
}

func (h *ArtifactHTTPHandler) writeStoreError(w http.ResponseWriter, err error) {
	var componentError *artifacts.ComponentValidationError
	if errors.As(err, &componentError) {
		writeArtifactValidationError(w, err)
		return
	}
	switch {
	case errors.Is(err, artifacts.ErrInvalidBuildRequest), errors.Is(err, artifacts.ErrInvalidTemplate), errors.Is(err, artifacts.ErrInvalidContentHash):
		writeArtifactError(w, http.StatusBadRequest, "invalid_artifact_request", "Artifact request is invalid.")
	case errors.Is(err, artifacts.ErrArtifactNotFound), errors.Is(err, artifacts.ErrArtifactFileNotFound):
		writeArtifactError(w, http.StatusNotFound, "artifact_not_found", "Artifact or file not found.")
	case errors.Is(err, artifacts.ErrArtifactConflict):
		writeArtifactError(w, http.StatusConflict, "artifact_integrity_error", "Artifact failed integrity verification.")
	case errors.Is(err, artifacts.ErrArtifactExpired):
		writeArtifactError(w, http.StatusGone, "artifact_expired", "Artifact authorization has expired.")
	case errors.Is(err, artifacts.ErrInvalidSignature), errors.Is(err, artifacts.ErrUntrustedSigner):
		writeArtifactError(w, http.StatusConflict, "artifact_signature_error", "Artifact signature could not be verified.")
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		writeArtifactError(w, http.StatusRequestTimeout, "request_cancelled", "Artifact operation was cancelled.")
	default:
		writeArtifactError(w, http.StatusInternalServerError, "artifact_operation_failed", "Artifact operation failed.")
	}
}

func writeArtifactValidationError(w http.ResponseWriter, err error) {
	var componentError *artifacts.ComponentValidationError
	if errors.As(err, &componentError) {
		details := componentError.SafeDetails()
		writeArtifactJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": map[string]any{
			"code": "invalid_component_document", "message": "A component document is not valid curated data.",
			"details": map[string]string{"componentId": details.ComponentID, "key": details.Key, "reason": details.Reason},
		}})
		return
	}
	writeArtifactError(w, http.StatusBadRequest, "invalid_build_request", "Artifact build request is invalid.")
}

type artifactBuildResponse struct {
	Artifact artifactMetadata `json:"artifact"`
}

type artifactMetadata struct {
	ArtifactID  string                     `json:"artifactId"`
	ContentHash string                     `json:"contentHash"`
	CreatedAt   time.Time                  `json:"createdAt"`
	WorkspaceID int                        `json:"workspaceId"`
	Template    artifacts.TemplateIdentity `json:"template"`
	ModuleIDs   []string                   `json:"moduleIds"`
	URL         string                     `json:"url"`
	ManifestURL string                     `json:"manifestUrl"`
	FileBaseURL string                     `json:"fileBaseUrl"`
}

func artifactMetadataFrom(manifest artifacts.Manifest) artifactMetadata {
	modules := make([]string, 0, len(manifest.Modules))
	for _, module := range manifest.Modules {
		modules = append(modules, module.ID)
	}
	baseURL := artifactAPIBase + "/" + manifest.ContentHash
	return artifactMetadata{
		ArtifactID:  manifest.ArtifactID,
		ContentHash: manifest.ContentHash,
		CreatedAt:   manifest.CreatedAt,
		WorkspaceID: manifest.WorkspaceID,
		Template:    manifest.Template,
		ModuleIDs:   modules,
		URL:         baseURL,
		ManifestURL: baseURL + "/manifest.json",
		FileBaseURL: baseURL + "/files/",
	}
}

type artifactErrorResponse struct {
	Error artifactErrorBody `json:"error"`
}

type artifactErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeArtifactError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Cache-Control", "no-store")
	writeArtifactJSON(w, status, artifactErrorResponse{Error: artifactErrorBody{Code: code, Message: message}})
}

func writeArtifactJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeImmutableJSON(w http.ResponseWriter, r *http.Request, etag string, value any) {
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	w.Header().Set("ETag", `"`+etag+`"`)
	if requestETagMatches(r, etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if r.Method == http.MethodHead {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		return
	}
	writeArtifactJSON(w, http.StatusOK, value)
}

func writeImmutableFile(w http.ResponseWriter, r *http.Request, file artifacts.ArtifactFile, manifest artifacts.Manifest) {
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	w.Header().Set("ETag", `"`+file.SHA256+`"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	switch path.Ext(file.Path) {
	case ".css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case ".html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if policy, ok := artifactCSPForFile(manifest, file.Path); ok {
			w.Header().Set("Content-Security-Policy", artifactCSPHeader(policy))
		}
	case ".js":
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	default:
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	}
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(file.Contents)))
	if requestETagMatches(r, file.SHA256) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(file.Contents)
	}
}

func originPolicySupplied(policy artifacts.OriginPolicy) bool {
	return len(policy.Surfaces)+len(policy.Embedders)+len(policy.Connections)+len(policy.Resources) > 0
}

func normalizedAuthorityOrigins(policy artifacts.OriginPolicy) artifacts.OriginPolicy {
	copyAndSort := func(values []string) []string {
		copy := append([]string(nil), values...)
		sort.Strings(copy)
		return copy
	}
	return artifacts.OriginPolicy{
		Surfaces: copyAndSort(policy.Surfaces), Embedders: copyAndSort(policy.Embedders),
		Connections: copyAndSort(policy.Connections), Resources: copyAndSort(policy.Resources),
	}
}

func artifactCSPForFile(manifest artifacts.Manifest, filePath string) (artifacts.CSPPolicy, bool) {
	mode := artifacts.RenderModeStandalone
	if filePath == "embed.html" || strings.HasPrefix(filePath, "cards/") {
		mode = artifacts.RenderModeEmbed
	}
	for _, entry := range manifest.Security.CSP {
		if entry.RenderMode == mode {
			return entry.Policy, true
		}
	}
	return artifacts.CSPPolicy{}, false
}

func artifactCSPHeader(policy artifacts.CSPPolicy) string {
	directives := []struct {
		name   string
		values []string
	}{
		{"default-src", policy.DefaultSrc}, {"script-src", policy.ScriptSrc}, {"style-src", policy.StyleSrc},
		{"connect-src", policy.ConnectSrc}, {"img-src", policy.ImageSrc}, {"font-src", policy.FontSrc},
		{"frame-ancestors", policy.FrameAncestors}, {"object-src", policy.ObjectSrc}, {"base-uri", policy.BaseURI},
		{"form-action", policy.FormAction},
	}
	parts := make([]string, 0, len(directives))
	for _, directive := range directives {
		if len(directive.values) > 0 {
			parts = append(parts, directive.name+" "+strings.Join(directive.values, " "))
		}
	}
	return strings.Join(parts, "; ")
}

func requestETagMatches(r *http.Request, etag string) bool {
	for _, candidate := range strings.Split(r.Header.Get("If-None-Match"), ",") {
		if strings.TrimSpace(candidate) == `"`+etag+`"` {
			return true
		}
	}
	return false
}
