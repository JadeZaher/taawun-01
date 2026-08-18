package artifacts

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"
)

var (
	ErrInvalidSigningConfig = errors.New("invalid artifact signing configuration")
	ErrUntrustedSigner      = errors.New("artifact signer is not trusted")
	ErrInvalidSignature     = errors.New("artifact signature is invalid")
	ErrArtifactExpired      = errors.New("artifact authorization has expired")
)

var signedManifestFieldsV1 = []string{
	"allowedServerSignals",
	"appName",
	"artifactId",
	"authorization",
	"compliance",
	"contentHash",
	"contractVersion",
	"createdAt",
	"dataClassifications",
	"defaultEmbedBoundary",
	"files",
	"financial",
	"modules",
	"renderModes",
	"relay",
	"runtime",
	"security",
	"stateSplit",
	"template",
	"theme",
	"workspaceId",
}

var signedManifestFieldsV2 = func() []string {
	fields := append([]string(nil), signedManifestFieldsV1...)
	fields = append(fields, "components")
	sort.Strings(fields)
	return fields
}()

// SigningConfig injects a stable signer and trusted verification key registry.
type SigningConfig struct {
	KeyID       string
	PrivateKey  ed25519.PrivateKey
	TrustedKeys map[string]ed25519.PublicKey
}

func ephemeralSigningConfig() (SigningConfig, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return SigningConfig{}, err
	}
	digest := hex.EncodeToString(publicKey[:8])
	return SigningConfig{
		KeyID:       "ephemeral-" + digest,
		PrivateKey:  privateKey,
		TrustedKeys: map[string]ed25519.PublicKey{"ephemeral-" + digest: publicKey},
	}, nil
}

func validateSigningConfig(config SigningConfig) (SigningConfig, error) {
	if !validSubjectID(config.KeyID) || len(config.PrivateKey) != ed25519.PrivateKeySize {
		return SigningConfig{}, fmt.Errorf("%w: key id and Ed25519 private key are required", ErrInvalidSigningConfig)
	}
	publicKey, ok := config.PrivateKey.Public().(ed25519.PublicKey)
	if !ok || len(publicKey) != ed25519.PublicKeySize {
		return SigningConfig{}, fmt.Errorf("%w: private key has no Ed25519 public key", ErrInvalidSigningConfig)
	}
	trusted := make(map[string]ed25519.PublicKey, len(config.TrustedKeys)+1)
	for keyID, key := range config.TrustedKeys {
		if !validSubjectID(keyID) || len(key) != ed25519.PublicKeySize {
			return SigningConfig{}, fmt.Errorf("%w: invalid trusted key %q", ErrInvalidSigningConfig, keyID)
		}
		trusted[keyID] = append(ed25519.PublicKey(nil), key...)
	}
	if configured, exists := trusted[config.KeyID]; exists && !configured.Equal(publicKey) {
		return SigningConfig{}, fmt.Errorf("%w: signer key does not match trusted key %q", ErrInvalidSigningConfig, config.KeyID)
	}
	trusted[config.KeyID] = append(ed25519.PublicKey(nil), publicKey...)
	return SigningConfig{
		KeyID:       config.KeyID,
		PrivateKey:  append(ed25519.PrivateKey(nil), config.PrivateKey...),
		TrustedKeys: trusted,
	}, nil
}

func signManifest(manifest *Manifest, keyID string, privateKey ed25519.PrivateKey) error {
	manifest.Signature = BundleSignature{
		ContractVersion: SignatureContractVersion,
		Algorithm:       "Ed25519",
		KeyID:           keyID,
		SignedFields:    append([]string(nil), signedManifestFieldsV2...),
	}
	payload, err := signaturePayload(*manifest)
	if err != nil {
		return err
	}
	manifest.Signature.Value = base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, payload))
	return nil
}

// VerifyManifestSignature validates the canonical signature contract with a trusted key.
func VerifyManifestSignature(manifest Manifest, publicKey ed25519.PublicKey) error {
	expectedFields, validContract := signedFieldsForManifest(manifest)
	if len(publicKey) != ed25519.PublicKeySize || !validContract || manifest.Signature.Algorithm != "Ed25519" || manifest.Signature.KeyID == "" || manifest.Signature.KeyID != manifest.Authorization.SignerKeyID || !reflect.DeepEqual(manifest.Signature.SignedFields, expectedFields) {
		return ErrInvalidSignature
	}
	signature, err := base64.RawURLEncoding.DecodeString(manifest.Signature.Value)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return ErrInvalidSignature
	}
	payload, err := signaturePayload(manifest)
	if err != nil || !ed25519.Verify(publicKey, payload, signature) {
		return ErrInvalidSignature
	}
	return nil
}

func signedFieldsForManifest(manifest Manifest) ([]string, bool) {
	switch {
	case manifest.ContractVersion == ManifestContractVersionV1 && manifest.Signature.ContractVersion == SignatureContractVersionV1 && manifest.Authorization.Version == SignatureContractVersionV1 && len(manifest.Components) == 0:
		return signedManifestFieldsV1, true
	case manifest.ContractVersion == ManifestContractVersion && manifest.Signature.ContractVersion == SignatureContractVersion && manifest.Authorization.Version == SignatureContractVersion && len(manifest.Components) > 0:
		return signedManifestFieldsV2, true
	default:
		return nil, false
	}
}

func signaturePayload(manifest Manifest) ([]byte, error) {
	manifest.Signature.Value = ""
	return json.Marshal(manifest)
}

func (b *Builder) verifyManifestAuthorization(manifest Manifest) error {
	key, trusted := b.trustedKeys[manifest.Signature.KeyID]
	if !trusted {
		return fmt.Errorf("%w: %q", ErrUntrustedSigner, manifest.Signature.KeyID)
	}
	if err := VerifyManifestSignature(manifest, key); err != nil {
		return err
	}
	if manifest.CreatedAt.IsZero() || !manifest.Authorization.ExpiresAt.After(manifest.CreatedAt) || manifest.Authorization.Subject.WorkspaceID != manifest.WorkspaceID || manifest.Authorization.Subject.UserID <= 0 || !validSubjectID(manifest.Authorization.Subject.ID) {
		return ErrInvalidSignature
	}
	if _, valid := signedFieldsForManifest(manifest); !valid || !sameOriginPolicy(manifest.Authorization.AllowedOrigins, manifest.Security.AllowedOrigins) {
		return ErrInvalidSignature
	}
	if !reflect.DeepEqual(manifest.Authorization.ApprovedDomains, approvedDomains(manifest.Authorization.AllowedOrigins)) || (manifest.Authorization.Lifecycle != BundleLifecyclePreview && manifest.Authorization.Lifecycle != BundleLifecyclePublished) {
		return ErrInvalidSignature
	}
	return nil
}

// CheckManifestExpiry enforces serving authorization without hiding audit history.
func CheckManifestExpiry(manifest Manifest, now time.Time) error {
	if !manifest.Authorization.ExpiresAt.After(now.UTC()) {
		return ErrArtifactExpired
	}
	return nil
}

func sameOriginPolicy(left, right OriginPolicy) bool {
	return reflect.DeepEqual(left, right)
}

func approvedDomains(policy OriginPolicy) []string {
	seen := make(map[string]struct{})
	for _, origin := range policy.Surfaces {
		host := originHost(origin)
		if host != "" {
			seen[strings.ToLower(host)] = struct{}{}
		}
	}
	domains := make([]string, 0, len(seen))
	for domain := range seen {
		domains = append(domains, domain)
	}
	sort.Strings(domains)
	return domains
}

func originHost(origin string) string {
	// Origins are validated before this helper is called.
	separator := strings.Index(origin, "://")
	if separator < 0 {
		return ""
	}
	hostPort := origin[separator+3:]
	if strings.HasPrefix(hostPort, "[") {
		if end := strings.Index(hostPort, "]"); end >= 0 {
			return hostPort[1:end]
		}
	}
	if colon := strings.LastIndex(hostPort, ":"); colon >= 0 {
		return hostPort[:colon]
	}
	return hostPort
}
