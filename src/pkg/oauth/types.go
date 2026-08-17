// Package oauth implements Taawun's OAuth 2.1 authorization boundary for remote MCP.
// See AGENTS.md for protocol decisions and security invariants.
package oauth

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"taawun/pkg/models"
)

const (
	ScopeRead    = "taawun:read"
	ScopeBuild   = "taawun:build"
	ScopePublish = "taawun:publish"
)

var (
	ErrInvalidGrant = errors.New("invalid grant")
	ErrInvalidToken = errors.New("invalid access token")
	ErrReplay       = errors.New("refresh token replay detected")
	ErrCapacity     = errors.New("OAuth capacity reached")
)

var supportedScopes = []string{ScopeRead, ScopeBuild, ScopePublish}

type Config struct {
	Issuer          string
	Resource        string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	CodeTTL         time.Duration
	RequestTTL      time.Duration
	SessionTTL      time.Duration
	Now             func() time.Time
}

func (c Config) normalized() (Config, error) {
	c.Issuer = strings.TrimRight(strings.TrimSpace(c.Issuer), "/")
	c.Resource = strings.TrimSpace(c.Resource)
	if c.AccessTokenTTL <= 0 {
		c.AccessTokenTTL = 10 * time.Minute
	}
	if c.RefreshTokenTTL <= 0 {
		c.RefreshTokenTTL = 30 * 24 * time.Hour
	}
	if c.CodeTTL <= 0 {
		c.CodeTTL = 5 * time.Minute
	}
	if c.RequestTTL <= 0 {
		c.RequestTTL = 10 * time.Minute
	}
	if c.SessionTTL <= 0 {
		c.SessionTTL = 15 * time.Minute
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	issuer, err := parseEndpoint(c.Issuer, true)
	if err != nil || (issuer.Path != "" && issuer.Path != "/") || issuer.RawQuery != "" || issuer.Fragment != "" {
		return Config{}, fmt.Errorf("OAuth issuer must be an HTTPS URL (HTTP is allowed only on loopback)")
	}
	resource, err := parseEndpoint(c.Resource, true)
	if err != nil || resource.RawQuery != "" || resource.Fragment != "" {
		return Config{}, fmt.Errorf("OAuth resource must be an HTTPS URL without query or fragment")
	}
	return c, nil
}

func parseEndpoint(raw string, allowLoopbackHTTP bool) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || u.Host == "" || u.User != nil {
		return nil, errors.New("absolute URL required")
	}
	if u.Scheme == "https" {
		return u, nil
	}
	if allowLoopbackHTTP && u.Scheme == "http" && isLoopbackHost(u.Hostname()) {
		return u, nil
	}
	return nil, errors.New("HTTPS required")
}

func validRedirectURI(raw string) bool {
	if strings.Contains(raw, "*") {
		return false
	}
	u, err := parseEndpoint(raw, true)
	return err == nil && u.Fragment == ""
}

func isLoopbackHost(host string) bool {
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

func normalizeScopes(raw string) ([]string, error) {
	seen := make(map[string]bool)
	for _, scope := range strings.Fields(raw) {
		switch scope {
		case ScopeRead, ScopeBuild, ScopePublish:
			seen[scope] = true
		default:
			return nil, fmt.Errorf("unsupported scope %q", scope)
		}
	}
	if len(seen) == 0 {
		return nil, errors.New("at least one scope is required")
	}
	result := make([]string, 0, len(seen))
	for _, scope := range supportedScopes {
		if seen[scope] {
			result = append(result, scope)
		}
	}
	return result, nil
}

func sortedUniqueInts(values []int) ([]int, error) {
	seen := make(map[int]bool)
	for _, value := range values {
		if value <= 0 {
			return nil, errors.New("invalid workspace")
		}
		seen[value] = true
	}
	result := make([]int, 0, len(seen))
	for value := range seen {
		result = append(result, value)
	}
	sort.Ints(result)
	if len(result) == 0 {
		return nil, errors.New("at least one workspace is required")
	}
	return result, nil
}

type Identity interface {
	Authenticate(email, password string) (*models.User, error)
}

type UserLoader interface {
	GetByID(id int) (*models.User, error)
}

type WorkspaceAuthority interface {
	GetWorkspaces(*models.User) ([]*models.Workspace, error)
	AuthorizeWorkspaceCapability(*models.User, int, models.WorkspaceCapability) (*models.Workspace, error)
}

type UserBinder func(context.Context, *models.User) context.Context

type Client struct {
	ID                      string
	Name                    string
	RedirectURIs            []string
	GrantTypes              []string
	ResponseTypes           []string
	TokenEndpointAuthMethod string
	ClientURI               string
	Disabled                bool
}

type Principal struct {
	User         *models.User
	ClientID     string
	Resource     string
	Scopes       []string
	WorkspaceIDs []int
	TokenID      int64
}

type principalContextKey struct{}

func PrincipalFromContext(ctx context.Context) (*Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(*Principal)
	return principal, ok && principal != nil
}

func HasScope(ctx context.Context, required string) bool {
	principal, ok := PrincipalFromContext(ctx)
	if !ok {
		return false
	}
	for _, scope := range principal.Scopes {
		if scope == required {
			return true
		}
	}
	return false
}

func WorkspaceGranted(ctx context.Context, workspaceID int) bool {
	principal, ok := PrincipalFromContext(ctx)
	if !ok {
		return false
	}
	for _, id := range principal.WorkspaceIDs {
		if id == workspaceID {
			return true
		}
	}
	return false
}
