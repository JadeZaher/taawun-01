package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"
)

func TestConfiguredPort(t *testing.T) {
	t.Setenv("PORT", "")
	if port, err := configuredPort(); err != nil || port != "8080" {
		t.Fatalf("configuredPort() = (%q, %v), want 8080", port, err)
	}
	t.Setenv("PORT", " 4321 ")
	if port, err := configuredPort(); err != nil || port != "4321" {
		t.Fatalf("configuredPort() = (%q, %v), want 4321", port, err)
	}
	for _, invalid := range []string{"0", "65536", "not-a-port"} {
		t.Setenv("PORT", invalid)
		if _, err := configuredPort(); err == nil {
			t.Fatalf("expected PORT=%q to be rejected", invalid)
		}
	}
}

func TestConfiguredAppOriginsRejectsWildcardsAndInsecureRemoteOrigins(t *testing.T) {
	t.Setenv("TAWUN_ALLOWED_ORIGINS", "")
	origins, err := configuredAppOrigins("9000")
	if err != nil || len(origins) != 1 || origins[0] != "http://localhost:9000" {
		t.Fatalf("default origins = %#v, %v", origins, err)
	}

	for _, invalid := range []string{
		"*",
		"http://community.example",
		"https://community.example/path",
		"https://community.example/",
		"https://user:pass@community.example",
	} {
		t.Setenv("TAWUN_ALLOWED_ORIGINS", invalid)
		if _, err := configuredAppOrigins("9000"); err == nil {
			t.Fatalf("expected origin %q to be rejected", invalid)
		}
	}

	t.Setenv("TAWUN_ALLOWED_ORIGINS", "https://one.example, https://two.example:8443, http://127.0.0.1:5173")
	origins, err = configuredAppOrigins("9000")
	if err != nil || len(origins) != 3 {
		t.Fatalf("configured origins = %#v, %v", origins, err)
	}
}

func TestConfiguredJWTSecret(t *testing.T) {
	t.Setenv("TAWUN_JWT_SECRET", "too-short")
	if _, err := configuredJWTSecret(); err == nil {
		t.Fatal("expected short JWT secret to be rejected")
	}
	t.Setenv("TAWUN_JWT_SECRET", "0123456789abcdef0123456789abcdef")
	secret, err := configuredJWTSecret()
	if err != nil || len(secret) != 32 {
		t.Fatalf("configuredJWTSecret() length = %d, err = %v", len(secret), err)
	}
}

func TestConfiguredMCPHostsUsesExplicitHostsOrSafeDeploymentDefaults(t *testing.T) {
	t.Setenv("TAWUN_MCP_ALLOWED_HOSTS", "mcp.example, cards.example:8443")
	t.Setenv("RAILWAY_PUBLIC_DOMAIN", "")
	hosts, err := configuredMCPHosts([]string{"https://app.example"})
	if err != nil || len(hosts) != 2 || hosts[0] != "mcp.example" {
		t.Fatalf("explicit MCP hosts = %#v, %v", hosts, err)
	}

	t.Setenv("TAWUN_MCP_ALLOWED_HOSTS", "")
	t.Setenv("RAILWAY_PUBLIC_DOMAIN", "taawun-production.up.railway.app")
	hosts, err = configuredMCPHosts([]string{"https://app.example"})
	if err != nil || len(hosts) != 1 || hosts[0] != "taawun-production.up.railway.app" {
		t.Fatalf("Railway MCP hosts = %#v, %v", hosts, err)
	}

	t.Setenv("RAILWAY_PUBLIC_DOMAIN", "")
	hosts, err = configuredMCPHosts([]string{"https://app.example", "http://localhost:8080"})
	if err != nil || len(hosts) != 2 || hosts[0] != "app.example" || hosts[1] != "localhost:8080" {
		t.Fatalf("derived MCP hosts = %#v, %v", hosts, err)
	}

	for _, invalid := range []string{"*", "https://mcp.example", "mcp.example/path", "user@mcp.example", "mcp.example,mcp.example"} {
		t.Setenv("TAWUN_MCP_ALLOWED_HOSTS", invalid)
		if _, err := configuredMCPHosts([]string{"https://app.example"}); err == nil {
			t.Fatalf("expected MCP host %q to be rejected", invalid)
		}
	}
}

func TestConfiguredPublicBaseURLUsesHTTPSDeploymentOrigin(t *testing.T) {
	t.Setenv("TAWUN_PUBLIC_BASE_URL", "")
	t.Setenv("RAILWAY_PUBLIC_DOMAIN", "taawun-production.up.railway.app")
	value, err := configuredPublicBaseURL("8080", []string{"http://localhost:8080"})
	if err != nil || value != "https://taawun-production.up.railway.app" {
		t.Fatalf("Railway public URL = %q, %v", value, err)
	}

	for _, invalid := range []string{"http://taawun.example", "https://taawun.example/path", "https://user@taawun.example"} {
		t.Setenv("TAWUN_PUBLIC_BASE_URL", invalid)
		if _, err := configuredPublicBaseURL("8080", []string{"http://localhost:8080"}); err == nil {
			t.Fatalf("expected public URL %q to be rejected", invalid)
		}
	}
}

func TestConfiguredArtifactBuilderRequiresStableEd25519Signer(t *testing.T) {
	t.Setenv("TAWUN_ARTIFACT_ROOT", t.TempDir())
	t.Setenv("TAWUN_ARTIFACT_SIGNING_KEY_ID", "")
	t.Setenv("TAWUN_ARTIFACT_SIGNING_PRIVATE_KEY", "")
	if _, err := configuredArtifactBuilder(); err == nil {
		t.Fatal("expected missing signing configuration to fail")
	}

	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	privateKey := ed25519.NewKeyFromSeed(seed)
	t.Setenv("TAWUN_ARTIFACT_SIGNING_KEY_ID", "railway-primary")
	t.Setenv("TAWUN_ARTIFACT_SIGNING_PRIVATE_KEY", base64.RawURLEncoding.EncodeToString(privateKey))
	if _, err := configuredArtifactBuilder(); err != nil {
		t.Fatalf("configure signed artifact builder: %v", err)
	}

	t.Setenv("TAWUN_ARTIFACT_SIGNING_PRIVATE_KEY", base64.RawURLEncoding.EncodeToString(seed))
	if _, err := configuredArtifactBuilder(); err == nil {
		t.Fatal("expected Ed25519 seed to be rejected in place of a private key")
	}
}
