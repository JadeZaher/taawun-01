package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"path"
	"strings"

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
	Resume(context.Context, *models.User, string, int64) (*conductor.Track, error)
	RequestPublication(context.Context, *models.User, string, int64, string) (*conductor.Track, error)
	ActivatePublication(context.Context, *models.User, string, int64) (*conductor.Track, error)
	GetTrack(context.Context, *models.User, string) (*conductor.Track, error)
	Events(context.Context, *models.User, string) ([]conductor.TrackEvent, error)
}

type compositionArtifactReader interface {
	ReadFile(context.Context, string, string) (artifacts.ArtifactFile, error)
}

type currentPrincipal func(context.Context) (*models.User, bool)

// CompositionHTTPHandler translates the central builder journey into trusted service calls.
type CompositionHTTPHandler struct {
	service        CompositionService
	artifacts      compositionArtifactReader
	currentUser    currentPrincipal
	previewOrigins []string
}

// NewCompositionHTTPHandler requires a service that independently enforces workspace authority.
func NewCompositionHTTPHandler(service CompositionService, store compositionArtifactReader, currentUser func(context.Context) (*models.User, bool), previewOrigins []string) (*CompositionHTTPHandler, error) {
	if service == nil || store == nil || currentUser == nil || len(previewOrigins) == 0 {
		return nil, errors.New("composition service, artifact store, current-user resolver, and preview origins are required")
	}
	return &CompositionHTTPHandler{
		service: service, artifacts: store, currentUser: currentUser,
		previewOrigins: append([]string(nil), previewOrigins...),
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
	writeCompositionJSON(w, http.StatusOK, map[string]any{"modules": artifacts.ListModules()})
}

type previewCompositionInput struct {
	IdempotencyKey   string                 `json:"idempotencyKey,omitempty"`
	WorkspaceID      int                    `json:"workspaceId"`
	AppName          string                 `json:"appName"`
	OrganizationName string                 `json:"organizationName"`
	City             string                 `json:"city"`
	Madhhab          ethics.Madhhab         `json:"madhhab"`
	TemplateID       string                 `json:"templateId"`
	Theme            artifacts.ThemeRequest `json:"theme"`
	Modules          []string               `json:"modules"`
	RequestedOrigins artifacts.OriginPolicy `json:"requestedOrigins"`
	TTLHours         int                    `json:"ttlHours,omitempty"`
}

// Preview composes a signed preview under the authenticated actor and workspace capability.
func (h *CompositionHTTPHandler) Preview(w http.ResponseWriter, r *http.Request) {
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
		Modules: append([]string(nil), input.Modules...), RequestedOrigins: input.RequestedOrigins, TTLHours: input.TTLHours,
	})
	if err != nil {
		writeCompositionServiceError(w, err)
		return
	}
	if result == nil || result.Track == nil || result.Track.Preview == nil || result.Track.Artifact == nil {
		writeCompositionError(w, http.StatusConflict, "preview_not_ready", "The composition did not reach a signed preview.")
		return
	}
	preview := compositionPreviewURLs(result.Track)
	writeCompositionJSON(w, http.StatusCreated, map[string]any{
		"track": result.Track, "created": result.Created, "manifest": result.Track.Artifact.Manifest,
		"preview": preview, "previewUrl": preview.DocumentURL,
	})
}

// GetTrack returns a workspace-authorized composition track.
func (h *CompositionHTTPHandler) GetTrack(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	track, err := h.service.GetTrack(r.Context(), actor, mux.Vars(r)["track_id"])
	if err != nil {
		writeCompositionServiceError(w, err)
		return
	}
	writeCompositionJSON(w, http.StatusOK, track)
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
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	track, err := h.service.GetTrack(r.Context(), actor, mux.Vars(r)["track_id"])
	if err != nil {
		writeCompositionServiceError(w, err)
		return
	}
	if track.Preview == nil || track.Preview.ContentHash == "" {
		writeCompositionError(w, http.StatusNotFound, "preview_not_found", "Preview files are not available for this track.")
		return
	}
	filePath := mux.Vars(r)["path"]
	if filePath == "" || path.Clean(filePath) != filePath || strings.Contains(filePath, "\\") {
		writeCompositionError(w, http.StatusNotFound, "preview_file_not_found", "Preview file was not found.")
		return
	}
	file, err := h.artifacts.ReadFile(r.Context(), track.Preview.ContentHash, filePath)
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
	switch {
	case errors.Is(err, conductor.ErrWorkspaceForbidden):
		writeCompositionError(w, http.StatusForbidden, "workspace_forbidden", "This account is not authorized for that workspace operation.")
	case errors.Is(err, conductor.ErrTrackNotFound):
		writeCompositionError(w, http.StatusNotFound, "track_not_found", "Composition track was not found.")
	case errors.Is(err, conductor.ErrInvalidComposition):
		writeCompositionError(w, http.StatusUnprocessableEntity, "invalid_composition", "The composition is not a valid curated request.")
	case errors.Is(err, conductor.ErrTrackVersionConflict), errors.Is(err, conductor.ErrIdempotencyConflict), errors.Is(err, conductor.ErrTrackTransition):
		writeCompositionError(w, http.StatusConflict, "composition_conflict", err.Error())
	case errors.Is(err, conductor.ErrDependencyUnavailable):
		writeCompositionError(w, http.StatusBadGateway, "composition_dependency_unavailable", "A required composition dependency is unavailable.")
	default:
		writeCompositionError(w, http.StatusInternalServerError, "composition_operation_failed", "The composition operation could not be completed.")
	}
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
