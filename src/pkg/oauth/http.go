package oauth

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const oauthSessionCookie = "__Host-taawun-oauth-session"

type HTTPHandler struct {
	service    *Service
	cookieName string
	secure     bool
}

func NewHTTPHandler(service *Service) *HTTPHandler {
	secure := strings.HasPrefix(service.config.Issuer, "https://")
	name := oauthSessionCookie
	if !secure {
		name = "taawun-oauth-session"
	}
	return &HTTPHandler{service: service, cookieName: name, secure: secure}
}

func (h *HTTPHandler) ProtectedResourceMetadata(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"resource":                 h.service.config.Resource,
		"resource_name":            "Taawun MCP",
		"authorization_servers":    []string{h.service.config.Issuer},
		"scopes_supported":         supportedScopes,
		"bearer_methods_supported": []string{"header"},
	})
}

func (h *HTTPHandler) AuthorizationServerMetadata(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                                     h.service.config.Issuer,
		"authorization_endpoint":                     h.service.endpoint("/oauth/authorize"),
		"token_endpoint":                             h.service.endpoint("/oauth/token"),
		"registration_endpoint":                      h.service.endpoint("/oauth/register"),
		"revocation_endpoint":                        h.service.endpoint("/oauth/revoke"),
		"scopes_supported":                           supportedScopes,
		"response_types_supported":                   []string{"code"},
		"response_modes_supported":                   []string{"query"},
		"grant_types_supported":                      []string{"authorization_code", "refresh_token"},
		"token_endpoint_auth_methods_supported":      []string{"none"},
		"revocation_endpoint_auth_methods_supported": []string{"none"},
		"code_challenge_methods_supported":           []string{"S256"},
		"client_id_metadata_document_supported":      true,
	})
}

type registrationRequest struct {
	ClientName              string   `json:"client_name"`
	ClientURI               string   `json:"client_uri"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
}

func (h *HTTPHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeOAuthError(w, http.StatusUnsupportedMediaType, "invalid_client_metadata", "Content-Type must be application/json")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	var request registrationRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_client_metadata", "registration metadata must be valid JSON")
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeOAuthError(w, http.StatusBadRequest, "invalid_client_metadata", "registration body must contain one JSON object")
		return
	}
	client, err := h.service.RegisterClient(Client{Name: request.ClientName, ClientURI: request.ClientURI, RedirectURIs: request.RedirectURIs, GrantTypes: request.GrantTypes, ResponseTypes: request.ResponseTypes, TokenEndpointAuthMethod: request.TokenEndpointAuthMethod})
	if err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_client_metadata", err.Error())
		return
	}
	writeJSONStatus(w, http.StatusCreated, map[string]any{
		"client_id": client.ID, "client_id_issued_at": h.service.config.Now().UTC().Unix(),
		"client_name": client.Name, "client_uri": client.ClientURI, "redirect_uris": client.RedirectURIs,
		"grant_types": client.GrantTypes, "response_types": client.ResponseTypes,
		"token_endpoint_auth_method": client.TokenEndpointAuthMethod,
	})
}

func (h *HTTPHandler) Authorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	requestID, err := h.service.BeginAuthorization(AuthorizationInput{
		ClientID: r.URL.Query().Get("client_id"), RedirectURI: r.URL.Query().Get("redirect_uri"),
		ResponseType: r.URL.Query().Get("response_type"), Scope: r.URL.Query().Get("scope"),
		State: r.URL.Query().Get("state"), Resource: r.URL.Query().Get("resource"),
		CodeChallenge: r.URL.Query().Get("code_challenge"), CodeChallengeMethod: r.URL.Query().Get("code_challenge_method"),
	})
	if err != nil {
		renderError(w, http.StatusBadRequest, "Authorization request rejected", err.Error())
		return
	}
	if cookie, err := r.Cookie(h.cookieName); err == nil {
		view, err := h.service.ConsentView(requestID, cookie.Value, "")
		if err == nil {
			renderConsent(w, view)
			return
		}
	}
	renderLogin(w, requestID, "")
}

func (h *HTTPHandler) Login(w http.ResponseWriter, r *http.Request) {
	if !parseForm(w, r) {
		return
	}
	requestID := r.PostForm.Get("request_id")
	session, csrf, err := h.service.LoginAuthorization(requestID, r.PostForm.Get("email"), r.PostForm.Get("password"))
	if err != nil {
		renderLogin(w, requestID, "Email or password was not accepted.")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: h.cookieName, Value: session, Path: "/", MaxAge: int(h.service.config.SessionTTL.Seconds()), HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteLaxMode})
	view, err := h.service.ConsentView(requestID, session, csrf)
	if err != nil {
		renderError(w, http.StatusBadRequest, "Authorization expired", "Start the connection again from your MCP client.")
		return
	}
	renderConsent(w, view)
}

func (h *HTTPHandler) Consent(w http.ResponseWriter, r *http.Request) {
	if !parseForm(w, r) {
		return
	}
	cookie, err := r.Cookie(h.cookieName)
	if err != nil {
		renderError(w, http.StatusUnauthorized, "Sign-in required", "Start the connection again from your MCP client.")
		return
	}
	requestID, csrf := r.PostForm.Get("request_id"), r.PostForm.Get("csrf")
	if _, err := h.service.ConsentView(requestID, cookie.Value, csrf); err != nil {
		renderError(w, http.StatusBadRequest, "Consent request rejected", "The session or consent form is no longer valid.")
		return
	}
	if r.PostForm.Get("decision") != "approve" {
		redirect, err := h.service.Deny(requestID)
		if err != nil {
			renderError(w, http.StatusBadRequest, "Authorization expired", "Start the connection again from your MCP client.")
			return
		}
		http.Redirect(w, r, redirect, http.StatusFound)
		return
	}
	workspaceIDs := make([]int, 0, len(r.PostForm["workspace_id"]))
	for _, raw := range r.PostForm["workspace_id"] {
		id, err := strconv.Atoi(raw)
		if err != nil {
			renderError(w, http.StatusBadRequest, "Consent request rejected", "A selected workspace was invalid.")
			return
		}
		workspaceIDs = append(workspaceIDs, id)
	}
	redirect, err := h.service.Approve(requestID, cookie.Value, csrf, workspaceIDs)
	if err != nil {
		renderError(w, http.StatusForbidden, "Consent request rejected", err.Error())
		return
	}
	http.Redirect(w, r, redirect, http.StatusFound)
}

func (h *HTTPHandler) Token(w http.ResponseWriter, r *http.Request) {
	if !parseForm(w, r) {
		return
	}
	if r.Header.Get("Authorization") != "" {
		writeOAuthError(w, http.StatusUnauthorized, "invalid_client", "this authorization server accepts public clients only")
		return
	}
	var response *TokenResponse
	var err error
	switch r.PostForm.Get("grant_type") {
	case "authorization_code":
		response, err = h.service.ExchangeCode(r.PostForm.Get("code"), r.PostForm.Get("client_id"), r.PostForm.Get("redirect_uri"), r.PostForm.Get("resource"), r.PostForm.Get("code_verifier"))
	case "refresh_token":
		response, err = h.service.Refresh(r.PostForm.Get("refresh_token"), r.PostForm.Get("client_id"), r.PostForm.Get("resource"))
	default:
		writeOAuthError(w, http.StatusBadRequest, "unsupported_grant_type", "only authorization_code and refresh_token are supported")
		return
	}
	if err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "the grant is invalid, expired, revoked, or already used")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *HTTPHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	if !parseForm(w, r) {
		return
	}
	if r.Header.Get("Authorization") != "" {
		writeOAuthError(w, http.StatusUnauthorized, "invalid_client", "this authorization server accepts public clients only")
		return
	}
	_ = h.service.Revoke(r.PostForm.Get("token"), r.PostForm.Get("client_id"))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
}

func parseForm(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return false
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/x-www-form-urlencoded" {
		writeOAuthError(w, http.StatusUnsupportedMediaType, "invalid_request", "Content-Type must be application/x-www-form-urlencoded")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "form body could not be parsed")
		return false
	}
	return true
}

func (s *Service) resourceMetadataURL() string {
	resource, _ := url.Parse(s.config.Resource)
	return resource.Scheme + "://" + resource.Host + "/.well-known/oauth-protected-resource" + resource.EscapedPath()
}

func (s *Service) challenge(w http.ResponseWriter, status int, errorCode, scope string) {
	parameters := []string{fmt.Sprintf(`resource_metadata=%q`, s.resourceMetadataURL()), fmt.Sprintf(`scope=%q`, scope)}
	if errorCode != "" {
		parameters = append(parameters, fmt.Sprintf(`error=%q`, errorCode))
	}
	w.Header().Set("WWW-Authenticate", "Bearer "+strings.Join(parameters, ", "))
	http.Error(w, http.StatusText(status), status)
}

func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("access_token") {
			writeOAuthError(w, http.StatusBadRequest, "invalid_request", "access tokens are accepted only in the Authorization header")
			return
		}
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			s.challenge(w, http.StatusUnauthorized, "", ScopeRead)
			return
		}
		principal, err := s.ValidateAccess(parts[1])
		if err != nil {
			s.challenge(w, http.StatusUnauthorized, "invalid_token", ScopeRead)
			return
		}
		ctx := s.bindPrincipal(r.Context(), principal)
		if !HasScope(ctx, ScopeRead) {
			s.challenge(w, http.StatusForbidden, "insufficient_scope", ScopeRead)
			return
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Service) RequireScopeHTTP(scope string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !HasScope(r.Context(), scope) {
			s.challenge(w, http.StatusForbidden, "insufficient_scope", scope)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type rpcEnvelope struct {
	Method string `json:"method"`
	Params struct {
		Name string `json:"name"`
	} `json:"params"`
}

// MCPToolScopeMiddleware turns a JSON-RPC tool scope miss into the MCP-prescribed HTTP challenge.
func (s *Service) MCPToolScopeMiddleware(requirements map[string]string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Body == nil {
			next.ServeHTTP(w, r)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, (2<<20)+1))
		if err != nil || len(body) > 2<<20 {
			writeOAuthError(w, http.StatusRequestEntityTooLarge, "invalid_request", "MCP request body is too large")
			return
		}
		_ = r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(body))
		envelopes := make([]rpcEnvelope, 0, 1)
		trimmed := bytes.TrimSpace(body)
		if len(trimmed) > 0 && trimmed[0] == '[' {
			_ = json.Unmarshal(trimmed, &envelopes)
		} else {
			var envelope rpcEnvelope
			if json.Unmarshal(trimmed, &envelope) == nil {
				envelopes = append(envelopes, envelope)
			}
		}
		for _, envelope := range envelopes {
			required, known := requirements[envelope.Params.Name]
			if envelope.Method == "tools/call" && known && !HasScope(r.Context(), required) {
				s.challenge(w, http.StatusForbidden, "insufficient_scope", required)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	writeJSONStatus(w, status, value)
}

func writeJSONStatus(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeOAuthError(w http.ResponseWriter, status int, code, description string) {
	writeJSON(w, status, map[string]string{"error": code, "error_description": description})
}

var loginTemplate = template.Must(template.New("login").Parse(pageStart + `
<main><div class="mark">T</div><p class="eyebrow">TA'AWUN / SECURE CONNECTION</p><h1>Sign in to continue</h1>
<p class="lede">Use your Taawun account. Your platform token is never shared with the requesting client.</p>
{{if .Error}}<p class="error">{{.Error}}</p>{{end}}
<form method="post" action="/oauth/login"><input type="hidden" name="request_id" value="{{.RequestID}}">
<label>Email<input required autocomplete="username" type="email" name="email"></label>
<label>Password<input required autocomplete="current-password" type="password" name="password"></label>
<button type="submit">Continue <span>→</span></button></form></main>` + pageEnd))

var consentTemplate = template.Must(template.New("consent").Funcs(template.FuncMap{"loopback": func(raw string) bool { u, _ := url.Parse(raw); return u != nil && isLoopbackHost(u.Hostname()) }, "host": func(raw string) string {
	u, _ := url.Parse(raw)
	if u == nil {
		return ""
	}
	return u.Host
}}).Parse(pageStart + `
<main><div class="mark">T</div><p class="eyebrow">TA'AWUN / AUTHORIZE MCP CLIENT</p><h1>{{.Client.Name}}</h1>
<p class="lede">Signed in as <strong>{{.User.Email}}</strong>. Review exactly what this client will receive.</p>
<section><small>CLIENT ID</small><code>{{.Client.ID}}</code><small>REDIRECT HOST</small><strong class="host">{{host .Request.RedirectURI}}</strong><code>{{.Request.RedirectURI}}</code>
{{if loopback .Request.RedirectURI}}<p class="warning">Loopback warning: another local application could try to claim this callback port. Continue only if you initiated this connection.</p>{{end}}</section>
<form method="post" action="/oauth/consent"><input type="hidden" name="request_id" value="{{.RequestID}}"><input type="hidden" name="csrf" value="{{.CSRF}}">
<fieldset><legend>Requested permissions</legend>{{range .Request.Scopes}}<div class="scope">{{.}}</div>{{end}}</fieldset>
<fieldset><legend>Choose workspaces</legend>{{range .Workspaces}}<label class="check"><input type="checkbox" name="workspace_id" value="{{.ID}}"> <span>{{.Name}}</span></label>{{else}}<p>No eligible workspaces are available.</p>{{end}}</fieldset>
<div class="actions"><button name="decision" value="approve" type="submit">Authorize <span>→</span></button><button class="quiet" name="decision" value="deny" type="submit">Deny</button></div></form></main>` + pageEnd))

const pageStart = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Taawun authorization</title><style>
:root{color-scheme:light;--ink:#111;--paper:#f4f1e8;--accent:#d84b2a;--line:#111}*{box-sizing:border-box}body{margin:0;background:var(--paper);color:var(--ink);font:16px/1.45 Arial,sans-serif}main{width:min(680px,calc(100% - 32px));margin:7vh auto;border:2px solid var(--line);padding:clamp(24px,6vw,56px);box-shadow:10px 10px 0 var(--ink)}.mark{width:46px;height:46px;display:grid;place-items:center;background:var(--ink);color:var(--paper);font-weight:900;font-size:25px}.eyebrow,small,legend{font-size:12px;font-weight:800;letter-spacing:.12em;text-transform:uppercase}h1{font-size:clamp(35px,7vw,64px);line-height:.98;margin:24px 0}.lede{font-size:18px}form,section{display:grid;gap:18px;margin-top:28px}label{font-weight:800}input[type=email],input[type=password]{display:block;width:100%;margin-top:6px;padding:14px;border:2px solid var(--line);background:#fff;font:inherit}button{border:2px solid var(--ink);background:var(--ink);color:#fff;padding:14px 18px;font-weight:900;font-size:16px;cursor:pointer;display:flex;justify-content:space-between}.quiet{background:transparent;color:var(--ink)}fieldset,section{border:2px solid var(--line);padding:18px}.scope{border-top:1px solid #777;padding:10px 0;font-family:monospace}.check{display:flex;gap:10px;margin:10px 0}.check input{width:20px;height:20px}.actions{display:grid;grid-template-columns:1fr 1fr;gap:12px}.host{font-size:22px;overflow-wrap:anywhere}code{overflow-wrap:anywhere}.warning,.error{padding:12px;background:#ffd8cc;border-left:6px solid var(--accent);font-weight:700}small{display:block;margin-top:8px}</style></head><body>`
const pageEnd = `</body></html>`

func renderLogin(w http.ResponseWriter, requestID, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
	w.Header().Set("X-Frame-Options", "DENY")
	_ = loginTemplate.Execute(w, struct{ RequestID, Error string }{requestID, message})
}

func renderConsent(w http.ResponseWriter, view *ConsentView) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
	w.Header().Set("X-Frame-Options", "DENY")
	_ = consentTemplate.Execute(w, view)
}

func renderError(w http.ResponseWriter, status int, title, detail string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")
	w.Header().Set("X-Frame-Options", "DENY")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, "<!doctype html><title>%s</title><h1>%s</h1><p>%s</p>", template.HTMLEscapeString(title), template.HTMLEscapeString(title), template.HTMLEscapeString(detail))
}
