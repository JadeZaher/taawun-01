package primitives

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestRelaySessionBindsPeerAndArtifact(t *testing.T) {
	hub, err := NewP2PRelayHubWithConfig(RelayConfig{SharedSecret: []byte("0123456789abcdef0123456789abcdef")})
	if err != nil {
		t.Fatalf("create hub: %v", err)
	}
	token, err := hub.SignSession("ramadan-board", "member-42", time.Now().Add(time.Minute))
	if err != nil {
		t.Fatalf("sign session: %v", err)
	}
	if _, err := hub.VerifySession(token, "ramadan-board", "member-42"); err != nil {
		t.Fatalf("verify valid session: %v", err)
	}
	if _, err := hub.VerifySession(token, "other-artifact", "member-42"); err == nil {
		t.Fatal("expected an artifact mismatch to be rejected")
	}
	if _, err := hub.VerifySession(token+"tampered", "ramadan-board", "member-42"); err == nil {
		t.Fatal("expected a tampered session to be rejected")
	}
	if _, err := hub.SignSession("ramadan-board", "member-42", time.Now().Add(25*time.Hour)); err == nil {
		t.Fatal("expected an overlong session to be rejected")
	}
}

func TestRelayRejectsUnsignedConnection(t *testing.T) {
	hub, err := NewP2PRelayHubWithConfig(RelayConfig{SharedSecret: []byte("0123456789abcdef0123456789abcdef")})
	if err != nil {
		t.Fatalf("create hub: %v", err)
	}
	req := httptest.NewRequest("GET", "/api/p2p/stream?artifactId=ramadan-board&peerId=member-42", nil)
	response := httptest.NewRecorder()
	hub.HandleP2PStream(response, req)
	if response.Code != 401 {
		t.Fatalf("expected unsigned connection to return 401, got %d", response.Code)
	}
}

func TestRelayHonorsOriginAllowList(t *testing.T) {
	hub, err := NewP2PRelayHubWithConfig(RelayConfig{
		SharedSecret:   []byte("0123456789abcdef0123456789abcdef"),
		AllowedOrigins: []string{"https://app.taawun.community"},
	})
	if err != nil {
		t.Fatalf("create hub: %v", err)
	}
	allowed := httptest.NewRequest("GET", "/api/p2p/stream", nil)
	allowed.Header.Set("Origin", "https://app.taawun.community")
	if !hub.isOriginAllowed(allowed) {
		t.Fatal("expected configured origin to be allowed")
	}
	blocked := httptest.NewRequest("GET", "/api/p2p/stream", nil)
	blocked.Header.Set("Origin", "https://attacker.example")
	if hub.isOriginAllowed(blocked) {
		t.Fatal("expected unconfigured origin to be rejected")
	}
}
