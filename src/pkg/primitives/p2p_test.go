package primitives

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

const relayTestOrigin = "https://app.taawun.community"

func newRelayTestHub(t *testing.T) *P2PRelayHub {
	t.Helper()
	hub, err := NewP2PRelayHubWithConfig(RelayConfig{
		SharedSecret:   []byte("0123456789abcdef0123456789abcdef"),
		AllowedOrigins: []string{relayTestOrigin, "https://other.taawun.community"},
	})
	if err != nil {
		t.Fatalf("create hub: %v", err)
	}
	return hub
}

func issueRelayTestSession(t *testing.T, hub *P2PRelayHub) (string, *RelaySession) {
	t.Helper()
	token, session, err := hub.IssueSession(RelaySessionGrant{
		ArtifactID:  "ramadan-board",
		WorkspaceID: 42,
		PrincipalID: 7,
		DeviceID:    "device-42",
		PeerID:      "member-42",
		Origin:      relayTestOrigin,
		ExpiresAt:   time.Now().Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("issue session: %v", err)
	}
	return token, session
}

func TestRelaySessionBindsAuthenticatedGrant(t *testing.T) {
	hub := newRelayTestHub(t)
	token, issued := issueRelayTestSession(t, hub)

	verified, err := hub.VerifySession(token)
	if err != nil {
		t.Fatalf("verify valid session: %v", err)
	}
	if verified.SessionID != issued.SessionID || verified.ArtifactID != "ramadan-board" || verified.WorkspaceID != 42 || verified.PrincipalID != 7 || verified.DeviceID != "device-42" || verified.PeerID != "member-42" || verified.Origin != relayTestOrigin {
		t.Fatalf("relay claims were not preserved: %#v", verified)
	}
	if _, err := hub.VerifySession(token + "tampered"); err == nil {
		t.Fatal("expected a tampered session to be rejected")
	}
	if _, _, err := hub.IssueSession(RelaySessionGrant{
		ArtifactID: "ramadan-board", WorkspaceID: 42, PrincipalID: 7, DeviceID: "device-42", PeerID: "member-42",
		Origin: "https://attacker.example", ExpiresAt: time.Now().Add(time.Minute),
	}); err == nil {
		t.Fatal("expected an unconfigured origin to be rejected")
	}
	if _, _, err := hub.IssueSession(RelaySessionGrant{
		ArtifactID: "ramadan-board", WorkspaceID: 42, PrincipalID: 7, DeviceID: "device-42", PeerID: "member-42",
		Origin: relayTestOrigin, ExpiresAt: time.Now().Add(6 * time.Minute),
	}); err == nil {
		t.Fatal("expected an overlong session to be rejected")
	}
}

func TestRelayRejectsQueryAndAuthorizationCredentials(t *testing.T) {
	hub := newRelayTestHub(t)
	token, _ := issueRelayTestSession(t, hub)

	query := httptest.NewRequest(http.MethodGet, "/api/p2p/stream?artifactId=ramadan-board&peerId=member-42&token="+token, nil)
	query.Header.Set("Origin", relayTestOrigin)
	queryResponse := httptest.NewRecorder()
	hub.HandleP2PStream(queryResponse, query)
	if queryResponse.Code != http.StatusBadRequest {
		t.Fatalf("expected query credential to return 400, got %d", queryResponse.Code)
	}

	authorization := httptest.NewRequest(http.MethodGet, "/api/p2p/stream?artifactId=ramadan-board&peerId=member-42", nil)
	authorization.Header.Set("Origin", relayTestOrigin)
	authorization.Header.Set("Authorization", "Bearer "+token)
	authorizationResponse := httptest.NewRecorder()
	hub.HandleP2PStream(authorizationResponse, authorization)
	if authorizationResponse.Code != http.StatusBadRequest {
		t.Fatalf("expected Authorization credential to return 400, got %d", authorizationResponse.Code)
	}
}

func TestRelayUsesSubprotocolTicketAndConsumesItOnce(t *testing.T) {
	hub := newRelayTestHub(t)
	token, _ := issueRelayTestSession(t, hub)
	server := httptest.NewServer(http.HandlerFunc(hub.HandleP2PStream))
	defer server.Close()

	streamURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/p2p/stream?artifactId=ramadan-board&peerId=member-42"
	dialer := websocket.Dialer{Subprotocols: []string{RelayWebSocketSubprotocol, token}}
	headers := http.Header{"Origin": []string{relayTestOrigin}}
	connection, response, err := dialer.Dial(streamURL, headers)
	if err != nil {
		if response != nil {
			t.Fatalf("open authenticated relay connection: %v (status %d)", err, response.StatusCode)
		}
		t.Fatalf("open authenticated relay connection: %v", err)
	}
	if connection.Subprotocol() != RelayWebSocketSubprotocol {
		t.Fatalf("expected relay subprotocol %q, got %q", RelayWebSocketSubprotocol, connection.Subprotocol())
	}
	_ = connection.Close()

	second, retryResponse, retryErr := dialer.Dial(streamURL, headers)
	if second != nil {
		_ = second.Close()
	}
	if retryErr == nil {
		t.Fatal("expected a reused relay ticket to be rejected")
	}
	if retryResponse == nil || retryResponse.StatusCode != http.StatusUnauthorized {
		status := 0
		if retryResponse != nil {
			status = retryResponse.StatusCode
		}
		t.Fatalf("expected reused ticket to return 401, got %d (%v)", status, retryErr)
	}
}

func TestRelayRejectsConnectionWithDifferentOrigin(t *testing.T) {
	hub := newRelayTestHub(t)
	token, _ := issueRelayTestSession(t, hub)
	req := httptest.NewRequest(http.MethodGet, "/api/p2p/stream?artifactId=ramadan-board&peerId=member-42", nil)
	req.Header.Set("Origin", "https://other.taawun.community")
	req.Header.Set("Sec-WebSocket-Protocol", RelayWebSocketSubprotocol+", "+token)
	response := httptest.NewRecorder()
	hub.HandleP2PStream(response, req)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected mismatched origin to return 401, got %d", response.Code)
	}
}

func TestRelayRejectsUnsignedConnection(t *testing.T) {
	hub := newRelayTestHub(t)
	req := httptest.NewRequest(http.MethodGet, "/api/p2p/stream?artifactId=ramadan-board&peerId=member-42", nil)
	response := httptest.NewRecorder()
	hub.HandleP2PStream(response, req)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected unsigned connection to return 401, got %d", response.Code)
	}
}

func TestRelayHonorsOriginAllowList(t *testing.T) {
	hub := newRelayTestHub(t)
	allowed := httptest.NewRequest(http.MethodGet, "/api/p2p/stream", nil)
	allowed.Header.Set("Origin", relayTestOrigin)
	if !hub.isOriginAllowed(allowed) {
		t.Fatal("expected configured origin to be allowed")
	}
	blocked := httptest.NewRequest(http.MethodGet, "/api/p2p/stream", nil)
	blocked.Header.Set("Origin", "https://attacker.example")
	if hub.isOriginAllowed(blocked) {
		t.Fatal("expected unconfigured origin to be rejected")
	}
}
