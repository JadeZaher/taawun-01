package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"taawun/pkg/ethics"
)

const (
	maximumEthicsAuditBodyBytes = 8 << 10
	maximumEthicsPromptBytes    = 4 << 10
	ethicsAuditAttemptLimit     = 30
	ethicsAuditWindow           = time.Minute
)

type ethicsAuditor interface {
	AuditPrompt(string) (*ethics.ComplianceResult, error)
}

// EthicsHTTPHandler exposes the bounded private-beta compliance audit.
type EthicsHTTPHandler struct {
	auditor ethicsAuditor
	limiter *authRateLimiter
}

func NewEthicsHTTPHandler(auditor ethicsAuditor) *EthicsHTTPHandler {
	return &EthicsHTTPHandler{auditor: auditor, limiter: newAuthRateLimiter(ethicsAuditWindow, maximumPublicAuthClients)}
}

func (h *EthicsHTTPHandler) Audit(w http.ResponseWriter, r *http.Request) {
	actor, ok := CurrentUser(r.Context())
	if !ok || actor.ID <= 0 {
		writeEthicsError(w, http.StatusUnauthorized, "authentication_required", "Authentication is required.")
		return
	}
	allowed, retryAfter := h.limiter.Allow("ethics_audit", strconv.Itoa(actor.ID), ethicsAuditAttemptLimit)
	if !allowed {
		seconds := int(retryAfter.Round(time.Second) / time.Second)
		if seconds < 1 {
			seconds = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(seconds))
		writeEthicsError(w, http.StatusTooManyRequests, "audit_rate_limited", "Too many audit requests. Please try again later.")
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeEthicsError(w, http.StatusUnsupportedMediaType, "content_type_required", "Content-Type must be application/json.")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maximumEthicsAuditBodyBytes))
	if err != nil {
		writeEthicsError(w, http.StatusRequestEntityTooLarge, "request_too_large", "Request body is too large.")
		return
	}
	var input struct {
		Prompt string `json:"prompt"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeEthicsError(w, http.StatusBadRequest, "invalid_request", "Request body must be one valid JSON object.")
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeEthicsError(w, http.StatusBadRequest, "invalid_request", "Request body must contain one JSON object.")
		return
	}
	input.Prompt = strings.TrimSpace(input.Prompt)
	if input.Prompt == "" || len(input.Prompt) > maximumEthicsPromptBytes {
		writeEthicsError(w, http.StatusBadRequest, "invalid_prompt", "Prompt must contain between 1 and 4096 bytes of text.")
		return
	}
	result, err := h.auditor.AuditPrompt(input.Prompt)
	if err != nil {
		writeEthicsError(w, http.StatusUnprocessableEntity, "audit_rejected", "The prompt could not be audited.")
		return
	}
	writeEthicsJSON(w, http.StatusOK, result)
}

func writeEthicsError(w http.ResponseWriter, status int, code, message string) {
	writeEthicsJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func writeEthicsJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
