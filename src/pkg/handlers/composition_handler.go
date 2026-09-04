package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"path"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"taawun/pkg/artifacts"
	"taawun/pkg/conductor"
	"taawun/pkg/ethics"
	"taawun/pkg/models"
)

const maximumCompositionBodyBytes = 64 << 10

// CompositionService is the authenticated control-plane workflow used by the builder UI.
type CompositionService interface {
	Compose(context.Context, *models.User, conductor.CompositionRequest) (*conductor.ComposeResult, error)
	Reissue(context.Context, *models.User, string, int64, string, int) (*conductor.ComposeResult, error)
	Resume(context.Context, *models.User, string, int64) (*conductor.Track, error)
	RequestPublication(context.Context, *models.User, string, int64, string) (*conductor.Track, error)
	ActivatePublication(context.Context, *models.User, string, int64) (*conductor.Track, error)
	ListTracks(context.Context, *models.User, int, int, string) (conductor.TrackSummaryPage, error)
	GetTrack(context.Context, *models.User, string) (*conductor.Track, error)
	Events(context.Context, *models.User, string) ([]conductor.TrackEvent, error)
}

type reissueCompositionInput struct {
	ExpectedVersion int64  `json:"expectedVersion"`
	IdempotencyKey  string `json:"idempotencyKey"`
	TTLHours        int    `json:"ttlHours,omitempty"`
}

type compositionArtifactReader interface {
	Open(context.Context, string) (artifacts.BuildResult, error)
	ReadFile(context.Context, string, string) (artifacts.ArtifactFile, error)
	ReadVerifiedFile(context.Context, artifacts.BuildResult, string) (artifacts.ArtifactFile, error)
}

type currentPrincipal func(context.Context) (*models.User, bool)

// CompositionHTTPHandler translates the central builder journey into trusted service calls.
type CompositionHTTPHandler struct {
	service        CompositionService
	artifacts      compositionArtifactReader
	currentUser    currentPrincipal
	previewOrigins []string
	logger         *slog.Logger
	now            func() time.Time
}

// NewCompositionHTTPHandler requires a service that independently enforces workspace authority.
func NewCompositionHTTPHandler(service CompositionService, store compositionArtifactReader, currentUser func(context.Context) (*models.User, bool), previewOrigins []string) (*CompositionHTTPHandler, error) {
	return NewCompositionHTTPHandlerWithClock(service, store, currentUser, previewOrigins, time.Now)
}

// NewCompositionHTTPHandlerWithClock derives every response from the server clock.
func NewCompositionHTTPHandlerWithClock(service CompositionService, store compositionArtifactReader, currentUser func(context.Context) (*models.User, bool), previewOrigins []string, now func() time.Time) (*CompositionHTTPHandler, error) {
	if service == nil || store == nil || currentUser == nil || len(previewOrigins) == 0 || now == nil {
		return nil, errors.New("composition service, artifact store, current-user resolver, and preview origins are required")
	}
	return &CompositionHTTPHandler{
		service: service, artifacts: store, currentUser: currentUser,
		previewOrigins: append([]string(nil), previewOrigins...), logger: slog.Default(), now: now,
	}, nil
}

// Templates exposes only the curated catalog; it never accepts client-defined templates.
func (h *CompositionHTTPHandler) Templates(w http.ResponseWriter, r *http.Request) {
	templates := artifacts.ListTemplates()
	response := make([]struct {
		ID             string                     `json:"id"`
		Version        string                     `json:"version"`
		Template       artifacts.TemplateIdentity `json:"template"`
		Title          string                     `json:"title"`
		Description    string                     `json:"description"`
		AllowedModules []string                   `json:"allowedModules"`
		Slots          []string                   `json:"slots"`
	}, 0, len(templates))
	for _, entry := range templates {
		response = append(response, struct {
			ID             string                     `json:"id"`
			Version        string                     `json:"version"`
			Template       artifacts.TemplateIdentity `json:"template"`
			Title          string                     `json:"title"`
			Description    string                     `json:"description"`
			AllowedModules []string                   `json:"allowedModules"`
			Slots          []string                   `json:"slots"`
		}{
			ID: entry.Identity.ID, Version: entry.Identity.Version, Template: entry.Identity,
			Title: entry.Title, Description: entry.Description,
			AllowedModules: append([]string(nil), entry.AllowedModules...), Slots: append([]string(nil), entry.Slots...),
		})
	}
	writeCompositionJSON(w, http.StatusOK, map[string]any{"templates": response})
}

// Modules exposes the fixed primitive contracts available to curated templates.
func (h *CompositionHTTPHandler) Modules(w http.ResponseWriter, r *http.Request) {
	writeCompositionJSON(w, http.StatusOK, map[string]any{"modules": artifacts.ListModules(), "componentDocumentPolicy": artifacts.ComponentPolicy()})
}

type previewCompositionInput struct {
	IdempotencyKey   string                        `json:"idempotencyKey,omitempty"`
	WorkspaceID      int                           `json:"workspaceId"`
	AppName          string                        `json:"appName"`
	OrganizationName string                        `json:"organizationName"`
	City             string                        `json:"city"`
	Madhhab          ethics.Madhhab                `json:"madhhab"`
	TemplateID       string                        `json:"templateId"`
	Theme            artifacts.ThemeRequest        `json:"theme"`
	Modules          []string                      `json:"modules"`
	Components       []artifacts.ComponentInstance `json:"components,omitempty"`
	RequestedOrigins artifacts.OriginPolicy        `json:"requestedOrigins"`
	TTLHours         int                           `json:"ttlHours,omitempty"`
}

// Preview composes a signed preview under the authenticated actor and workspace capability.
func (h *CompositionHTTPHandler) Preview(w http.ResponseWriter, r *http.Request) {
	requestNow := h.now().UTC()
	requestID := compositionRequestID(r)
	w.Header().Set("X-Request-ID", requestID)
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	if mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mediaType != "application/json" {
		writeCompositionError(w, http.StatusUnsupportedMediaType, "content_type_required", "Content-Type must be application/json.")
		return
	}
	var input previewCompositionInput
	if !decodeCompositionJSON(w, r, &input) {
		return
	}
	if input.IdempotencyKey == "" {
		key, err := compositionID("preview")
		if err != nil {
			writeCompositionError(w, http.StatusInternalServerError, "composition_unavailable", "Could not create a preview request.")
			return
		}
		input.IdempotencyKey = key
	}
	if input.TTLHours == 0 {
		input.TTLHours = 24
	}
	if len(input.RequestedOrigins.Surfaces) == 0 {
		input.RequestedOrigins.Surfaces = append([]string(nil), h.previewOrigins...)
	}
	result, err := h.service.Compose(r.Context(), actor, conductor.CompositionRequest{
		IdempotencyKey: input.IdempotencyKey, WorkspaceID: input.WorkspaceID,
		AppName: input.AppName, OrganizationName: input.OrganizationName, City: input.City,
		Madhhab: input.Madhhab, TemplateID: input.TemplateID, Theme: input.Theme,
		Modules: append([]string(nil), input.Modules...), Components: cloneComponentInputs(input.Components), RequestedOrigins: input.RequestedOrigins, TTLHours: input.TTLHours,
	})
	if err != nil {
		if errors.Is(err, conductor.ErrPreviewOriginDenied) {
			h.logPreviewOutcome(requestID, "denied", "origin_not_verified", http.StatusUnprocessableEntity)
		}
		writeCompositionServiceError(w, err)
		return
	}
	if result == nil || result.Track == nil || result.Track.Preview == nil || result.Track.Artifact == nil {
		writeCompositionError(w, http.StatusConflict, "preview_not_ready", "The composition did not reach a signed preview.")
		return
	}
	opened, err := h.artifacts.Open(r.Context(), result.Track.Artifact.ContentHash)
	if err != nil || opened.ArtifactID != result.Track.Artifact.ArtifactID || opened.ContentHash != result.Track.Artifact.ContentHash || opened.Manifest.WorkspaceID != result.Track.WorkspaceID || result.Track.Preview.ContentHash != opened.ContentHash || !verifiedTrackComponentBinding(result.Track, opened) {
		writeCompositionError(w, http.StatusConflict, "preview_integrity_error", "The signed preview could not be verified.")
		return
	}
	response, err := verifiedCompositionResponse(result.Track, result.Created, opened, requestNow)
	if err != nil {
		writeCompositionError(w, http.StatusInternalServerError, "composition_unavailable", "The signed preview receipt could not be prepared.")
		return
	}
	writeCompositionJSON(w, http.StatusCreated, response)
}

func (h *CompositionHTTPHandler) logPreviewOutcome(requestID, outcome, reason string, status int) {
	logger := h.logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Info("composition_preview_outcome",
		slog.String("request_id", requestID),
		slog.String("outcome", outcome),
		slog.String("reason", reason),
		slog.Int("status", status),
	)
}

func compositionRequestID(r *http.Request) string {
	if r != nil {
		host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
		if err == nil {
			peer, parseErr := netip.ParseAddr(host)
			candidate := strings.TrimSpace(r.Header.Get("X-Railway-Request-Id"))
			if parseErr == nil && trustedRailwayProxyPeer(peer.Unmap()) && safeCorrelationID(candidate) {
				return candidate
			}
		}
	}
	requestID, err := compositionID("request")
	if err != nil {
		return "request-unavailable"
	}
	return requestID
}

func safeCorrelationID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || strings.ContainsRune("-_.", character) {
			continue
		}
		return false
	}
	return true
}

// ListTracks returns a bounded, redacted workspace history for recovery.
func (h *CompositionHTTPHandler) ListTracks(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	query := r.URL.Query()
	for key, values := range query {
		if (key != "workspaceId" && key != "limit" && key != "cursor") || len(values) != 1 {
			writeCompositionError(w, http.StatusBadRequest, "invalid_track_query", "The track list query is not valid.")
			return
		}
	}
	workspaceID, valid := positiveDecimal(query.Get("workspaceId"))
	if !valid {
		writeCompositionError(w, http.StatusBadRequest, "invalid_workspace", "Workspace ID must be a positive integer.")
		return
	}
	limit := conductor.DefaultTrackListLimit
	if values, present := query["limit"]; present {
		parsed, valid := positiveDecimal(values[0])
		if !valid {
			writeCompositionError(w, http.StatusBadRequest, "invalid_limit", "Limit must be a positive integer.")
			return
		}
		limit = parsed
		if limit > conductor.MaximumTrackListLimit {
			limit = conductor.MaximumTrackListLimit
		}
	}
	cursor := query.Get("cursor")
	if _, present := query["cursor"]; present && cursor == "" {
		writeCompositionError(w, http.StatusBadRequest, "invalid_cursor", "Track cursor is not valid.")
		return
	}
	page, err := h.service.ListTracks(r.Context(), actor, workspaceID, limit, cursor)
	if err != nil {
		writeCompositionServiceError(w, err)
		return
	}
	writeCompositionJSON(w, http.StatusOK, page)
}

func positiveDecimal(value string) (int, bool) {
	if value == "" || len(value) > 10 {
		return 0, false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return 0, false
		}
	}
	parsed, err := strconv.Atoi(value)
	return parsed, err == nil && parsed > 0
}

// GetTrack returns a workspace-authorized composition track.
func (h *CompositionHTTPHandler) GetTrack(w http.ResponseWriter, r *http.Request) {
	requestNow := h.now().UTC()
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	track, err := h.service.GetTrack(r.Context(), actor, mux.Vars(r)["track_id"])
	if err != nil {
		writeCompositionServiceError(w, err)
		return
	}
	if r.URL.Query().Get("includeVerifiedPreview") != "true" {
		writeCompositionJSON(w, http.StatusOK, track)
		return
	}
	if track.Preview == nil || track.Artifact == nil {
		writeCompositionError(w, http.StatusConflict, "preview_not_ready", "The composition does not have a verified preview.")
		return
	}
	opened, err := h.artifacts.Open(r.Context(), track.Artifact.ContentHash)
	if err != nil || opened.ArtifactID != track.Artifact.ArtifactID || opened.ContentHash != track.Artifact.ContentHash || opened.Manifest.WorkspaceID != track.WorkspaceID || track.Preview.ContentHash != opened.ContentHash || !verifiedTrackComponentBinding(track, opened) {
		writeCompositionError(w, http.StatusConflict, "preview_integrity_error", "The signed preview could not be verified.")
		return
	}
	response, err := verifiedCompositionResponse(track, false, opened, requestNow)
	if err != nil {
		writeCompositionError(w, http.StatusInternalServerError, "composition_unavailable", "The signed preview receipt could not be prepared.")
		return
	}
	writeCompositionJSON(w, http.StatusOK, response)
}

// Events returns the append-only history for a workspace-authorized composition track.
func (h *CompositionHTTPHandler) Events(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	events, err := h.service.Events(r.Context(), actor, mux.Vars(r)["track_id"])
	if err != nil {
		writeCompositionServiceError(w, err)
		return
	}
	writeCompositionJSON(w, http.StatusOK, map[string]any{"events": events})
}

// Reissue signs an existing curated composition into a new immutable preview track.
func (h *CompositionHTTPHandler) Reissue(w http.ResponseWriter, r *http.Request) {
	requestNow := h.now().UTC()
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	if mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mediaType != "application/json" {
		writeCompositionError(w, http.StatusUnsupportedMediaType, "content_type_required", "Content-Type must be application/json.")
		return
	}
	var input reissueCompositionInput
	if !decodeCompositionJSON(w, r, &input) {
		return
	}
	if input.IdempotencyKey == "" {
		writeCompositionError(w, http.StatusBadRequest, "idempotency_key_required", "A stable idempotency key is required so this re-sign request can be retried safely.")
		return
	}
	if input.TTLHours == 0 {
		input.TTLHours = 24
	}
	result, err := h.service.Reissue(r.Context(), actor, mux.Vars(r)["track_id"], input.ExpectedVersion, input.IdempotencyKey, input.TTLHours)
	if err != nil {
		writeCompositionServiceError(w, err)
		return
	}
	if result == nil || result.Track == nil || result.Track.Preview == nil || result.Track.Artifact == nil {
		writeCompositionError(w, http.StatusConflict, "preview_not_ready", "The reissued composition did not reach a signed preview.")
		return
	}
	opened, err := h.artifacts.Open(r.Context(), result.Track.Artifact.ContentHash)
	if err != nil || opened.ArtifactID != result.Track.Artifact.ArtifactID || opened.ContentHash != result.Track.Artifact.ContentHash ||
		opened.Manifest.WorkspaceID != result.Track.WorkspaceID || result.Track.Preview.ContentHash != opened.ContentHash || !verifiedTrackComponentBinding(result.Track, opened) {
		writeCompositionError(w, http.StatusConflict, "preview_integrity_error", "The reissued signed preview could not be verified.")
		return
	}
	response, err := verifiedCompositionResponse(result.Track, result.Created, opened, requestNow)
	if err != nil {
		writeCompositionError(w, http.StatusInternalServerError, "composition_unavailable", "The signed preview receipt could not be prepared.")
		return
	}
	writeCompositionJSON(w, http.StatusCreated, response)
}

// Resume retries a durable partially completed composition with optimistic version control.
func (h *CompositionHTTPHandler) Resume(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	var input struct {
		ExpectedVersion int64 `json:"expectedVersion"`
	}
	if !decodeCompositionJSON(w, r, &input) {
		return
	}
	track, err := h.service.Resume(r.Context(), actor, mux.Vars(r)["track_id"], input.ExpectedVersion)
	if err != nil {
		writeCompositionServiceError(w, err)
		return
	}
	writeCompositionJSON(w, http.StatusOK, track)
}

// RequestPublication links a signed preview to a verified-domain claim without activating it.
func (h *CompositionHTTPHandler) RequestPublication(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	var input struct {
		ExpectedVersion int64  `json:"expectedVersion"`
		ClaimID         string `json:"claimId"`
	}
	if !decodeCompositionJSON(w, r, &input) {
		return
	}
	track, err := h.service.RequestPublication(r.Context(), actor, mux.Vars(r)["track_id"], input.ExpectedVersion, input.ClaimID)
	if err != nil {
		writeCompositionServiceError(w, err)
		return
	}
	writeCompositionJSON(w, http.StatusOK, track)
}

// ActivatePublication sends the requested track through the verified-domain publisher.
func (h *CompositionHTTPHandler) ActivatePublication(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	var input struct {
		ExpectedVersion int64 `json:"expectedVersion"`
	}
	if !decodeCompositionJSON(w, r, &input) {
		return
	}
	track, err := h.service.ActivatePublication(r.Context(), actor, mux.Vars(r)["track_id"], input.ExpectedVersion)
	if err != nil {
		writeCompositionServiceError(w, err)
		return
	}
	writeCompositionJSON(w, http.StatusCreated, track)
}

// PreviewFile provides an authenticated, track-scoped read of a manifest-listed preview file.
func (h *CompositionHTTPHandler) PreviewFile(w http.ResponseWriter, r *http.Request) {
	requestNow := h.now().UTC()
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	track, err := h.service.GetTrack(r.Context(), actor, mux.Vars(r)["track_id"])
	if err != nil {
		writeCompositionServiceError(w, err)
		return
	}
	if track.Preview == nil || track.Preview.ContentHash == "" || track.Artifact == nil {
		writeCompositionError(w, http.StatusNotFound, "preview_not_found", "Preview files are not available for this track.")
		return
	}
	filePath := mux.Vars(r)["path"]
	if filePath == "" || path.Clean(filePath) != filePath || strings.Contains(filePath, "\\") {
		writeCompositionError(w, http.StatusNotFound, "preview_file_not_found", "Preview file was not found.")
		return
	}
	opened, err := h.artifacts.Open(r.Context(), track.Preview.ContentHash)
	if err != nil || opened.ArtifactID != track.Artifact.ArtifactID || opened.ContentHash != track.Artifact.ContentHash ||
		opened.Manifest.WorkspaceID != track.WorkspaceID || track.Preview.ContentHash != opened.ContentHash ||
		!verifiedTrackComponentBinding(track, opened) {
		writeCompositionError(w, http.StatusConflict, "preview_integrity_error", "Preview file integrity could not be verified.")
		return
	}
	if errors.Is(artifacts.CheckManifestExpiry(opened.Manifest, requestNow), artifacts.ErrArtifactExpired) {
		writeCompositionError(w, http.StatusGone, "preview_authorization_expired", "Preview authorization has expired; signed runtime files are not available.")
		return
	}
	file, err := h.artifacts.ReadVerifiedFile(r.Context(), opened, filePath)
	if err != nil {
		if errors.Is(err, artifacts.ErrArtifactFileNotFound) || errors.Is(err, artifacts.ErrArtifactNotFound) {
			writeCompositionError(w, http.StatusNotFound, "preview_file_not_found", "Preview file was not found.")
			return
		}
		writeCompositionError(w, http.StatusConflict, "preview_integrity_error", "Preview file integrity could not be verified.")
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	switch path.Ext(file.Path) {
	case ".html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case ".css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case ".js":
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	default:
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	}
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(file.Contents)
	}
}

type previewURLs struct {
	DocumentURL string `json:"documentUrl"`
	ThemeURL    string `json:"themeUrl"`
	StylesURL   string `json:"stylesUrl"`
}

func compositionPreviewURLs(track *conductor.Track) previewURLs {
	base := "/api/conductor/tracks/" + track.ID + "/preview/files/"
	return previewURLs{DocumentURL: base + "index.html", ThemeURL: base + "theme.css", StylesURL: base + "app.css"}
}

func (h *CompositionHTTPHandler) actor(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
	actor, ok := h.currentUser(r.Context())
	if !ok || actor == nil || actor.ID <= 0 {
		writeCompositionError(w, http.StatusUnauthorized, "authentication_required", "Authentication is required.")
		return nil, false
	}
	return actor, true
}

func decodeCompositionJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maximumCompositionBodyBytes))
	if err != nil {
		writeCompositionError(w, http.StatusRequestEntityTooLarge, "request_too_large", "Request body is too large.")
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeCompositionError(w, http.StatusBadRequest, "invalid_request", "Request body must be one valid JSON object.")
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeCompositionError(w, http.StatusBadRequest, "invalid_request", "Request body must contain one JSON object.")
		return false
	}
	return true
}

func compositionID(prefix string) (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return prefix + "-" + hex.EncodeToString(value), nil
}

func writeCompositionServiceError(w http.ResponseWriter, err error) {
	var componentError *artifacts.ComponentValidationError
	if errors.As(err, &componentError) {
		details := componentError.SafeDetails()
		writeCompositionJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": map[string]any{
			"code": "invalid_composition", "message": "A component document is not valid curated data.",
			"details": map[string]string{"componentId": details.ComponentID, "key": details.Key, "reason": details.Reason},
		}})
		return
	}
	switch {
	case errors.Is(err, conductor.ErrWorkspaceForbidden):
		writeCompositionError(w, http.StatusForbidden, "workspace_forbidden", "This account is not authorized for that workspace operation.")
	case errors.Is(err, conductor.ErrTrackNotFound):
		writeCompositionError(w, http.StatusNotFound, "track_not_found", "Composition track was not found.")
	case errors.Is(err, conductor.ErrInvalidTrackQuery):
		writeCompositionError(w, http.StatusBadRequest, "invalid_track_query", "The track list query is not valid.")
	case errors.Is(err, conductor.ErrInvalidComposition):
		writeCompositionError(w, http.StatusUnprocessableEntity, "invalid_composition", "The composition is not a valid curated request.")
	case errors.Is(err, conductor.ErrTrackVersionConflict), errors.Is(err, conductor.ErrIdempotencyConflict), errors.Is(err, conductor.ErrTrackTransition):
		writeCompositionError(w, http.StatusConflict, "composition_conflict", err.Error())
	case errors.Is(err, conductor.ErrPublicationClaimState):
		writeCompositionError(w, http.StatusConflict, "publication_claim_unavailable", "The domain claim must be currently verified before publication can continue.")
	case errors.Is(err, conductor.ErrDependencyUnavailable):
		writeCompositionError(w, http.StatusBadGateway, "composition_dependency_unavailable", "A required composition dependency is unavailable.")
	default:
		writeCompositionError(w, http.StatusInternalServerError, "composition_operation_failed", "The composition operation could not be completed.")
	}
}

func cloneComponentInputs(components []artifacts.ComponentInstance) []artifacts.ComponentInstance {
	if components == nil {
		return nil
	}
	cloned := make([]artifacts.ComponentInstance, len(components))
	for index, component := range components {
		cloned[index] = component
		cloned[index].Data = append(json.RawMessage(nil), component.Data...)
	}
	return cloned
}

func verifiedCompositionResponse(track *conductor.Track, created bool, opened artifacts.BuildResult, serverTime time.Time) (map[string]any, error) {
	if len(opened.ManifestJSON) == 0 {
		return nil, errors.New("exact manifest bytes are unavailable")
	}
	manifestDigest := sha256.Sum256(opened.ManifestJSON)
	authorizationState := "active"
	if errors.Is(artifacts.CheckManifestExpiry(opened.Manifest, serverTime), artifacts.ErrArtifactExpired) {
		authorizationState = "expired"
	}
	preview := compositionPreviewURLs(track)
	return map[string]any{
		"track": track, "created": created, "manifest": opened.Manifest,
		"verification": map[string]any{
			"status": "verified", "verified": true,
			"artifactId": opened.Manifest.ArtifactID, "contentHash": opened.Manifest.ContentHash, "workspaceId": opened.Manifest.WorkspaceID,
			"signatureAlgorithm": opened.Manifest.Signature.Algorithm, "signerKeyId": opened.Manifest.Signature.KeyID, "signatureValue": opened.Manifest.Signature.Value,
			"manifestDigest": hex.EncodeToString(manifestDigest[:]), "manifestJson": string(opened.ManifestJSON),
			"authorizationState": authorizationState, "serverTime": serverTime.UTC(),
		},
		"preview": preview, "previewUrl": preview.DocumentURL,
	}, nil
}

func verifiedTrackComponentBinding(track *conductor.Track, opened artifacts.BuildResult) bool {
	if track == nil || track.BuildRequest == nil || track.Artifact == nil || track.Preview == nil {
		return false
	}
	storedManifest := track.Artifact.Manifest
	if storedManifest.ContractVersion == artifacts.ManifestContractVersionV1 || opened.Manifest.ContractVersion == artifacts.ManifestContractVersionV1 {
		return verifiedLegacyTrackBinding(track, opened)
	}
	expected, err := artifacts.ResolveBuildRequest(artifacts.BuildRequest{
		TemplateID: track.Request.TemplateID,
		Modules:    append([]string(nil), track.Request.Modules...),
		Components: cloneComponentInputs(track.Request.Components),
	})
	if err != nil || track.BuildRequest.TemplateID != track.Request.TemplateID || !reflect.DeepEqual(track.BuildRequest.Modules, expected.Modules) || !reflect.DeepEqual(track.BuildRequest.Components, expected.Components) {
		return false
	}
	if artifacts.ValidateManifestComponents(storedManifest) != nil || artifacts.ValidateManifestComponents(opened.Manifest) != nil || storedManifest.Template.ID != track.BuildRequest.TemplateID || !manifestComponentsMatchBuild(storedManifest, *track.BuildRequest) {
		return false
	}
	return reflect.DeepEqual(opened.Manifest.Template, storedManifest.Template) &&
		reflect.DeepEqual(opened.Manifest.Modules, storedManifest.Modules) &&
		reflect.DeepEqual(opened.Manifest.Components, storedManifest.Components)
}

func verifiedLegacyTrackBinding(track *conductor.Track, opened artifacts.BuildResult) bool {
	build := track.BuildRequest
	stored := track.Artifact.Manifest
	reopened := opened.Manifest
	if build == nil || track.Request.Components != nil || build.Components != nil || stored.Components != nil || reopened.Components != nil {
		return false
	}
	if stored.ContractVersion != artifacts.ManifestContractVersionV1 || reopened.ContractVersion != artifacts.ManifestContractVersionV1 ||
		stored.Signature.ContractVersion != artifacts.SignatureContractVersionV1 || reopened.Signature.ContractVersion != artifacts.SignatureContractVersionV1 ||
		stored.Authorization.Version != artifacts.SignatureContractVersionV1 || reopened.Authorization.Version != artifacts.SignatureContractVersionV1 {
		return false
	}
	if artifacts.ValidateManifestComponents(stored) != nil || artifacts.ValidateManifestComponents(reopened) != nil ||
		build.TemplateID != track.Request.TemplateID || stored.Template.ID != build.TemplateID ||
		!reflect.DeepEqual(track.Request.Modules, build.Modules) || !manifestModulesMatchBuild(stored, *build) {
		return false
	}
	return reflect.DeepEqual(reopened.Template, stored.Template) && reflect.DeepEqual(reopened.Modules, stored.Modules)
}

func manifestModulesMatchBuild(manifest artifacts.Manifest, build artifacts.BuildRequest) bool {
	if len(manifest.Modules) != len(build.Modules) {
		return false
	}
	for index, module := range manifest.Modules {
		if module.ID != build.Modules[index] {
			return false
		}
	}
	return true
}

func manifestComponentsMatchBuild(manifest artifacts.Manifest, build artifacts.BuildRequest) bool {
	if !manifestModulesMatchBuild(manifest, build) || len(manifest.Components) != len(build.Components) {
		return false
	}
	for index, component := range manifest.Components {
		buildComponent := build.Components[index]
		if component.ID != buildComponent.ID || component.Type != buildComponent.Type || !reflect.DeepEqual(component.Data, buildComponent.Data) {
			return false
		}
	}
	return true
}

func writeCompositionError(w http.ResponseWriter, status int, code, message string) {
	writeCompositionJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func writeCompositionJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
