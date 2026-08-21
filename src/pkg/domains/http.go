package domains

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gorilla/mux"

	"taawun/pkg/models"
)

const maximumClaimBodyBytes = 4096

type CurrentUserFunc func(context.Context) (*models.User, bool)

type HTTPHandler struct {
	service     *Service
	contexts    *PublicationContextService
	currentUser CurrentUserFunc
}

func NewHTTPHandler(service *Service, currentUser CurrentUserFunc) (*HTTPHandler, error) {
	if service == nil || currentUser == nil {
		return nil, errors.New("domain service and current-user resolver are required")
	}
	return &HTTPHandler{service: service, currentUser: currentUser}, nil
}

// NewHTTPHandlerWithPublicationContext exposes the bounded publication audit view.
func NewHTTPHandlerWithPublicationContext(service *Service, contexts *PublicationContextService, currentUser CurrentUserFunc) (*HTTPHandler, error) {
	if service == nil || contexts == nil || currentUser == nil {
		return nil, errors.New("domain service, publication context service, and current-user resolver are required")
	}
	return &HTTPHandler{service: service, contexts: contexts, currentUser: currentUser}, nil
}

func (h *HTTPHandler) Claim(w http.ResponseWriter, r *http.Request) {
	actor, workspaceID, ok := h.authority(w, r)
	if !ok {
		return
	}
	if mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mediaType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "content_type_required", "Content-Type must be application/json.")
		return
	}
	var input struct {
		Origin string `json:"origin"`
	}
	if err := decodeStrictJSON(w, r, &input); err != nil {
		return
	}
	result, err := h.service.ClaimOrigin(r.Context(), actor, workspaceID, input.Origin)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *HTTPHandler) List(w http.ResponseWriter, r *http.Request) {
	actor, workspaceID, ok := h.authority(w, r)
	if !ok {
		return
	}
	claims, err := h.service.List(r.Context(), actor, workspaceID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"claims": claims})
}

func (h *HTTPHandler) Inspect(w http.ResponseWriter, r *http.Request) {
	actor, workspaceID, ok := h.authority(w, r)
	if !ok {
		return
	}
	claim, err := h.service.Inspect(r.Context(), actor, workspaceID, mux.Vars(r)["claim_id"])
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, claim)
}

func (h *HTTPHandler) Verify(w http.ResponseWriter, r *http.Request) {
	actor, workspaceID, ok := h.authority(w, r)
	if !ok {
		return
	}
	claim, err := h.service.Verify(r.Context(), actor, workspaceID, mux.Vars(r)["claim_id"])
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, claim)
}

func (h *HTTPHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	actor, workspaceID, ok := h.authority(w, r)
	if !ok {
		return
	}
	claim, err := h.service.Revoke(r.Context(), actor, workspaceID, mux.Vars(r)["claim_id"])
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, claim)
}

func (h *HTTPHandler) Publish(w http.ResponseWriter, r *http.Request) {
	actor, workspaceID, ok := h.authority(w, r)
	if !ok {
		return
	}
	if mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mediaType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "content_type_required", "Content-Type must be application/json.")
		return
	}
	var input struct {
		ContentHash string `json:"contentHash"`
	}
	if err := decodeStrictJSON(w, r, &input); err != nil {
		return
	}
	publication, err := h.service.Publish(r.Context(), actor, workspaceID, mux.Vars(r)["claim_id"], input.ContentHash)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, publication)
}

func (h *HTTPHandler) PublicationHistory(w http.ResponseWriter, r *http.Request) {
	actor, workspaceID, ok := h.authority(w, r)
	if !ok {
		return
	}
	if h.contexts == nil {
		writeError(w, http.StatusInternalServerError, "domain_service_error", "Domain operation failed.")
		return
	}
	query, err := publicationContextQuery(r)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	page, err := h.contexts.Page(r.Context(), actor, workspaceID, mux.Vars(r)["claim_id"], query)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func publicationContextQuery(r *http.Request) (PublicationContextQuery, error) {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return PublicationContextQuery{}, ErrInvalidPublicationQuery
	}
	for key, entries := range values {
		if key != "limit" && key != "cursor" && key != "publicationId" || len(entries) != 1 || entries[0] == "" {
			return PublicationContextQuery{}, ErrInvalidPublicationQuery
		}
	}
	publicationID := values.Get("publicationId")
	if publicationID != "" {
		if values.Has("limit") || values.Has("cursor") {
			return PublicationContextQuery{}, ErrInvalidPublicationQuery
		}
		return PublicationContextQuery{PublicationID: publicationID}, nil
	}
	limit := DefaultPublicationContextLimit
	if values.Has("limit") {
		parsed, err := strconv.Atoi(values.Get("limit"))
		if err != nil || parsed < 1 || parsed > MaximumPublicationContextLimit {
			return PublicationContextQuery{}, ErrInvalidPublicationQuery
		}
		limit = parsed
	}
	return PublicationContextQuery{Limit: limit, Cursor: values.Get("cursor")}, nil
}

func (h *HTTPHandler) Activate(w http.ResponseWriter, r *http.Request) {
	actor, workspaceID, ok := h.authority(w, r)
	if !ok {
		return
	}
	publication, err := h.service.Activate(r.Context(), actor, workspaceID, mux.Vars(r)["claim_id"], mux.Vars(r)["publication_id"])
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, publication)
}

func (h *HTTPHandler) authority(w http.ResponseWriter, r *http.Request) (*models.User, int, bool) {
	actor, ok := h.currentUser(r.Context())
	if !ok || actor == nil || actor.ID <= 0 {
		writeError(w, http.StatusUnauthorized, "authentication_required", "Authentication is required.")
		return nil, 0, false
	}
	workspaceID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil || workspaceID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_workspace", "Workspace ID must be positive.")
		return nil, 0, false
	}
	return actor, workspaceID, true
}

func decodeStrictJSON(w http.ResponseWriter, r *http.Request, target any) error {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maximumClaimBodyBytes))
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "request_too_large", "Request body is too large.")
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be one valid JSON object.")
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must contain one JSON object.")
		return errors.New("trailing JSON data")
	}
	return nil
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		writeError(w, http.StatusForbidden, "workspace_forbidden", "Workspace owner or administrator access is required.")
	case errors.Is(err, ErrInvalidOrigin):
		writeError(w, http.StatusBadRequest, "invalid_origin", "Use one exact public HTTPS origin without a path, wildcard, or non-default port.")
	case errors.Is(err, ErrClaimNotFound):
		writeError(w, http.StatusNotFound, "claim_not_found", "Domain claim was not found.")
	case errors.Is(err, ErrOriginClaimed), errors.Is(err, ErrAlreadyVerified), errors.Is(err, ErrInvalidState):
		writeError(w, http.StatusConflict, "claim_conflict", err.Error())
	case errors.Is(err, ErrChallengeExpired):
		writeError(w, http.StatusGone, "challenge_expired", "DNS challenge has expired; create a new claim.")
	case errors.Is(err, ErrDNSProofNotFound):
		writeError(w, http.StatusUnprocessableEntity, "dns_proof_missing", "The expected DNS TXT value was not found.")
	case errors.Is(err, ErrOriginNotVerified):
		writeError(w, http.StatusConflict, "origin_not_verified", "The exact origin is not currently verified.")
	case errors.Is(err, ErrArtifactInvalid):
		writeError(w, http.StatusUnprocessableEntity, "artifact_not_publishable", "The signed artifact is invalid, expired, or not bound to this workspace and domain.")
	case errors.Is(err, ErrPublicationMissing):
		writeError(w, http.StatusNotFound, "publication_not_found", "Publication was not found.")
	case errors.Is(err, ErrInvalidPublicationQuery):
		writeError(w, http.StatusBadRequest, "invalid_publication_query", "Use a bounded history cursor or one exact publicationId selection.")
	default:
		writeError(w, http.StatusInternalServerError, "domain_service_error", "Domain operation failed.")
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
