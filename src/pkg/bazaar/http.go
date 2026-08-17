package bazaar

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/gorilla/mux"

	"taawun/pkg/models"
)

const maximumBazaarBodyBytes = 64 << 10

type CurrentUserFunc func(context.Context) (*models.User, bool)

type HTTPHandler struct {
	service     *Service
	currentUser CurrentUserFunc
}

func NewHTTPHandler(service *Service, currentUser CurrentUserFunc) (*HTTPHandler, error) {
	if service == nil || currentUser == nil {
		return nil, errors.New("Bazaar service and current-user resolver are required")
	}
	return &HTTPHandler{service: service, currentUser: currentUser}, nil
}

// RegisterRoutes attaches the lean public catalog and authenticated lifecycle API.
func RegisterRoutes(router *mux.Router, authenticate func(http.Handler) http.Handler, handler *HTTPHandler) error {
	if router == nil || authenticate == nil || handler == nil {
		return errors.New("Bazaar router, authentication middleware, and handler are required")
	}
	router.HandleFunc("/api/bazaar/listings", handler.PublicList).Methods(http.MethodGet)
	router.HandleFunc("/api/bazaar/listings/{listing_id}", handler.PublicDetail).Methods(http.MethodGet)
	router.HandleFunc("/api/bazaar/listings/{listing_id}/test-drive", handler.TestDrive).Methods(http.MethodGet)
	router.Handle("/api/bazaar/listings", authenticate(http.HandlerFunc(handler.Create))).Methods(http.MethodPost)
	router.Handle("/api/bazaar/listings/{listing_id}", authenticate(http.HandlerFunc(handler.Update))).Methods(http.MethodPut)
	router.Handle("/api/bazaar/listings/{listing_id}/transitions", authenticate(http.HandlerFunc(handler.Transition))).Methods(http.MethodPost)
	router.Handle("/api/bazaar/listings/{listing_id}/events", authenticate(http.HandlerFunc(handler.Events))).Methods(http.MethodGet)
	router.Handle("/api/bazaar/listings/{listing_id}/purchases", authenticate(http.HandlerFunc(handler.Purchase))).Methods(http.MethodPost)
	router.Handle("/api/bazaar/purchases/{purchase_id}/refresh", authenticate(http.HandlerFunc(handler.RefreshPurchase))).Methods(http.MethodPost)
	router.Handle("/api/bazaar/entitlements/{entitlement_id}/install", authenticate(http.HandlerFunc(handler.Install))).Methods(http.MethodPost)
	return nil
}

func (h *HTTPHandler) PublicList(w http.ResponseWriter, r *http.Request) {
	listings, err := h.service.PublicList(r.Context())
	if err != nil {
		writeBazaarServiceError(w, err)
		return
	}
	writeBazaarJSON(w, http.StatusOK, map[string]any{"listings": listings})
}

func (h *HTTPHandler) PublicDetail(w http.ResponseWriter, r *http.Request) {
	listing, err := h.service.PublicDetail(r.Context(), mux.Vars(r)["listing_id"])
	if err != nil {
		writeBazaarServiceError(w, err)
		return
	}
	writeBazaarJSON(w, http.StatusOK, listing)
}

func (h *HTTPHandler) TestDrive(w http.ResponseWriter, r *http.Request) {
	listing, err := h.service.PublicDetail(r.Context(), mux.Vars(r)["listing_id"])
	if err != nil {
		writeBazaarServiceError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, listing.Revision.PublicTestDriveURL, http.StatusTemporaryRedirect)
}

func (h *HTTPHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	var input DraftInput
	if !decodeBazaarJSON(w, r, &input) {
		return
	}
	listing, err := h.service.CreateDraft(r.Context(), actor, input)
	if err != nil {
		writeBazaarServiceError(w, err)
		return
	}
	writeBazaarJSON(w, http.StatusCreated, listing)
}

func (h *HTTPHandler) Update(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	var input struct {
		ExpectedVersion int64      `json:"expectedVersion"`
		Draft           DraftInput `json:"draft"`
	}
	if !decodeBazaarJSON(w, r, &input) {
		return
	}
	listing, err := h.service.UpdateDraft(r.Context(), actor, mux.Vars(r)["listing_id"], input.ExpectedVersion, input.Draft)
	if err != nil {
		writeBazaarServiceError(w, err)
		return
	}
	writeBazaarJSON(w, http.StatusOK, listing)
}

func (h *HTTPHandler) Transition(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	var input TransitionInput
	if !decodeBazaarJSON(w, r, &input) {
		return
	}
	listing, err := h.service.Transition(r.Context(), actor, mux.Vars(r)["listing_id"], input)
	if err != nil {
		writeBazaarServiceError(w, err)
		return
	}
	writeBazaarJSON(w, http.StatusOK, listing)
}

func (h *HTTPHandler) Events(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	events, err := h.service.Events(r.Context(), actor, mux.Vars(r)["listing_id"])
	if err != nil {
		writeBazaarServiceError(w, err)
		return
	}
	writeBazaarJSON(w, http.StatusOK, map[string]any{"events": events})
}

func (h *HTTPHandler) Purchase(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	var input struct {
		TargetWorkspaceID int    `json:"targetWorkspaceId"`
		IdempotencyKey    string `json:"idempotencyKey"`
	}
	if !decodeBazaarJSON(w, r, &input) {
		return
	}
	purchase, err := h.service.Purchase(r.Context(), actor, mux.Vars(r)["listing_id"], input.TargetWorkspaceID, input.IdempotencyKey)
	if err != nil {
		writeBazaarServiceError(w, err)
		return
	}
	writeBazaarJSON(w, http.StatusCreated, purchase)
}

func (h *HTTPHandler) RefreshPurchase(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	purchase, err := h.service.RefreshPurchase(r.Context(), actor, mux.Vars(r)["purchase_id"])
	if err != nil && !errors.Is(err, ErrSettlementPending) {
		writeBazaarServiceError(w, err)
		return
	}
	status := http.StatusOK
	if errors.Is(err, ErrSettlementPending) {
		status = http.StatusAccepted
	}
	writeBazaarJSON(w, status, purchase)
}

func (h *HTTPHandler) Install(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	var input struct {
		ExpectedVersion int64 `json:"expectedVersion"`
	}
	if !decodeBazaarJSON(w, r, &input) {
		return
	}
	installation, err := h.service.Install(r.Context(), actor, mux.Vars(r)["entitlement_id"], input.ExpectedVersion)
	if err != nil {
		writeBazaarServiceError(w, err)
		return
	}
	writeBazaarJSON(w, http.StatusCreated, installation)
}

func (h *HTTPHandler) actor(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
	actor, ok := h.currentUser(r.Context())
	if !ok || actor == nil || actor.ID <= 0 {
		writeBazaarError(w, http.StatusUnauthorized, "authentication_required", "Authentication is required.")
		return nil, false
	}
	return actor, true
}

func decodeBazaarJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeBazaarError(w, http.StatusUnsupportedMediaType, "content_type_required", "Content-Type must be application/json.")
		return false
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maximumBazaarBodyBytes))
	if err != nil {
		writeBazaarError(w, http.StatusRequestEntityTooLarge, "request_too_large", "Request body is too large.")
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeBazaarError(w, http.StatusBadRequest, "invalid_request", "Request body must be one valid JSON object.")
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeBazaarError(w, http.StatusBadRequest, "invalid_request", "Request body must contain one JSON object.")
		return false
	}
	return true
}

func writeBazaarServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		writeBazaarError(w, http.StatusForbidden, "bazaar_forbidden", "This account is not authorized for that workspace operation.")
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrNotEntitled), errors.Is(err, ErrNotPublished):
		writeBazaarError(w, http.StatusNotFound, "bazaar_not_found", "Bazaar record was not found.")
	case errors.Is(err, ErrVersionConflict), errors.Is(err, ErrInvalidTransition), errors.Is(err, ErrIdempotencyConflict):
		writeBazaarError(w, http.StatusConflict, "bazaar_conflict", err.Error())
	case errors.Is(err, ErrSettlementPending):
		writeBazaarError(w, http.StatusAccepted, "settlement_pending", "Marketplace settlement has not been reconciled.")
	case errors.Is(err, ErrInvalid):
		writeBazaarError(w, http.StatusUnprocessableEntity, "invalid_bazaar_request", "The listing or transaction does not satisfy Bazaar policy.")
	default:
		writeBazaarError(w, http.StatusInternalServerError, "bazaar_operation_failed", "Bazaar operation failed.")
	}
}

func writeBazaarError(w http.ResponseWriter, status int, code, message string) {
	writeBazaarJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func writeBazaarJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
