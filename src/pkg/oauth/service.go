package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"taawun/pkg/models"
)

var pkceValuePattern = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)

type Service struct {
	repository *Repository
	identity   Identity
	users      UserLoader
	workspaces WorkspaceAuthority
	bindUser   UserBinder
	config     Config
	metadata   *clientMetadataResolver
}

type AuthorizationInput struct {
	ClientID            string
	RedirectURI         string
	ResponseType        string
	Scope               string
	State               string
	Resource            string
	CodeChallenge       string
	CodeChallengeMethod string
}

type ConsentView struct {
	RequestID  string
	CSRF       string
	Client     *Client
	User       *models.User
	Request    *authorizationRequest
	Workspaces []*models.Workspace
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope"`
}

func NewService(repository *Repository, identity Identity, users UserLoader, workspaces WorkspaceAuthority, bindUser UserBinder, config Config) (*Service, error) {
	if repository == nil || identity == nil || users == nil || workspaces == nil || bindUser == nil {
		return nil, errors.New("OAuth repository, identity, user loader, workspace authority, and user binder are required")
	}
	normalized, err := config.normalized()
	if err != nil {
		return nil, err
	}
	return &Service{repository: repository, identity: identity, users: users, workspaces: workspaces, bindUser: bindUser, config: normalized, metadata: newClientMetadataResolver(repository, normalized.Now)}, nil
}

func randomCredential(prefix string) (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(bytes), nil
}

func credentialHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func containsExact(values []string, target string) bool {
	return slices.Contains(values, target)
}

func (s *Service) endpoint(path string) string {
	return s.config.Issuer + path
}

func (s *Service) RegisterClient(client Client) (*Client, error) {
	validated, err := validateClientDefinition(client)
	if err != nil {
		return nil, err
	}
	client = validated
	id, err := randomCredential("taawun_client_")
	if err != nil {
		return nil, err
	}
	client.ID = id
	if err := s.repository.createClient(client, s.config.Now().UTC()); err != nil {
		return nil, err
	}
	return &client, nil
}

func validateClientDefinition(client Client) (Client, error) {
	client.Name = strings.TrimSpace(client.Name)
	client.ClientURI = strings.TrimSpace(client.ClientURI)
	if client.Name == "" || len(client.Name) > 100 {
		return Client{}, errors.New("client_name is required and must not exceed 100 characters")
	}
	if len(client.RedirectURIs) == 0 || len(client.RedirectURIs) > 10 {
		return Client{}, errors.New("between one and ten redirect_uris are required")
	}
	seen := make(map[string]bool)
	for _, redirect := range client.RedirectURIs {
		if !validRedirectURI(redirect) || seen[redirect] {
			return Client{}, fmt.Errorf("invalid or duplicate redirect_uri %q", redirect)
		}
		seen[redirect] = true
	}
	if client.ClientURI != "" {
		u, err := parseEndpoint(client.ClientURI, false)
		if err != nil || u.Fragment != "" {
			return Client{}, errors.New("client_uri must be HTTPS")
		}
	}
	if len(client.GrantTypes) == 0 {
		client.GrantTypes = []string{"authorization_code", "refresh_token"}
	}
	for _, grant := range client.GrantTypes {
		if grant != "authorization_code" && grant != "refresh_token" {
			return Client{}, errors.New("only authorization_code and refresh_token grants are supported")
		}
	}
	if !containsExact(client.GrantTypes, "authorization_code") {
		return Client{}, errors.New("authorization_code grant is required")
	}
	if len(client.ResponseTypes) == 0 {
		client.ResponseTypes = []string{"code"}
	}
	if len(client.ResponseTypes) != 1 || client.ResponseTypes[0] != "code" {
		return Client{}, errors.New("only the code response type is supported")
	}
	if client.TokenEndpointAuthMethod == "" {
		client.TokenEndpointAuthMethod = "none"
	}
	if client.TokenEndpointAuthMethod != "none" {
		return Client{}, errors.New("only public clients with token_endpoint_auth_method none are supported")
	}
	return client, nil
}

func (s *Service) resolveClient(clientID string) (*Client, error) {
	if isMetadataClientID(clientID) {
		client, err := s.metadata.resolve(context.Background(), clientID)
		if err != nil {
			return nil, err
		}
		stored, err := s.repository.getClient(clientID)
		if err != nil {
			return nil, err
		}
		if stored != nil && stored.Disabled {
			client.Disabled = true
		}
		return client, nil
	}
	return s.repository.getClient(clientID)
}

func (s *Service) BeginAuthorization(input AuthorizationInput) (string, error) {
	client, err := s.resolveClient(input.ClientID)
	if err != nil || client == nil || client.Disabled {
		return "", errors.New("unknown client")
	}
	if input.ResponseType != "code" {
		return "", errors.New("response_type must be code")
	}
	if !containsExact(client.RedirectURIs, input.RedirectURI) {
		return "", errors.New("redirect_uri is not registered")
	}
	if input.Resource != s.config.Resource {
		return "", errors.New("resource must exactly match the MCP resource")
	}
	if input.CodeChallengeMethod != "S256" || !pkceValuePattern.MatchString(input.CodeChallenge) {
		return "", errors.New("PKCE S256 code_challenge is required")
	}
	if input.State == "" || len(input.State) > 512 {
		return "", errors.New("state is required")
	}
	scopes, err := normalizeScopes(input.Scope)
	if err != nil {
		return "", err
	}
	raw, err := randomCredential("taawun_req_")
	if err != nil {
		return "", err
	}
	request := authorizationRequest{
		Hash: credentialHash(raw), ClientID: input.ClientID, RedirectURI: input.RedirectURI,
		Resource: input.Resource, Scopes: scopes, State: input.State, CodeChallenge: input.CodeChallenge,
		ExpiresAt: s.config.Now().UTC().Add(s.config.RequestTTL),
	}
	if err := s.repository.createAuthorizationRequest(request); err != nil {
		return "", err
	}
	return raw, nil
}

func (s *Service) LoginAuthorization(requestID, email, password string) (sessionToken, csrfToken string, err error) {
	request, err := s.activeRequest(requestID)
	if err != nil || request == nil {
		return "", "", ErrInvalidGrant
	}
	user, err := s.identity.Authenticate(email, password)
	if err != nil {
		return "", "", err
	}
	sessionToken, err = randomCredential("taawun_session_")
	if err != nil {
		return "", "", err
	}
	csrfToken, err = randomCredential("taawun_csrf_")
	if err != nil {
		return "", "", err
	}
	session := browserSession{Hash: credentialHash(sessionToken), UserID: user.ID, CSRFHash: credentialHash(csrfToken), ExpiresAt: s.config.Now().UTC().Add(s.config.SessionTTL)}
	if err := s.repository.createSession(session); err != nil {
		return "", "", err
	}
	return sessionToken, csrfToken, nil
}

func (s *Service) activeRequest(raw string) (*authorizationRequest, error) {
	if raw == "" || len(raw) > 256 {
		return nil, ErrInvalidGrant
	}
	request, err := s.repository.getAuthorizationRequest(credentialHash(raw))
	if err != nil || request == nil || request.Consumed || !request.ExpiresAt.After(s.config.Now().UTC()) {
		return nil, ErrInvalidGrant
	}
	return request, nil
}

func (s *Service) sessionUser(raw string) (*browserSession, *models.User, error) {
	if raw == "" || len(raw) > 256 {
		return nil, nil, ErrInvalidGrant
	}
	session, err := s.repository.getSession(credentialHash(raw))
	if err != nil || session == nil || session.Revoked || !session.ExpiresAt.After(s.config.Now().UTC()) {
		return nil, nil, ErrInvalidGrant
	}
	user, err := s.users.GetByID(session.UserID)
	if err != nil || user == nil || user.Status != models.StatusActive {
		return nil, nil, ErrInvalidGrant
	}
	return session, user, nil
}

func (s *Service) ConsentView(requestID, sessionToken, csrfToken string) (*ConsentView, error) {
	request, err := s.activeRequest(requestID)
	if err != nil {
		return nil, err
	}
	session, user, err := s.sessionUser(sessionToken)
	if err != nil {
		return nil, err
	}
	if csrfToken == "" {
		csrfToken, err = randomCredential("taawun_csrf_")
		if err != nil {
			return nil, err
		}
		if err := s.repository.updateSessionCSRF(session.Hash, credentialHash(csrfToken), s.config.Now().UTC()); err != nil {
			return nil, err
		}
		session.CSRFHash = credentialHash(csrfToken)
	} else if subtle.ConstantTimeCompare([]byte(session.CSRFHash), []byte(credentialHash(csrfToken))) != 1 {
		return nil, ErrInvalidGrant
	}
	client, err := s.resolveClient(request.ClientID)
	if err != nil || client == nil || client.Disabled {
		return nil, ErrInvalidGrant
	}
	workspaces, err := s.workspaces.GetWorkspaces(user)
	if err != nil {
		return nil, err
	}
	return &ConsentView{RequestID: requestID, CSRF: csrfToken, Client: client, User: user, Request: request, Workspaces: workspaces}, nil
}

func capabilityForScope(scope string) models.WorkspaceCapability {
	switch scope {
	case ScopePublish:
		return models.WorkspaceCapabilityPublish
	case ScopeBuild:
		return models.WorkspaceCapabilityBuild
	default:
		return models.WorkspaceCapabilityView
	}
}

func (s *Service) authorizeWorkspaceGrants(user *models.User, scopes []string, workspaceIDs []int) error {
	for _, workspaceID := range workspaceIDs {
		for _, scope := range scopes {
			if _, err := s.workspaces.AuthorizeWorkspaceCapability(user, workspaceID, capabilityForScope(scope)); err != nil {
				return errors.New("requested scope is not allowed for a selected workspace")
			}
		}
	}
	return nil
}

func (s *Service) Approve(requestID, sessionToken, csrfToken string, workspaceIDs []int) (string, error) {
	request, err := s.activeRequest(requestID)
	if err != nil {
		return "", err
	}
	session, user, err := s.sessionUser(sessionToken)
	if err != nil {
		return "", err
	}
	if subtle.ConstantTimeCompare([]byte(session.CSRFHash), []byte(credentialHash(csrfToken))) != 1 {
		return "", ErrInvalidGrant
	}
	workspaceIDs, err = sortedUniqueInts(workspaceIDs)
	if err != nil {
		return "", err
	}
	if err := s.authorizeWorkspaceGrants(user, request.Scopes, workspaceIDs); err != nil {
		return "", err
	}
	rawCode, err := randomCredential("taawun_code_")
	if err != nil {
		return "", err
	}
	record := consent{UserID: user.ID, ClientID: request.ClientID, Resource: request.Resource, Scopes: request.Scopes, WorkspaceIDs: workspaceIDs}
	code := authorizationCode{Hash: credentialHash(rawCode), UserID: user.ID, ClientID: request.ClientID, RedirectURI: request.RedirectURI, Resource: request.Resource, Scopes: request.Scopes, WorkspaceIDs: workspaceIDs, CodeChallenge: request.CodeChallenge, ExpiresAt: s.config.Now().UTC().Add(s.config.CodeTTL)}
	if err := s.repository.approve(credentialHash(requestID), record, code, s.config.Now().UTC()); err != nil {
		return "", err
	}
	return authorizationRedirect(request.RedirectURI, rawCode, request.State, s.config.Issuer), nil
}

func authorizationRedirect(redirectURI, code, state, issuer string) string {
	u, _ := url.Parse(redirectURI)
	query := u.Query()
	query.Set("code", code)
	query.Set("state", state)
	query.Set("iss", issuer)
	u.RawQuery = query.Encode()
	return u.String()
}

func (s *Service) Deny(requestID string) (string, error) {
	request, err := s.activeRequest(requestID)
	if err != nil {
		return "", err
	}
	if err := s.repository.consumeAuthorizationRequest(credentialHash(requestID), s.config.Now().UTC()); err != nil {
		return "", err
	}
	u, _ := url.Parse(request.RedirectURI)
	query := u.Query()
	query.Set("error", "access_denied")
	query.Set("state", request.State)
	query.Set("iss", s.config.Issuer)
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func pkceChallenge(verifier string) (string, error) {
	if !pkceValuePattern.MatchString(verifier) {
		return "", errors.New("invalid code_verifier")
	}
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

func (s *Service) ExchangeCode(codeRaw, clientID, redirectURI, resource, verifier string) (*TokenResponse, error) {
	if resource != s.config.Resource {
		return nil, ErrInvalidGrant
	}
	client, err := s.resolveClient(clientID)
	if err != nil || client == nil || client.Disabled || !containsExact(client.RedirectURIs, redirectURI) {
		return nil, ErrInvalidGrant
	}
	challenge, err := pkceChallenge(verifier)
	if err != nil {
		return nil, ErrInvalidGrant
	}
	pending, err := s.repository.getAuthorizationCode(credentialHash(codeRaw))
	if err != nil || pending == nil || pending.Consumed || !pending.ExpiresAt.After(s.config.Now().UTC()) || pending.ClientID != clientID || pending.RedirectURI != redirectURI || pending.Resource != resource || pending.CodeChallenge != challenge {
		return nil, ErrInvalidGrant
	}
	active, err := s.repository.consentActive(pending.ConsentID)
	if err != nil || !active {
		return nil, ErrInvalidGrant
	}
	user, err := s.users.GetByID(pending.UserID)
	if err != nil || user == nil || user.Status != models.StatusActive || s.authorizeWorkspaceGrants(user, pending.Scopes, pending.WorkspaceIDs) != nil {
		return nil, ErrInvalidGrant
	}
	accessRaw, err := randomCredential("taawun_at_")
	if err != nil {
		return nil, err
	}
	refreshRaw, err := randomCredential("taawun_rt_")
	if err != nil {
		return nil, err
	}
	family, err := randomCredential("taawun_family_")
	if err != nil {
		return nil, err
	}
	now := s.config.Now().UTC()
	tokens := issuedTokens{AccessHash: credentialHash(accessRaw), RefreshHash: credentialHash(refreshRaw), FamilyID: family, ClientID: clientID, Resource: resource, AccessExp: now.Add(s.config.AccessTokenTTL), RefreshExp: now.Add(s.config.RefreshTokenTTL)}
	expected := authorizationCode{ClientID: clientID, RedirectURI: redirectURI, Resource: resource, CodeChallenge: challenge}
	if err := s.repository.exchangeCode(credentialHash(codeRaw), expected, tokens, now); err != nil {
		return nil, ErrInvalidGrant
	}
	code, err := s.repository.getAccessToken(tokens.AccessHash)
	if err != nil || code == nil {
		return nil, errors.New("failed to load issued access token")
	}
	return &TokenResponse{AccessToken: accessRaw, TokenType: "Bearer", ExpiresIn: int64(s.config.AccessTokenTTL.Seconds()), RefreshToken: refreshRaw, Scope: strings.Join(code.Scopes, " ")}, nil
}

func (s *Service) Refresh(refreshRaw, clientID, resource string) (*TokenResponse, error) {
	if resource != s.config.Resource {
		return nil, ErrInvalidGrant
	}
	client, err := s.resolveClient(clientID)
	if err != nil || client == nil || client.Disabled || !containsExact(client.GrantTypes, "refresh_token") {
		return nil, ErrInvalidGrant
	}
	pending, err := s.repository.getRefreshToken(credentialHash(refreshRaw))
	if err != nil || pending == nil || pending.ClientID != clientID || pending.Resource != resource {
		return nil, ErrInvalidGrant
	}
	if !pending.Used && !pending.Revoked && pending.ExpiresAt.After(s.config.Now().UTC()) {
		active, activeErr := s.repository.consentActive(pending.ConsentID)
		user, userErr := s.users.GetByID(pending.UserID)
		if activeErr != nil || !active || userErr != nil || user == nil || user.Status != models.StatusActive || s.authorizeWorkspaceGrants(user, pending.Scopes, pending.WorkspaceIDs) != nil {
			_ = s.repository.revokeToken(credentialHash(refreshRaw), clientID, s.config.Now().UTC())
			return nil, ErrInvalidGrant
		}
	}
	accessRaw, err := randomCredential("taawun_at_")
	if err != nil {
		return nil, err
	}
	refreshNext, err := randomCredential("taawun_rt_")
	if err != nil {
		return nil, err
	}
	now := s.config.Now().UTC()
	next := issuedTokens{AccessHash: credentialHash(accessRaw), RefreshHash: credentialHash(refreshNext), AccessExp: now.Add(s.config.AccessTokenTTL), RefreshExp: now.Add(s.config.RefreshTokenTTL)}
	old, err := s.repository.rotateRefresh(credentialHash(refreshRaw), clientID, resource, next, now)
	if err != nil {
		return nil, ErrInvalidGrant
	}
	return &TokenResponse{AccessToken: accessRaw, TokenType: "Bearer", ExpiresIn: int64(s.config.AccessTokenTTL.Seconds()), RefreshToken: refreshNext, Scope: strings.Join(old.Scopes, " ")}, nil
}

func (s *Service) ValidateAccess(raw string) (*Principal, error) {
	if raw == "" || len(raw) > 256 {
		return nil, ErrInvalidToken
	}
	record, err := s.repository.getAccessToken(credentialHash(raw))
	if err != nil || record == nil || record.Revoked || record.Resource != s.config.Resource || !record.ExpiresAt.After(s.config.Now().UTC()) {
		return nil, ErrInvalidToken
	}
	active, err := s.repository.consentActive(record.ConsentID)
	if err != nil || !active {
		return nil, ErrInvalidToken
	}
	client, err := s.resolveClient(record.ClientID)
	if err != nil || client == nil || client.Disabled {
		return nil, ErrInvalidToken
	}
	user, err := s.users.GetByID(record.UserID)
	if err != nil || user == nil || user.Status != models.StatusActive {
		return nil, ErrInvalidToken
	}
	if err := s.authorizeWorkspaceGrants(user, record.Scopes, record.WorkspaceIDs); err != nil {
		return nil, ErrInvalidToken
	}
	return &Principal{User: user, ClientID: record.ClientID, Resource: record.Resource, Scopes: record.Scopes, WorkspaceIDs: record.WorkspaceIDs, TokenID: record.ID}, nil
}

func (s *Service) Revoke(raw, clientID string) error {
	client, err := s.resolveClient(clientID)
	if err != nil || client == nil || client.Disabled {
		return nil
	}
	return s.repository.revokeToken(credentialHash(raw), clientID, s.config.Now().UTC())
}

func (s *Service) AuthorizeScope(ctx context.Context, scope string) error {
	if !HasScope(ctx, scope) {
		return fmt.Errorf("OAuth scope %s is required", scope)
	}
	return nil
}

func (s *Service) AuthorizeWorkspaceGrant(ctx context.Context, workspaceID int) error {
	if !WorkspaceGranted(ctx, workspaceID) {
		return errors.New("workspace is not included in the OAuth consent grant")
	}
	return nil
}

func (s *Service) bindPrincipal(ctx context.Context, principal *Principal) context.Context {
	ctx = context.WithValue(ctx, principalContextKey{}, principal)
	return s.bindUser(ctx, principal.User)
}
