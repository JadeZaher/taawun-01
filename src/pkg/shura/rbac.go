package shura

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	"taawun/pkg/models"
)

const (
	CapabilityType     = "TAWUN-SHURA-CAP"
	CapabilityAudience = "taawun-shura"
	maxCapabilityTTL   = 30 * time.Minute
	maxClockSkew       = 2 * time.Minute
)

var (
	ErrCapabilityInvalid    = errors.New("invalid Shura capability")
	ErrCapabilityExpired    = errors.New("Shura capability expired")
	ErrCapabilityNotActive  = errors.New("Shura capability is not active")
	ErrCapabilityForbidden  = errors.New("Shura capability forbidden")
	ErrRevocationCheck      = errors.New("Shura revocation check failed")
	ErrWorkspaceDenied      = errors.New("Shura workspace authorization denied")
	ErrGovernanceInvalid    = errors.New("invalid Shura governance request")
	ErrGovernanceNotFound   = errors.New("Shura governance record not found")
	ErrGovernanceConflict   = errors.New("Shura governance version conflict")
	ErrGovernanceTransition = errors.New("invalid Shura governance transition")
)

type Role string

const (
	RoleArchitect  Role = "Architect"
	RoleMaintainer Role = "Maintainer"
	RoleViewer     Role = "Viewer"
)

type Scope string

const (
	ScopeRead             Scope = "shura:read"
	ScopeDeliberate       Scope = "shura:deliberate"
	ScopeVote             Scope = "shura:vote"
	ScopePropose          Scope = "shura:propose"
	ScopeDecide           Scope = "shura:decide"
	ScopeInvite           Scope = "shura:invite"
	ScopeCapabilityIssue  Scope = "shura:capability.issue"
	ScopeCapabilityRevoke Scope = "shura:capability.revoke"
)

var roleScopes = map[Role]map[Scope]struct{}{
	RoleViewer: {
		ScopeRead: {},
	},
	RoleMaintainer: {
		ScopeRead: {}, ScopeDeliberate: {}, ScopeVote: {}, ScopePropose: {},
	},
	RoleArchitect: {
		ScopeRead: {}, ScopeDeliberate: {}, ScopeVote: {}, ScopePropose: {}, ScopeDecide: {},
		ScopeInvite: {}, ScopeCapabilityIssue: {}, ScopeCapabilityRevoke: {},
	},
}

// CapabilityClaims is the canonical signed authorization payload.
type CapabilityClaims struct {
	KeyID       string  `json:"kid"`
	Issuer      string  `json:"iss"`
	Audience    string  `json:"aud"`
	TokenID     string  `json:"jti"`
	Subject     string  `json:"sub"`
	WorkspaceID int     `json:"workspace_id"`
	Role        Role    `json:"role"`
	Scopes      []Scope `json:"scopes"`
	IssuedAt    int64   `json:"iat"`
	NotBefore   int64   `json:"nbf"`
	ExpiresAt   int64   `json:"exp"`
}

type capabilityHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
	KeyID     string `json:"kid"`
}

// CapabilityIssuer signs with a stable injected Ed25519 key.
type CapabilityIssuer struct {
	issuer     string
	keyID      string
	privateKey ed25519.PrivateKey
}

func NewCapabilityIssuer(issuer, keyID string, privateKey ed25519.PrivateKey) (*CapabilityIssuer, error) {
	if !validIssuer(issuer) || !validIdentifier(keyID) || len(privateKey) != ed25519.PrivateKeySize {
		return nil, ErrCapabilityInvalid
	}
	return &CapabilityIssuer{issuer: issuer, keyID: keyID, privateKey: append(ed25519.PrivateKey(nil), privateKey...)}, nil
}

func (i *CapabilityIssuer) PublicKey() ed25519.PublicKey {
	if i == nil || len(i.privateKey) != ed25519.PrivateKeySize {
		return nil
	}
	key := i.privateKey.Public().(ed25519.PublicKey)
	return append(ed25519.PublicKey(nil), key...)
}

func (i *CapabilityIssuer) KeyMetadata() JWKSet {
	return JWKSet{Keys: []JWK{{KeyType: "OKP", Curve: "Ed25519", Use: "sig", Algorithm: "EdDSA", KeyID: i.keyID, X: base64.RawURLEncoding.EncodeToString(i.PublicKey())}}}
}

func (i *CapabilityIssuer) sign(claims CapabilityClaims) (string, error) {
	if i == nil || len(i.privateKey) != ed25519.PrivateKeySize || claims.Issuer != i.issuer || claims.KeyID != i.keyID {
		return "", ErrCapabilityInvalid
	}
	headerJSON, err := json.Marshal(capabilityHeader{Algorithm: "EdDSA", Type: CapabilityType, KeyID: i.keyID})
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	encodedHeader := base64.RawURLEncoding.EncodeToString(headerJSON)
	encodedClaims := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signingInput := encodedHeader + "." + encodedClaims
	signature := ed25519.Sign(i.privateKey, []byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

type JWKSet struct {
	Keys []JWK `json:"keys"`
}

type JWK struct {
	KeyType   string `json:"kty"`
	Curve     string `json:"crv"`
	Use       string `json:"use"`
	Algorithm string `json:"alg"`
	KeyID     string `json:"kid"`
	X         string `json:"x"`
}

type TrustedKeyRegistry interface {
	LookupCapabilityKey(context.Context, string, string) (ed25519.PublicKey, error)
}

type StaticKeyRegistry struct {
	keys map[string]ed25519.PublicKey
}

func NewStaticKeyRegistry(issuer, keyID string, publicKey ed25519.PublicKey) (*StaticKeyRegistry, error) {
	if !validIssuer(issuer) || !validIdentifier(keyID) || len(publicKey) != ed25519.PublicKeySize {
		return nil, ErrCapabilityInvalid
	}
	return &StaticKeyRegistry{keys: map[string]ed25519.PublicKey{issuer + "\x00" + keyID: append(ed25519.PublicKey(nil), publicKey...)}}, nil
}

func (r *StaticKeyRegistry) LookupCapabilityKey(_ context.Context, issuer, keyID string) (ed25519.PublicKey, error) {
	if r == nil {
		return nil, ErrCapabilityInvalid
	}
	key, ok := r.keys[issuer+"\x00"+keyID]
	if !ok {
		return nil, ErrCapabilityInvalid
	}
	return append(ed25519.PublicKey(nil), key...), nil
}

// WorkspaceAuthorizer preserves WorkspaceService as the membership authority.
type WorkspaceAuthorizer interface {
	AuthorizeWorkspaceCapability(*models.User, int, models.WorkspaceCapability) (*models.Workspace, error)
}

type CapabilityVerifier struct {
	registry   TrustedKeyRegistry
	repository *Repository
	workspaces WorkspaceAuthorizer
	now        func() time.Time
	skew       time.Duration
}

func NewCapabilityVerifier(registry TrustedKeyRegistry, repository *Repository, workspaces WorkspaceAuthorizer) (*CapabilityVerifier, error) {
	if registry == nil || repository == nil || workspaces == nil {
		return nil, ErrCapabilityInvalid
	}
	return &CapabilityVerifier{registry: registry, repository: repository, workspaces: workspaces, now: time.Now, skew: maxClockSkew}, nil
}

// Verify performs signature, binding, time, least-privilege, revocation, and membership checks.
func (v *CapabilityVerifier) Verify(ctx context.Context, raw string, workspaceID int, audience string, action Scope) (*CapabilityClaims, error) {
	header, claims, signingInput, signature, err := parseCapability(raw)
	if err != nil {
		return nil, err
	}
	if header.KeyID != claims.KeyID || header.Algorithm != "EdDSA" || header.Type != CapabilityType {
		return nil, ErrCapabilityInvalid
	}
	key, err := v.registry.LookupCapabilityKey(ctx, claims.Issuer, claims.KeyID)
	if err != nil || len(key) != ed25519.PublicKeySize || !ed25519.Verify(key, []byte(signingInput), signature) {
		return nil, ErrCapabilityInvalid
	}
	if claims.WorkspaceID != workspaceID || claims.Audience != audience || audience != CapabilityAudience || !validClaims(claims) {
		return nil, ErrCapabilityInvalid
	}
	now := v.now().UTC()
	if now.Add(v.skew).Unix() < claims.NotBefore || now.Add(v.skew).Unix() < claims.IssuedAt {
		return nil, ErrCapabilityNotActive
	}
	if now.Add(-v.skew).Unix() >= claims.ExpiresAt {
		return nil, ErrCapabilityExpired
	}
	if !scopeAllowed(claims.Role, action) || !containsScope(claims.Scopes, action) {
		return nil, ErrCapabilityForbidden
	}
	hash := sha256.Sum256([]byte(raw))
	active, err := v.repository.capabilityActive(ctx, claims.TokenID, hex.EncodeToString(hash[:]))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRevocationCheck, err)
	}
	if !active {
		return nil, ErrCapabilityNotActive
	}
	subjectID, err := strconv.Atoi(claims.Subject)
	if err != nil || subjectID <= 0 {
		return nil, ErrCapabilityInvalid
	}
	if err := authorizeWorkspaceRole(v.workspaces, &models.User{ID: subjectID}, claims.WorkspaceID, claims.Role); err != nil {
		return nil, err
	}
	copy := claims
	copy.Scopes = append([]Scope(nil), claims.Scopes...)
	return &copy, nil
}

func parseCapability(raw string) (capabilityHeader, CapabilityClaims, string, []byte, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 || len(raw) > 8192 {
		return capabilityHeader{}, CapabilityClaims{}, "", nil, ErrCapabilityInvalid
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return capabilityHeader{}, CapabilityClaims{}, "", nil, ErrCapabilityInvalid
	}
	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return capabilityHeader{}, CapabilityClaims{}, "", nil, ErrCapabilityInvalid
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(signature) != ed25519.SignatureSize {
		return capabilityHeader{}, CapabilityClaims{}, "", nil, ErrCapabilityInvalid
	}
	if base64.RawURLEncoding.EncodeToString(headerBytes) != parts[0] ||
		base64.RawURLEncoding.EncodeToString(claimsBytes) != parts[1] ||
		base64.RawURLEncoding.EncodeToString(signature) != parts[2] {
		return capabilityHeader{}, CapabilityClaims{}, "", nil, ErrCapabilityInvalid
	}
	var header capabilityHeader
	var claims CapabilityClaims
	if strictJSON(headerBytes, &header) != nil || strictJSON(claimsBytes, &claims) != nil {
		return capabilityHeader{}, CapabilityClaims{}, "", nil, ErrCapabilityInvalid
	}
	canonicalHeader, _ := json.Marshal(header)
	canonicalClaims, _ := json.Marshal(claims)
	if !bytes.Equal(headerBytes, canonicalHeader) || !bytes.Equal(claimsBytes, canonicalClaims) {
		return capabilityHeader{}, CapabilityClaims{}, "", nil, ErrCapabilityInvalid
	}
	return header, claims, parts[0] + "." + parts[1], signature, nil
}

func validClaims(claims CapabilityClaims) bool {
	if !validIdentifier(claims.KeyID) || !validIssuer(claims.Issuer) || !validIdentifier(claims.TokenID) || claims.Subject == "" || claims.WorkspaceID <= 0 || !validRole(claims.Role) {
		return false
	}
	if claims.IssuedAt <= 0 || claims.NotBefore < claims.IssuedAt || claims.ExpiresAt <= claims.NotBefore || time.Duration(claims.ExpiresAt-claims.IssuedAt)*time.Second > maxCapabilityTTL {
		return false
	}
	if len(claims.Scopes) == 0 || len(claims.Scopes) > len(roleScopes[RoleArchitect]) {
		return false
	}
	seen := make(map[Scope]struct{}, len(claims.Scopes))
	for _, scope := range claims.Scopes {
		if !scopeAllowed(claims.Role, scope) {
			return false
		}
		if _, duplicate := seen[scope]; duplicate {
			return false
		}
		seen[scope] = struct{}{}
	}
	return true
}

func authorizeWorkspaceRole(authorizer WorkspaceAuthorizer, actor *models.User, workspaceID int, role Role) error {
	capability := models.WorkspaceCapabilityView
	switch role {
	case RoleArchitect:
		capability = models.WorkspaceCapabilityPublish
	case RoleMaintainer:
		capability = models.WorkspaceCapabilityBuild
	case RoleViewer:
	default:
		return ErrCapabilityForbidden
	}
	if _, err := authorizer.AuthorizeWorkspaceCapability(actor, workspaceID, capability); err != nil {
		return fmt.Errorf("%w: %v", ErrWorkspaceDenied, err)
	}
	return nil
}

func scopeAllowed(role Role, scope Scope) bool {
	_, ok := roleScopes[role][scope]
	return ok
}

func containsScope(scopes []Scope, expected Scope) bool {
	for _, scope := range scopes {
		if scope == expected {
			return true
		}
	}
	return false
}

func validRole(role Role) bool {
	_, ok := roleScopes[role]
	return ok
}

func validIdentifier(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || strings.ContainsRune("._:-", character) {
			continue
		}
		return false
	}
	return true
}

func validIssuer(value string) bool {
	if validIdentifier(value) {
		return true
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil && parsed.Fragment == "" && len(value) <= 512
}

func strictJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ErrCapabilityInvalid
	}
	return nil
}
