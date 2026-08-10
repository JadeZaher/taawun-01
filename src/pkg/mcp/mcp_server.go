package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"taawun/pkg/ethics"
	"taawun/pkg/iac"
	"taawun/pkg/primitives"
)

// JSONRPCRequest represents an MCP JSON-RPC 2.0 payload.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      interface{}     `json:"id"`
}

// JSONRPCResponse represents an MCP response payload.
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

// MCPServer embeds MCP capabilities directly inside the Taawun Go control plane binary.
type MCPServer struct {
	ethicsEngine *ethics.HaramCheckEngine
	dockerEngine *iac.DockerProvider
	p2pHub       *primitives.P2PRelayHub
}

func NewMCPServer(p2pHub *primitives.P2PRelayHub) *MCPServer {
	return &MCPServer{
		ethicsEngine: ethics.NewHaramCheckEngine(),
		dockerEngine: iac.NewDockerProvider(),
		p2pHub:       p2pHub,
	}
}

// HandleRPC processes inbound MCP JSON-RPC requests.
func (s *MCPServer) HandleRPC(w http.ResponseWriter, r *http.Request) {
	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	res := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID}

	switch req.Method {
	case "initialize":
		res.Result = map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]string{
				"name":    "Taawun-Go-MCP-Server",
				"version": "1.0.0",
			},
		}

	case "tools/list":
		res.Result = map[string]interface{}{
			"tools": []Tool{
				{
					Name:        "run_ethics_check",
					Description: "Audits a prompt or code artifact for Taqwa compliance (anti-Riba, anti-Gharar, non-prohibited domain model).",
				},
				{
					Name:        "deploy_container",
					Description: "Provisions a container sandbox or hosted app using the Go IaC Engine.",
				},
				{
					Name:        "init_p2p_relay",
					Description: "Initializes a Web P2P signaling channel & WebSocket relay for local-first browser artifacts.",
				},
			},
		}

	case "tools/call":
		s.handleToolCall(&req, &res)

	default:
		res.Error = &RPCError{Code: -32601, Message: fmt.Sprintf("Method '%s' not found", req.Method)}
	}

	json.NewEncoder(w).Encode(res)
}

func (s *MCPServer) handleToolCall(req *JSONRPCRequest, res *JSONRPCResponse) {
	var callParams struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &callParams); err != nil {
		res.Error = &RPCError{Code: -32602, Message: "Invalid tool call parameters"}
		return
	}

	switch callParams.Name {
	case "run_ethics_check":
		prompt, _ := callParams.Arguments["prompt"].(string)
		auditRes, err := s.ethicsEngine.AuditPrompt(prompt)
		if err != nil {
			res.Error = &RPCError{Code: -32000, Message: err.Error()}
			return
		}
		res.Result = auditRes

	case "deploy_container":
		artifactID, _ := callParams.Arguments["artifactId"].(string)
		name, _ := callParams.Arguments["name"].(string)
		image, _ := callParams.Arguments["image"].(string)
		if image == "" {
			image = "nginx:alpine"
		}

		manifest := &iac.AppManifest{
			ArtifactID: artifactID,
			Name:       name,
			Image:      image,
			Port:       80,
			HostPort:   8081,
			MemoryMB:   256,
		}
		status, err := s.dockerEngine.Deploy(context.Background(), manifest)
		if err != nil {
			res.Result = map[string]interface{}{
				"status": "dry-run",
				"note":   fmt.Sprintf("Docker execution attempted (%v). Manifest prepared.", err),
				"manifest": manifest,
			}
			return
		}
		res.Result = status

	case "init_p2p_relay":
		artifactID, _ := callParams.Arguments["artifactId"].(string)
		res.Result = map[string]interface{}{
			"artifactId":   artifactID,
			"signalingUrl": fmt.Sprintf("ws://localhost:8080/api/p2p/stream?artifactId=%s", artifactID),
			"mode":         "local-first P2P with Taawun relay fallback",
		}

	default:
		res.Error = &RPCError{Code: -32601, Message: fmt.Sprintf("Tool '%s' not found", callParams.Name)}
	}
}
