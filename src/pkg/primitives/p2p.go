package primitives

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// P2PMessage defines signaling and CRDT sync payloads exchanged between peers or relays.
type P2PMessage struct {
	Type      string          `json:"type"`       // "join", "offer", "answer", "ice-candidate", "crdt-sync", "leave"
	ArtifactID string         `json:"artifactId"` // ID of the generated artifact network
	PeerID    string          `json:"peerId"`
	TargetID  string          `json:"targetId,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type Client struct {
	ID         string
	ArtifactID string
	Conn       *websocket.Conn
	Send       chan []byte
}

// P2PRelayHub manages WebRTC signaling channels and WebSocket fallback relays for sandboxed browser artifacts.
type P2PRelayHub struct {
	mu         sync.RWMutex
	artifacts  map[string]map[string]*Client // artifactID -> peerID -> Client
	register   chan *Client
	unregister chan *Client
}

func NewP2PRelayHub() *P2PRelayHub {
	hub := &P2PRelayHub{
		artifacts:  make(map[string]map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
	go hub.run()
	return hub
}

func (h *P2PRelayHub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if _, exists := h.artifacts[client.ArtifactID]; !exists {
				h.artifacts[client.ArtifactID] = make(map[string]*Client)
			}
			h.artifacts[client.ArtifactID][client.ID] = client
			h.mu.Unlock()
			log.Printf("[P2P Relay] Peer %s joined artifact network %s", client.ID, client.ArtifactID)

		case client := <-h.unregister:
			h.mu.Lock()
			if peers, exists := h.artifacts[client.ArtifactID]; exists {
				if _, ok := peers[client.ID]; ok {
					delete(peers, client.ID)
					close(client.Send)
					if len(peers) == 0 {
						delete(h.artifacts, client.ArtifactID)
					}
					log.Printf("[P2P Relay] Peer %s left artifact network %s", client.ID, client.ArtifactID)
				}
			}
			h.mu.Unlock()
		}
	}
}

// HandleP2PStream handles WebSocket connections for peer signaling and CRDT fallback stream relay.
func (h *P2PRelayHub) HandleP2PStream(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("P2P WebSocket upgrade error: %v", err)
		return
	}

	artifactID := r.URL.Query().Get("artifactId")
	peerID := r.URL.Query().Get("peerId")
	if artifactID == "" || peerID == "" {
		http.Error(w, "artifactId and peerId are required parameters", http.StatusBadRequest)
		conn.Close()
		return
	}

	client := &Client{
		ID:         peerID,
		ArtifactID: artifactID,
		Conn:       conn,
		Send:       make(chan []byte, 256),
	}

	h.register <- client

	go client.writePump()
	client.readPump(h)
}

func (c *Client) readPump(h *P2PRelayHub) {
	defer func() {
		h.unregister <- c
		c.Conn.Close()
	}()

	for {
		_, rawMsg, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}

		var msg P2PMessage
		if err := json.Unmarshal(rawMsg, &msg); err != nil {
			continue
		}

		msg.PeerID = c.ID
		msg.ArtifactID = c.ArtifactID

		h.mu.RLock()
		peers, exists := h.artifacts[c.ArtifactID]
		if !exists {
			h.mu.RUnlock()
			continue
		}

		// Relay to targeted peer or broadcast to all peers in the artifact network
		if msg.TargetID != "" {
			if targetPeer, ok := peers[msg.TargetID]; ok {
				select {
				case targetPeer.Send <- rawMsg:
				default:
				}
			}
		} else {
			for pid, peer := range peers {
				if pid != c.ID {
					select {
					case peer.Send <- rawMsg:
					default:
					}
				}
			}
		}
		h.mu.RUnlock()
	}
}

func (c *Client) writePump() {
	defer c.Conn.Close()
	for message := range c.Send {
		if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
			return
		}
	}
}
