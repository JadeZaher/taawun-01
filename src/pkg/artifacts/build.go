package artifacts

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"taawun/pkg/ethics"
)

var (
	ErrInvalidOutputRoot = errors.New("invalid artifact output root")
	ErrArtifactConflict  = errors.New("artifact content address conflicts with existing data")
)

// Builder atomically publishes immutable bundles beneath one resolved output root.
type Builder struct {
	root            string
	now             func() time.Time
	keyID           string
	privateKey      ed25519.PrivateKey
	trustedKeys     map[string]ed25519.PublicKey
	ephemeralSigner bool
}

// BuildResult identifies a published content-addressed bundle.
type BuildResult struct {
	ArtifactID  string   `json:"artifactId"`
	ContentHash string   `json:"contentHash"`
	Directory   string   `json:"directory"`
	Manifest    Manifest `json:"manifest"`
}

// NewBuilder prepares and resolves a dedicated artifact output root.
func NewBuilder(outputRoot string) (*Builder, error) {
	signing, err := ephemeralSigningConfig()
	if err != nil {
		return nil, fmt.Errorf("create ephemeral artifact signer: %w", err)
	}
	return newSignedBuilder(outputRoot, signing, true)
}

// NewSignedBuilder prepares a production builder with an injected Ed25519 signer.
func NewSignedBuilder(outputRoot string, signing SigningConfig) (*Builder, error) {
	return newSignedBuilder(outputRoot, signing, false)
}

func newSignedBuilder(outputRoot string, signing SigningConfig, ephemeral bool) (*Builder, error) {
	validatedSigning, err := validateSigningConfig(signing)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(outputRoot) == "" {
		return nil, fmt.Errorf("%w: path is required", ErrInvalidOutputRoot)
	}
	absRoot, err := filepath.Abs(outputRoot)
	if err != nil {
		return nil, fmt.Errorf("%w: resolve path: %v", ErrInvalidOutputRoot, err)
	}
	if err := os.MkdirAll(absRoot, 0o755); err != nil {
		return nil, fmt.Errorf("%w: create path: %v", ErrInvalidOutputRoot, err)
	}
	info, err := os.Lstat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("%w: inspect path: %v", ErrInvalidOutputRoot, err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%w: path must be a real directory", ErrInvalidOutputRoot)
	}
	resolvedRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return nil, fmt.Errorf("%w: resolve links: %v", ErrInvalidOutputRoot, err)
	}
	return &Builder{
		root:            filepath.Clean(resolvedRoot),
		now:             time.Now,
		keyID:           validatedSigning.KeyID,
		privateKey:      validatedSigning.PrivateKey,
		trustedKeys:     validatedSigning.TrustedKeys,
		ephemeralSigner: ephemeral,
	}, nil
}

// ProductionReady reports whether the builder uses an explicitly injected stable signer.
func (b *Builder) ProductionReady() bool {
	return b != nil && !b.ephemeralSigner && len(b.privateKey) == ed25519.PrivateKeySize
}

// Build validates, renders, hashes, and atomically publishes one curated bundle.
func (b *Builder) Build(ctx context.Context, request BuildRequest) (BuildResult, error) {
	if b == nil || b.root == "" || b.now == nil || len(b.privateKey) != ed25519.PrivateKeySize {
		return BuildResult{}, fmt.Errorf("%w: builder is not initialized", ErrInvalidOutputRoot)
	}
	if ctx == nil {
		return BuildResult{}, errors.New("artifact build context is required")
	}
	if err := contextError(ctx); err != nil {
		return BuildResult{}, err
	}
	resolved, err := ResolveBuildRequest(request)
	if err != nil {
		return BuildResult{}, err
	}
	request = resolved
	if err := ValidateRequest(request); err != nil {
		return BuildResult{}, err
	}
	createdAt := b.now().UTC().Truncate(time.Millisecond)
	if !request.ExpiresAt.After(createdAt) {
		return BuildResult{}, fmt.Errorf("%w: expiresAt must be after creation", ErrInvalidBuildRequest)
	}

	normalized := normalizedRequest(request)
	manifest, files, err := assembleBundle(normalized, b.keyID)
	if err != nil {
		return BuildResult{}, err
	}
	manifest.Files = digestFiles(files)
	contentHash, err := semanticManifestHash(manifest)
	if err != nil {
		return BuildResult{}, fmt.Errorf("hash artifact contract: %w", err)
	}
	manifest.ContentHash = contentHash

	destination, err := safeJoin(b.root, contentHash)
	if err != nil {
		return BuildResult{}, err
	}
	if _, err := os.Stat(destination); err == nil {
		return b.loadExisting(destination, contentHash)
	} else if !errors.Is(err, os.ErrNotExist) {
		return BuildResult{}, fmt.Errorf("inspect artifact destination: %w", err)
	}
	if err := contextError(ctx); err != nil {
		return BuildResult{}, err
	}

	artifactID, err := randomIdentifier("art_", 16)
	if err != nil {
		return BuildResult{}, fmt.Errorf("generate artifact id: %w", err)
	}
	manifest.ArtifactID = artifactID
	manifest.CreatedAt = createdAt
	if err := signManifest(&manifest, b.keyID, b.privateKey); err != nil {
		return BuildResult{}, fmt.Errorf("sign artifact manifest: %w", err)
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return BuildResult{}, fmt.Errorf("encode manifest: %w", err)
	}
	files["manifest.json"] = manifestBytes

	temporary, err := b.createTemporaryDirectory()
	if err != nil {
		return BuildResult{}, err
	}
	published := false
	defer func() {
		if !published {
			_ = os.RemoveAll(temporary)
		}
	}()

	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if err := contextError(ctx); err != nil {
			return BuildResult{}, err
		}
		if err := writeBundleFile(temporary, path, files[path]); err != nil {
			return BuildResult{}, err
		}
	}
	if err := os.Rename(temporary, destination); err != nil {
		if _, statErr := os.Stat(destination); statErr == nil {
			return b.loadExisting(destination, contentHash)
		}
		return BuildResult{}, fmt.Errorf("publish artifact atomically: %w", err)
	}
	published = true
	return BuildResult{
		ArtifactID:  manifest.ArtifactID,
		ContentHash: manifest.ContentHash,
		Directory:   destination,
		Manifest:    manifest,
	}, nil
}

func (b *Builder) createTemporaryDirectory() (string, error) {
	for attempt := 0; attempt < 4; attempt++ {
		token, err := randomIdentifier(".taawun-build-", 12)
		if err != nil {
			return "", fmt.Errorf("generate temporary directory: %w", err)
		}
		path, err := safeJoin(b.root, token)
		if err != nil {
			return "", err
		}
		if err := os.Mkdir(path, 0o700); err == nil {
			return path, nil
		} else if !errors.Is(err, os.ErrExist) {
			return "", fmt.Errorf("create temporary artifact directory: %w", err)
		}
	}
	return "", errors.New("could not allocate a temporary artifact directory")
}

func (b *Builder) loadExisting(directory, expectedHash string) (BuildResult, error) {
	manifestPath, err := safeJoin(directory, "manifest.json")
	if err != nil {
		return BuildResult{}, err
	}
	encoded, err := os.ReadFile(manifestPath)
	if err != nil {
		return BuildResult{}, fmt.Errorf("%w: read manifest: %v", ErrArtifactConflict, err)
	}
	var manifest Manifest
	if err := json.Unmarshal(encoded, &manifest); err != nil {
		return BuildResult{}, fmt.Errorf("%w: decode manifest: %v", ErrArtifactConflict, err)
	}
	actualSemanticHash, err := semanticManifestHash(manifest)
	if err != nil || manifest.ContentHash != expectedHash || actualSemanticHash != expectedHash {
		return BuildResult{}, fmt.Errorf("%w: manifest does not match %s", ErrArtifactConflict, expectedHash)
	}
	if err := b.verifyManifestAuthorization(manifest); err != nil {
		return BuildResult{}, err
	}
	for _, file := range manifest.Files {
		path, err := safeJoin(directory, file.Path)
		if err != nil {
			return BuildResult{}, fmt.Errorf("%w: %v", ErrArtifactConflict, err)
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return BuildResult{}, fmt.Errorf("%w: read %s: %v", ErrArtifactConflict, file.Path, err)
		}
		digest := sha256.Sum256(contents)
		if len(contents) != file.Bytes || hex.EncodeToString(digest[:]) != file.SHA256 {
			return BuildResult{}, fmt.Errorf("%w: file digest mismatch for %s", ErrArtifactConflict, file.Path)
		}
	}
	if err := validateStoredComponentDocuments(directory, manifest); err != nil {
		return BuildResult{}, err
	}
	return BuildResult{
		ArtifactID:  manifest.ArtifactID,
		ContentHash: manifest.ContentHash,
		Directory:   directory,
		Manifest:    manifest,
	}, nil
}

func semanticManifestHash(manifest Manifest) (string, error) {
	manifest.ArtifactID = ""
	manifest.ContentHash = ""
	manifest.CreatedAt = time.Time{}
	manifest.Signature = BundleSignature{}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func digestFiles(files map[string][]byte) []FileDigest {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	digests := make([]FileDigest, 0, len(paths))
	for _, path := range paths {
		digest := sha256.Sum256(files[path])
		digests = append(digests, FileDigest{
			Path:   path,
			SHA256: hex.EncodeToString(digest[:]),
			Bytes:  len(files[path]),
		})
	}
	return digests
}

func writeBundleFile(root, relativePath string, contents []byte) error {
	path, err := safeJoin(root, relativePath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create artifact directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create artifact file %s: %w", relativePath, err)
	}
	closeWithError := func(prior error) error {
		if closeErr := file.Close(); prior == nil && closeErr != nil {
			return closeErr
		}
		return prior
	}
	if _, err := file.Write(contents); err != nil {
		return fmt.Errorf("write artifact file %s: %w", relativePath, closeWithError(err))
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync artifact file %s: %w", relativePath, closeWithError(err))
	}
	if err := closeWithError(nil); err != nil {
		return fmt.Errorf("close artifact file %s: %w", relativePath, err)
	}
	return nil
}

func safeJoin(root, relativePath string) (string, error) {
	if root == "" || relativePath == "" || filepath.IsAbs(relativePath) {
		return "", fmt.Errorf("%w: unsafe artifact path", ErrInvalidOutputRoot)
	}
	cleanRelative := filepath.Clean(filepath.FromSlash(relativePath))
	if cleanRelative == "." || cleanRelative == ".." || strings.HasPrefix(cleanRelative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: unsafe artifact path %q", ErrInvalidOutputRoot, relativePath)
	}
	joined := filepath.Join(root, cleanRelative)
	relative, err := filepath.Rel(root, joined)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: artifact path escapes output root", ErrInvalidOutputRoot)
	}
	return joined, nil
}

func randomIdentifier(prefix string, byteCount int) (string, error) {
	bytes := make([]byte, byteCount)
	if _, err := io.ReadFull(rand.Reader, bytes); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(bytes), nil
}

func marshalDocument(value any) ([]byte, error) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

func contextError(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func complianceReferences(madhhab ethics.Madhhab) ([]ComplianceReference, error) {
	records, err := ethics.NewSeedComplianceCorpus().Retrieve(
		"community iftar donation campaign marketplace terms checkout settlement escrow financing loan interest riba",
		madhhab,
		4,
	)
	if err != nil {
		return nil, err
	}
	references := make([]ComplianceReference, 0, len(records))
	for _, record := range records {
		references = append(references, ComplianceReference{
			ID:       record.ID,
			Title:    record.Title,
			Citation: record.Citation,
			Status:   record.Status,
		})
	}
	return references, nil
}
