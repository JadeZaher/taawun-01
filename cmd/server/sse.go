package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type SSESignal struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}
type SSEUpdate struct {
	Signals []SSESignal `json:"signals"`
}

var (
	sseClients   = make(map[chan SSEUpdate]bool)
	sseClientsMu sync.RWMutex
)

func sseHandler(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	clientChan := make(chan SSEUpdate, 10)
	sseClientsMu.Lock()
	sseClients[clientChan] = true
	sseClientsMu.Unlock()

	sendSSEUpdate(w, flusher, SSEUpdate{Signals: []SSESignal{
		{Name: "connected", Value: true},
		{Name: "timestamp", Value: time.Now().Format(time.RFC3339)},
	}})

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	defer func() {
		sseClientsMu.Lock()
		delete(sseClients, clientChan)
		sseClientsMu.Unlock()
		close(clientChan)
	}()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			sendSSEUpdate(w, flusher, SSEUpdate{Signals: []SSESignal{{Name: "heartbeat", Value: time.Now().Format(time.RFC3339)}}})
		case update := <-clientChan:
			sendSSEUpdate(w, flusher, update)
		}
	}
}

func sendSSEUpdate(w http.ResponseWriter, flusher http.Flusher, update SSEUpdate) {
	data, err := json.Marshal(update)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

// broadcastSignal pushes real-time collaboration events to all connected clients.
func broadcastSignal(name string, value interface{}) {
	update := SSEUpdate{Signals: []SSESignal{{Name: name, Value: value}}}
	sseClientsMu.RLock()
	defer sseClientsMu.RUnlock()
	for client := range sseClients {
		select {
		case client <- update:
		default:
		}
	}
}
