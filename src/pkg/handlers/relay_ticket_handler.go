package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"taawun/pkg/artifacts"
	"taawun/pkg/models"
	"taawun/pkg/primitives"
)

const (
	maximumRelayTicketBodyBytes = 16 << 10
	defaultRelayTicketTTL       = 2 * time.Minute
)

// RelayTicketIssuer signs a relay grant only after this adapter establishes authority.
type RelayTicketIssuer interface {
	IssueSession(primitives.RelaySessionGrant) (string, *primitives.RelaySession, error)
}

type relayTicketArtifactReader interface {
	Open(context.Context, string) (artifacts.BuildResult, error)
}

type relayTicketWorkspaceAuthorizer interface {
	AuthorizeWorkspaceCapability(*models.User, int, models.WorkspaceCapability) (*models.Workspace, error)
}

// RelayTicketHTTPHandler issues one-use relay credentials to current workspace members.
type RelayTicketHTTPHandler struct {
	issuer      RelayTicketIssuer
	artifacts   relayTicketArtifactReader
	workspaces  relayTicketWorkspaceAuthorizer
	currentUser currentPrincipal
	now         func() time.Time
}

// NewRelayTicketHTTPHandler creates an authenticated control-plane relay-ticket adapter.
func NewRelayTicketHTTPHandler(issuer RelayTicketIssuer, store relayTicketArtifactReader, workspaces relayTicketWorkspaceAuthorizer, currentUser func(context.Context) (*models.User, bool)) (*RelayTicketHTTPHandler, error) {
	if issuer == nil || store == nil || workspaces == nil || currentUser == nil {
		return nil, errors.New("relay ticket issuer, artifact store, workspace authority, and current-user resolver are required")
	}
	return &RelayTicketHTTPHandler{issuer: issuer, artifacts: store, workspaces: workspaces, currentUser: currentUser, now: time.Now}, nil
}

type relayTicketInput struct {
	ArtifactID  string `json:"artifactId"`
	ContentHash string `json:"contentHash"`
	DeviceID    string `json:"deviceId"`
	PeerID      string `json:"peerId"`
}

// Issue derives every relay claim from the authenticated request and a verified manifest.
func (h *RelayTicketHTTPHandler) Issue(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.currentUser(r.Context())
	if !ok || actor == nil || actor.ID <= 0 {
		writeRelayTicketError(w, http.StatusUnauthorized, "authentication_required", "Authentication is required.")
		return
	}
	workspaceID, err := strconv.Atoi(mux.Vars(r)["workspace_id"])
	if err != nil || workspaceID <= 0 {
		writeRelayTicketError(w, http.StatusNotFound, "relay_artifact_not_found", "Artifact was not found.")
		return
	}
	if _, err := h.workspaces.AuthorizeWorkspaceCapability(actor, workspaceID, models.WorkspaceCapabilityView); err != nil {
		writeRelayTicketError(w, http.StatusNotFound, "relay_artifact_not_found", "Artifact was not found.")
		return
	}
	var input relayTicketInput
	if !decodeRelayTicketJSON(w, r, &input) {
		return
	}
	origin, err := exactRequestOrigin(r.Header.Get("Origin"))
	if err != nil {
		writeRelayTicketError(w, http.StatusForbidden, "relay_origin_not_allowed", "The current origin is not approved for this artifact.")
		return
	}
	bundle, err := h.artifacts.Open(r.Context(), input.ContentHash)
	if err != nil || bundle.ContentHash != input.ContentHash || bundle.ArtifactID != input.ArtifactID || bundle.Manifest.WorkspaceID != workspaceID || bundle.Manifest.Authorization.Subject.WorkspaceID != workspaceID || !containsOrigin(bundle.Manifest.Authorization.AllowedOrigins.Surfaces, origin) {
		writeRelayTicketError(w, http.StatusNotFound, "relay_artifact_not_found", "Artifact was not found.")
		return
	}
	now := h.now().UTC()
	ticket, session, err := h.issuer.IssueSession(primitives.RelaySessionGrant{
		ArtifactID: input.ArtifactID, WorkspaceID: workspaceID, PrincipalID: actor.ID,
		DeviceID: input.DeviceID, PeerID: input.PeerID, Origin: origin,
		ExpiresAt: now.Add(defaultRelayTicketTTL),
	})
	if err != nil || session == nil {
		writeRelayTicketError(w, http.StatusBadRequest, "relay_ticket_invalid", "Relay ticket request is invalid.")
		return
	}
	writeRelayTicketJSON(w, http.StatusCreated, map[string]any{
		"ticket": ticket, "protocol": primitives.RelayWebSocketSubprotocol,
		"artifactId": session.ArtifactID, "workspaceId": session.WorkspaceID,
		"peerId": session.PeerID, "deviceId": session.DeviceID,
		"expiresAt": time.UnixMilli(session.ExpiresAt).UTC(),
	})
}

func decodeRelayTicketJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maximumRelayTicketBodyBytes))
	if err != nil {
		writeRelayTicketError(w, http.StatusRequestEntityTooLarge, "request_too_large", "Request body is too large.")
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeRelayTicketError(w, http.StatusBadRequest, "invalid_request", "Request body must be one valid JSON object.")
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeRelayTicketError(w, http.StatusBadRequest, "invalid_request", "Request body must contain one JSON object.")
		return false
	}
	return true
}

func exactRequestOrigin(value string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("invalid request origin")
	}
	return strings.ToLower(parsed.Scheme + "://" + parsed.Host), nil
}

func containsOrigin(origins []string, wanted string) bool {
	for _, origin := range origins {
		if origin == wanted {
			return true
		}
	}
	return false
}

func writeRelayTicketError(w http.ResponseWriter, status int, code, message string) {
	writeRelayTicketJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func writeRelayTicketJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
