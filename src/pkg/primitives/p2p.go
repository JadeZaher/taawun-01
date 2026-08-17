package primitives

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	defaultRelayMaxMessageBytes      = 64 * 1024
	defaultRelayConnectionsPerMinute = 10
	maxRelaySessionLifetime          = 5 * time.Minute
	RelayWebSocketSubprotocol        = "taawun-relay-v1"
)

var validP2PMessageTypes = map[string]struct{}{
	"join": {}, "offer": {}, "answer": {}, "ice-candidate": {}, "crdt-sync": {}, "leave": {},
}

// RelayConfig controls the trust boundary for a self-hosted relay instance.
type RelayConfig struct {
	SharedSecret            []byte
	AllowedOrigins          []string
	MaxMessageBytes         int64
	MaxConnectionsPerMinute int
}

// RelaySessionGrant is trusted control-plane authorization to open one relay connection.
type RelaySessionGrant struct {
	ArtifactID  string
	WorkspaceID int
	PrincipalID int
	DeviceID    string
	PeerID      string
	Origin      string
	ExpiresAt   time.Time
}

// RelaySession is the signed, short-lived relay authorization payload.
type RelaySession struct {
	SessionID   string `json:"session_id"`
	ArtifactID  string `json:"artifact_id"`
	WorkspaceID int    `json:"workspace_id"`
	PrincipalID int    `json:"principal_id"`
	DeviceID    string `json:"device_id"`
	PeerID      string `json:"peer_id"`
	Origin      string `json:"origin"`
	ExpiresAt   int64  `json:"expires_at"`
	IssuedAt    int64  `json:"issued_at"`
}

type connectionWindow struct {
	startedAt time.Time
	count     int
}

// P2PMessage defines signaling and encrypted CRDT payloads exchanged between peers.
type P2PMessage struct {
	Type       string          `json:"type"`
	ArtifactID string          `json:"artifactId"`
	PeerID     string          `json:"peerId"`
	TargetID   string          `json:"targetId,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
}

type Client struct {
	ID         string
	ArtifactID string
	Conn       *websocket.Conn
	Send       chan []byte
	expiresAt  int64
}

// P2PRelayHub manages authenticated WebRTC signaling and encrypted fallback relays.
type P2PRelayHub struct {
	mu                sync.RWMutex
	artifacts         map[string]map[string]*Client
	sharedSecret      []byte
	allowedOrigins    map[string]struct{}
	maxMessageBytes   int64
	connectionsPerMin int
	connections       map[string]connectionWindow
	usedSessions      map[string]int64
	upgrader          websocket.Upgrader
}

// NewP2PRelayHub creates a safe-by-default in-process relay with an ephemeral secret.
// Production callers should use NewP2PRelayHubWithConfig so issued sessions survive restarts.
func NewP2PRelayHub() *P2PRelayHub {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		panic(fmt.Sprintf("generate relay secret: %v", err))
	}
	hub, err := NewP2PRelayHubWithConfig(RelayConfig{SharedSecret: secret})
	if err != nil {
		panic(fmt.Sprintf("create relay hub: %v", err))
	}
	return hub
}

// NewP2PRelayHubWithConfig creates a relay that requires HMAC-signed sessions.
func NewP2PRelayHubWithConfig(config RelayConfig) (*P2PRelayHub, error) {
	if len(config.SharedSecret) < 32 {
		return nil, errors.New("relay shared secret must be at least 32 bytes")
	}
	if config.MaxMessageBytes <= 0 {
		config.MaxMessageBytes = defaultRelayMaxMessageBytes
	}
	if config.MaxConnectionsPerMinute <= 0 {
		config.MaxConnectionsPerMinute = defaultRelayConnectionsPerMinute
	}

	origins := make(map[string]struct{}, len(config.AllowedOrigins))
	for _, origin := range config.AllowedOrigins {
		normalized, err := normalizeOrigin(origin)
		if err != nil {
			return nil, fmt.Errorf("invalid allowed relay origin %q: %w", origin, err)
		}
		origins[normalized] = struct{}{}
	}

	hub := &P2PRelayHub{
		artifacts:         make(map[string]map[string]*Client),
		sharedSecret:      append([]byte(nil), config.SharedSecret...),
		allowedOrigins:    origins,
		maxMessageBytes:   config.MaxMessageBytes,
		connectionsPerMin: config.MaxConnectionsPerMinute,
		connections:       make(map[string]connectionWindow),
		usedSessions:      make(map[string]int64),
	}
	hub.upgrader = websocket.Upgrader{CheckOrigin: hub.isOriginAllowed, Subprotocols: []string{RelayWebSocketSubprotocol}}
	return hub, nil
}

// IssueSession signs a control-plane-authorized, principal-bound one-time relay session.
func (h *P2PRelayHub) IssueSession(grant RelaySessionGrant) (string, *RelaySession, error) {
	if h == nil {
		return "", nil, errors.New("relay hub is required")
	}
	now := time.Now()
	origin, err := normalizeOrigin(grant.Origin)
	if err != nil {
		return "", nil, errors.New("relay session origin must be an exact HTTP or HTTPS origin")
	}
	if !validRelayID(grant.ArtifactID) || !validRelayID(grant.DeviceID) || !validRelayID(grant.PeerID) || grant.WorkspaceID <= 0 || grant.PrincipalID <= 0 {
		return "", nil, errors.New("relay session requires valid artifact, workspace, principal, device, and peer bindings")
	}
	if grant.ExpiresAt.UnixMilli() <= now.UnixMilli() {
		return "", nil, errors.New("relay session expiry must be in the future")
	}
	if grant.ExpiresAt.After(now.Add(maxRelaySessionLifetime)) {
		return "", nil, fmt.Errorf("relay session expiry cannot exceed %s", maxRelaySessionLifetime)
	}
	if _, allowed := h.allowedOrigins[origin]; !allowed {
		return "", nil, errors.New("relay session origin is not allowed by this relay")
	}
	sessionID, err := relaySessionID()
	if err != nil {
		return "", nil, err
	}
	session := &RelaySession{
		SessionID: sessionID, ArtifactID: grant.ArtifactID, WorkspaceID: grant.WorkspaceID,
		PrincipalID: grant.PrincipalID, DeviceID: grant.DeviceID, PeerID: grant.PeerID,
		Origin: origin, ExpiresAt: grant.ExpiresAt.UnixMilli(), IssuedAt: now.UnixMilli(),
	}
	payload, err := json.Marshal(session)
	if err != nil {
		return "", nil, fmt.Errorf("marshal relay session: %w", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, h.sharedSecret)
	_, _ = mac.Write([]byte(encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), session, nil
}

// VerifySession validates signed claims before a trusted caller issues an upgrade.
func (h *P2PRelayHub) VerifySession(token string) (*RelaySession, error) {
	if h == nil {
		return nil, errors.New("relay hub is required")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, errors.New("malformed relay session token")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("malformed relay session signature")
	}
	mac := hmac.New(sha256.New, h.sharedSecret)
	_, _ = mac.Write([]byte(parts[0]))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return nil, errors.New("invalid relay session signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("malformed relay session payload")
	}
	var session RelaySession
	if err := json.Unmarshal(payload, &session); err != nil {
		return nil, errors.New("malformed relay session")
	}
	if !validRelayID(session.SessionID) || !validRelayID(session.ArtifactID) || !validRelayID(session.DeviceID) || !validRelayID(session.PeerID) || session.WorkspaceID <= 0 || session.PrincipalID <= 0 {
		return nil, errors.New("invalid relay session identity")
	}
	origin, err := normalizeOrigin(session.Origin)
	if err != nil || origin != session.Origin {
		return nil, errors.New("invalid relay session origin")
	}
	if session.ExpiresAt <= time.Now().UnixMilli() {
		return nil, errors.New("relay session expired")
	}
	return &session, nil
}

// HandleP2PStream handles authenticated WebSocket connections for signaling and fallback relay.
func (h *P2PRelayHub) HandleP2PStream(w http.ResponseWriter, r *http.Request) {
	artifactID := r.URL.Query().Get("artifactId")
	peerID := r.URL.Query().Get("peerId")
	if !validRelayID(artifactID) || !validRelayID(peerID) {
		http.Error(w, "artifactId and peerId must be valid identifiers", http.StatusBadRequest)
		return
	}
	if r.URL.Query().Has("token") || r.Header.Get("Authorization") != "" {
		http.Error(w, "relay credentials must use the WebSocket subprotocol", http.StatusBadRequest)
		return
	}
	token, ok := relayTokenFromSubprotocol(r)
	if !ok {
		http.Error(w, "valid relay session token required", http.StatusUnauthorized)
		return
	}
	session, err := h.VerifySession(token)
	if err != nil {
		http.Error(w, "valid relay session token required", http.StatusUnauthorized)
		return
	}
	origin, err := normalizeOrigin(r.Header.Get("Origin"))
	if err != nil || session.ArtifactID != artifactID || session.PeerID != peerID || session.Origin != origin {
		http.Error(w, "relay session does not match this connection", http.StatusUnauthorized)
		return
	}
	if !h.allowConnection(r.RemoteAddr) {
		http.Error(w, "relay connection rate limit exceeded", http.StatusTooManyRequests)
		return
	}
	if !h.reserveSession(session) {
		http.Error(w, "relay session has already been used", http.StatusUnauthorized)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.releaseSession(session.SessionID)
		return
	}
	client := &Client{ID: peerID, ArtifactID: artifactID, Conn: conn, Send: make(chan []byte, 64), expiresAt: session.ExpiresAt}
	if err := h.registerClient(client); err != nil {
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, err.Error()), time.Now().Add(time.Second))
		_ = conn.Close()
		return
	}

	go client.writePump()
	client.readPump(h)
}

func relaySessionID() (string, error) {
	value := make([]byte, 18)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate relay session ID: %w", err)
	}
	return "relay_" + base64.RawURLEncoding.EncodeToString(value), nil
}

func relayTokenFromSubprotocol(r *http.Request) (string, bool) {
	protocols := websocket.Subprotocols(r)
	if len(protocols) != 2 {
		return "", false
	}
	var token string
	for _, protocol := range protocols {
		switch {
		case protocol == RelayWebSocketSubprotocol:
			continue
		case token == "" && len(protocol) <= 8192:
			token = protocol
		default:
			return "", false
		}
	}
	return token, token != ""
}

func (h *P2PRelayHub) reserveSession(session *RelaySession) bool {
	if h == nil || session == nil {
		return false
	}
	now := time.Now().UnixMilli()
	h.mu.Lock()
	defer h.mu.Unlock()
	for sessionID, expiresAt := range h.usedSessions {
		if expiresAt <= now {
			delete(h.usedSessions, sessionID)
		}
	}
	if _, used := h.usedSessions[session.SessionID]; used {
		return false
	}
	h.usedSessions[session.SessionID] = session.ExpiresAt
	return true
}

func (h *P2PRelayHub) releaseSession(sessionID string) {
	h.mu.Lock()
	delete(h.usedSessions, sessionID)
	h.mu.Unlock()
}

func (h *P2PRelayHub) registerClient(client *Client) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	peers := h.artifacts[client.ArtifactID]
	if peers == nil {
		peers = make(map[string]*Client)
		h.artifacts[client.ArtifactID] = peers
	}
	if _, exists := peers[client.ID]; exists {
		return errors.New("peer is already connected to this artifact")
	}
	peers[client.ID] = client
	return nil
}

func (h *P2PRelayHub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	peers, exists := h.artifacts[client.ArtifactID]
	if !exists || peers[client.ID] != client {
		return
	}
	delete(peers, client.ID)
	close(client.Send)
	if len(peers) == 0 {
		delete(h.artifacts, client.ArtifactID)
	}
}

func (c *Client) readPump(h *P2PRelayHub) {
	defer func() {
		h.unregisterClient(c)
		_ = c.Conn.Close()
	}()
	c.Conn.SetReadLimit(h.maxMessageBytes)
	_ = c.Conn.SetReadDeadline(time.UnixMilli(c.expiresAt))
	for {
		_, rawMsg, err := c.Conn.ReadMessage()
		if err != nil {
			return
		}
		var message P2PMessage
		if err := json.Unmarshal(rawMsg, &message); err != nil || !validP2PMessage(message) {
			continue
		}
		message.PeerID = c.ID
		message.ArtifactID = c.ArtifactID
		encoded, err := json.Marshal(message)
		if err != nil {
			continue
		}
		h.forward(c, encoded, message.TargetID)
	}
}

func (h *P2PRelayHub) forward(sender *Client, message []byte, targetID string) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	peers := h.artifacts[sender.ArtifactID]
	if targetID != "" {
		if peer := peers[targetID]; peer != nil {
			select {
			case peer.Send <- message:
			default:
			}
		}
		return
	}
	for peerID, peer := range peers {
		if peerID == sender.ID {
			continue
		}
		select {
		case peer.Send <- message:
		default:
		}
	}
}

func (c *Client) writePump() {
	defer c.Conn.Close()
	for message := range c.Send {
		_ = c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
			return
		}
	}
}

func (h *P2PRelayHub) isOriginAllowed(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	normalized, err := normalizeOrigin(origin)
	if err != nil {
		return false
	}
	_, allowed := h.allowedOrigins[normalized]
	return allowed
}

func (h *P2PRelayHub) allowConnection(remoteAddress string) bool {
	host, _, err := net.SplitHostPort(remoteAddress)
	if err != nil {
		host = remoteAddress
	}
	now := time.Now()
	h.mu.Lock()
	defer h.mu.Unlock()
	for ip, window := range h.connections {
		if now.Sub(window.startedAt) >= time.Minute {
			delete(h.connections, ip)
		}
	}
	window := h.connections[host]
	if window.startedAt.IsZero() || now.Sub(window.startedAt) >= time.Minute {
		window = connectionWindow{startedAt: now}
	}
	if window.count >= h.connectionsPerMin {
		return false
	}
	window.count++
	h.connections[host] = window
	return true
}

func validP2PMessage(message P2PMessage) bool {
	if _, ok := validP2PMessageTypes[message.Type]; !ok {
		return false
	}
	return message.TargetID == "" || validRelayID(message.TargetID)
}

func validRelayID(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || character == '.' || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func normalizeOrigin(origin string) (string, error) {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Path != "" && parsed.Path != "/" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("origin must be a scheme and host only")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("origin must use http or https")
	}
	return strings.ToLower(parsed.Scheme + "://" + parsed.Host), nil
}
