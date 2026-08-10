package mcp

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"taawun/pkg/primitives"
)

func TestMCPServer_InitializeAndListTools(t *testing.T) {
	hub := primitives.NewP2PRelayHub()
	server := NewMCPServer(hub)

	reqBody, _ := json.Marshal(JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "tools/list",
		ID:      1,
	})

	req := httptest.NewRequest("POST", "/api/mcp", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()

	server.HandleRPC(w, req)

	if w.Code != 200 {
		t.Fatalf("Expected HTTP 200, got %d", w.Code)
	}

	var res JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&res); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if res.Error != nil {
		t.Fatalf("Unexpected RPC error: %v", res.Error)
	}
}
