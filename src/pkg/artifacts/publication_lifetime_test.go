package artifacts

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPublicationLifetimeUsesInjectedClockAndExactStoredManifestBytes(t *testing.T) {
	createdAt := time.Date(2026, 8, 21, 12, 0, 0, 123_000_000, time.UTC)
	expiresAt := createdAt.Add(24 * time.Hour)
	seed := sha256.Sum256([]byte("publication lifetime acceptance signer"))
	privateKey := ed25519.NewKeyFromSeed(seed[:])
	builder, err := NewSignedBuilderWithClock(filepath.Join(t.TempDir(), "artifacts"), SigningConfig{
		KeyID: "publication-lifetime-test-key", PrivateKey: privateKey,
	}, func() time.Time { return createdAt })
	if err != nil {
		t.Fatalf("NewSignedBuilderWithClock() error = %v", err)
	}
	request := validBuildRequest()
	request.ExpiresAt = expiresAt
	request.Lifecycle = BundleLifecyclePublished
	result, err := builder.Build(context.Background(), request)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if !result.Manifest.CreatedAt.Equal(createdAt) {
		t.Fatalf("createdAt = %s, want injected %s", result.Manifest.CreatedAt, createdAt)
	}

	for _, test := range []struct {
		name    string
		now     time.Time
		expired bool
	}{
		{name: "one millisecond before", now: expiresAt.Add(-time.Millisecond)},
		{name: "exact boundary", now: expiresAt, expired: true},
		{name: "one millisecond after", now: expiresAt.Add(time.Millisecond), expired: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := CheckManifestExpiry(result.Manifest, test.now)
			if errors.Is(err, ErrArtifactExpired) != test.expired {
				t.Fatalf("CheckManifestExpiry(%s) error = %v, expired want %t", test.now, err, test.expired)
			}
		})
	}

	storedBytes := append(append([]byte(nil), result.ManifestJSON...), '\n')
	if bytes.Equal(storedBytes, result.ManifestJSON) {
		t.Fatal("test fixture did not change the manifest byte serialization")
	}
	if err := os.WriteFile(filepath.Join(result.Directory, "manifest.json"), storedBytes, 0o644); err != nil {
		t.Fatalf("rewrite semantically equivalent stored manifest: %v", err)
	}
	opened, err := builder.Open(context.Background(), result.ContentHash)
	if err != nil {
		t.Fatalf("Open() reformatted signed manifest error = %v", err)
	}
	if !bytes.Equal(opened.ManifestJSON, storedBytes) {
		t.Fatalf("Open() manifest bytes were reserialized: got %q want exact stored %q", opened.ManifestJSON, storedBytes)
	}
}

func TestPublicationBuilderRejectsMissingServerClock(t *testing.T) {
	seed := sha256.Sum256([]byte("publication lifetime missing clock signer"))
	privateKey := ed25519.NewKeyFromSeed(seed[:])
	if _, err := NewSignedBuilderWithClock(filepath.Join(t.TempDir(), "artifacts"), SigningConfig{
		KeyID: "publication-lifetime-test-key", PrivateKey: privateKey,
	}, nil); !errors.Is(err, ErrInvalidSigningConfig) {
		t.Fatalf("NewSignedBuilderWithClock(nil) error = %v, want ErrInvalidSigningConfig", err)
	}
}
