package artifacts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrInvalidContentHash   = errors.New("invalid artifact content hash")
	ErrArtifactNotFound     = errors.New("artifact not found")
	ErrArtifactFileNotFound = errors.New("artifact file not found")
)

// ArtifactFile is a verified immutable file from a published bundle.
type ArtifactFile struct {
	Path     string
	Contents []byte
	SHA256   string
}

// Open verifies and returns metadata for a published content-addressed bundle.
func (b *Builder) Open(ctx context.Context, contentHash string) (BuildResult, error) {
	if b == nil || b.root == "" {
		return BuildResult{}, fmt.Errorf("%w: builder is not initialized", ErrInvalidOutputRoot)
	}
	if ctx == nil {
		return BuildResult{}, errors.New("artifact read context is required")
	}
	if err := contextError(ctx); err != nil {
		return BuildResult{}, err
	}
	if !validContentHash(contentHash) {
		return BuildResult{}, fmt.Errorf("%w: %q", ErrInvalidContentHash, contentHash)
	}
	directory, err := safeJoin(b.root, contentHash)
	if err != nil {
		return BuildResult{}, err
	}
	info, err := os.Lstat(directory)
	if errors.Is(err, os.ErrNotExist) {
		return BuildResult{}, ErrArtifactNotFound
	}
	if err != nil {
		return BuildResult{}, fmt.Errorf("inspect artifact: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return BuildResult{}, fmt.Errorf("%w: artifact directory is not a real directory", ErrArtifactConflict)
	}
	return b.loadExisting(directory, contentHash)
}

// ReadFile verifies the whole bundle and returns only a manifest-listed file.
func (b *Builder) ReadFile(ctx context.Context, contentHash, relativePath string) (ArtifactFile, error) {
	result, err := b.Open(ctx, contentHash)
	if err != nil {
		return ArtifactFile{}, err
	}
	return b.ReadVerifiedFile(ctx, result, relativePath)
}

// ReadVerifiedFile reads one manifest-listed file from an already verified bundle.
func (b *Builder) ReadVerifiedFile(ctx context.Context, result BuildResult, relativePath string) (ArtifactFile, error) {
	if b == nil || ctx == nil || contextError(ctx) != nil || !safePublishedFilePath(relativePath) ||
		!validContentHash(result.ContentHash) || result.ContentHash != result.Manifest.ContentHash ||
		result.ArtifactID == "" || result.ArtifactID != result.Manifest.ArtifactID || result.Directory == "" {
		return ArtifactFile{}, fmt.Errorf("%w: invalid verified bundle", ErrArtifactFileNotFound)
	}
	directory, err := safeJoin(b.root, result.ContentHash)
	if err != nil || filepath.Clean(directory) != filepath.Clean(result.Directory) {
		return ArtifactFile{}, fmt.Errorf("%w: invalid verified bundle", ErrArtifactFileNotFound)
	}
	var expected *FileDigest
	for i := range result.Manifest.Files {
		if result.Manifest.Files[i].Path == relativePath {
			expected = &result.Manifest.Files[i]
			break
		}
	}
	if expected == nil {
		return ArtifactFile{}, ErrArtifactFileNotFound
	}
	path, err := safeJoin(result.Directory, relativePath)
	if err != nil {
		return ArtifactFile{}, ErrArtifactFileNotFound
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return ArtifactFile{}, ErrArtifactFileNotFound
	}
	if err != nil {
		return ArtifactFile{}, fmt.Errorf("read artifact file metadata: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return ArtifactFile{}, fmt.Errorf("%w: artifact file is not a regular file", ErrArtifactConflict)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return ArtifactFile{}, fmt.Errorf("read artifact file: %w", err)
	}
	digest := sha256.Sum256(contents)
	if len(contents) != expected.Bytes || hex.EncodeToString(digest[:]) != expected.SHA256 {
		return ArtifactFile{}, fmt.Errorf("%w: file digest mismatch", ErrArtifactConflict)
	}
	return ArtifactFile{Path: relativePath, Contents: contents, SHA256: expected.SHA256}, nil
}

func validContentHash(value string) bool {
	if len(value) != 64 || value != strings.ToLower(value) {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32
}

func safePublishedFilePath(value string) bool {
	if value == "" || strings.Contains(value, "\\") || strings.HasPrefix(value, "/") {
		return false
	}
	cleaned := filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))
	return cleaned == value && cleaned != "." && cleaned != ".." && !strings.HasPrefix(cleaned, "../")
}
