package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"

	"taawun/pkg/artifacts"
	"taawun/pkg/models"
	"taawun/pkg/primitives"
)

func TestRelayTicketUsesAuthenticatedArtifactScopeAndOrigin(t *testing.T) {
	issuer := &relayTicketIssuerStub{}
	handler, err := NewRelayTicketHTTPHandler(issuer, relayTicketArtifactStub{bundle: relayTicketBundle()}, relayTicketWorkspaceStub{}, CurrentUser)
	if err != nil {
		t.Fatalf("NewRelayTicketHTTPHandler() error = %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/workspaces/7/relay-sessions", http.NoBody)
	request = mux.SetURLVars(request, map[string]string{"workspace_id": "7"})
	request = request.WithContext(WithCurrentUser(request.Context(), &models.User{ID: 31}))
	request.Header.Set("Origin", "https://preview.taawun.example")
	request.Body = ioNopCloser(`{"artifactId":"artifact-1","contentHash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","deviceId":"device-1","peerId":"peer-1"}`)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	handler.Issue(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("ticket status = %d body=%s", response.Code, response.Body.String())
	}
	if issuer.grant.PrincipalID != 31 || issuer.grant.WorkspaceID != 7 || issuer.grant.Origin != "https://preview.taawun.example" || issuer.grant.ArtifactID != "artifact-1" || issuer.grant.PeerID != "peer-1" || issuer.grant.DeviceID != "device-1" {
		t.Fatalf("trusted relay grant = %+v", issuer.grant)
	}
	if ttl := time.Until(issuer.grant.ExpiresAt); ttl <= 0 || ttl > defaultRelayTicketTTL+time.Second {
		t.Fatalf("grant expiry = %s", ttl)
	}
	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil || payload["ticket"] != "signed-ticket" || payload["protocol"] != primitives.RelayWebSocketSubprotocol {
		t.Fatalf("ticket response = %#v err=%v", payload, err)
	}
}

func TestRelayTicketDoesNotRevealCrossWorkspaceOrArtifactScope(t *testing.T) {
	issuer := &relayTicketIssuerStub{}
	handler, err := NewRelayTicketHTTPHandler(issuer, relayTicketArtifactStub{bundle: relayTicketBundle()}, relayTicketWorkspaceStub{err: errors.New("forbidden")}, CurrentUser)
	if err != nil {
		t.Fatalf("NewRelayTicketHTTPHandler() error = %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/workspaces/7/relay-sessions", ioNopCloser(`{"artifactId":"artifact-1","contentHash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","deviceId":"device-1","peerId":"peer-1"}`))
	request = mux.SetURLVars(request, map[string]string{"workspace_id": "7"})
	request = request.WithContext(WithCurrentUser(request.Context(), &models.User{ID: 32}))
	request.Header.Set("Origin", "https://preview.taawun.example")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	handler.Issue(response, request)

	if response.Code != http.StatusNotFound || issuer.called {
		t.Fatalf("cross-workspace ticket = status=%d called=%t body=%s", response.Code, issuer.called, response.Body.String())
	}
}

type relayTicketIssuerStub struct {
	grant  primitives.RelaySessionGrant
	called bool
}

func (s *relayTicketIssuerStub) IssueSession(grant primitives.RelaySessionGrant) (string, *primitives.RelaySession, error) {
	s.grant, s.called = grant, true
	return "signed-ticket", &primitives.RelaySession{ArtifactID: grant.ArtifactID, WorkspaceID: grant.WorkspaceID, PrincipalID: grant.PrincipalID, DeviceID: grant.DeviceID, PeerID: grant.PeerID, Origin: grant.Origin, ExpiresAt: grant.ExpiresAt.UnixMilli()}, nil
}

type relayTicketArtifactStub struct{ bundle artifacts.BuildResult }

func (s relayTicketArtifactStub) Open(context.Context, string) (artifacts.BuildResult, error) {
	return s.bundle, nil
}

type relayTicketWorkspaceStub struct{ err error }

func (s relayTicketWorkspaceStub) AuthorizeWorkspaceCapability(_ *models.User, id int, _ models.WorkspaceCapability) (*models.Workspace, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &models.Workspace{ID: id}, nil
}

func relayTicketBundle() artifacts.BuildResult {
	return artifacts.BuildResult{ArtifactID: "artifact-1", ContentHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Manifest: artifacts.Manifest{
		ArtifactID: "artifact-1", ContentHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", WorkspaceID: 7,
		Authorization: artifacts.BundleAuthorization{Subject: artifacts.BundleSubject{WorkspaceID: 7}, AllowedOrigins: artifacts.OriginPolicy{Surfaces: []string{"https://preview.taawun.example"}}},
	}}
}

func ioNopCloser(value string) io.ReadCloser { return io.NopCloser(strings.NewReader(value)) }
