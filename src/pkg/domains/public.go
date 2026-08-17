package domains

import (
	"database/sql"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"taawun/pkg/artifacts"
)

type PublicHandler struct {
	service      *Service
	controlHosts map[string]struct{}
	fallback     http.Handler
}

// NewPublicHandler separates configured control-plane hosts from published domains.
func NewPublicHandler(service *Service, controlOrigins []string, fallback http.Handler) (*PublicHandler, error) {
	if service == nil || fallback == nil || len(controlOrigins) == 0 {
		return nil, errors.New("domain service, control origins, and fallback are required")
	}
	hosts := make(map[string]struct{}, len(controlOrigins))
	for _, origin := range controlOrigins {
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return nil, fmt.Errorf("invalid control origin %q", origin)
		}
		hosts[strings.ToLower(parsed.Host)] = struct{}{}
	}
	return &PublicHandler{service: service, controlHosts: hosts, fallback: fallback}, nil
}

func (h *PublicHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hostHeader := strings.ToLower(strings.TrimSpace(r.Host))
	if _, control := h.controlHosts[hostHeader]; control {
		h.fallback.ServeHTTP(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", http.MethodGet+", "+http.MethodHead)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_, host, err := NormalizeOrigin("https://" + hostHeader)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	publication, claim, err := h.service.activePublicationForHost(r.Context(), host)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Published card unavailable", http.StatusServiceUnavailable)
		return
	}
	built, err := h.service.artifacts.Open(r.Context(), publication.ContentHash)
	if err != nil || validateArtifactForClaim(built, claim, h.service.now().UTC()) != nil ||
		built.ArtifactID != publication.ArtifactID || built.ContentHash != publication.ContentHash {
		http.Error(w, "Published card unavailable", http.StatusServiceUnavailable)
		return
	}
	filePath := strings.TrimPrefix(r.URL.Path, "/")
	if filePath == "" {
		filePath = "index.html"
	} else if filePath == "embed" {
		filePath = "embed.html"
	} else if cleaned := path.Clean(filePath); cleaned != filePath || cleaned == "." || strings.HasPrefix(cleaned, "../") {
		http.NotFound(w, r)
		return
	}
	file, err := h.service.artifacts.ReadFile(r.Context(), publication.ContentHash, filePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	contentType := mime.TypeByExtension(path.Ext(file.Path))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Contents)))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("ETag", `"`+file.SHA256+`"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if strings.HasSuffix(file.Path, ".html") {
		w.Header().Set("Content-Security-Policy", manifestCSP(built.Manifest, file.Path))
	}
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, _ = w.Write(file.Contents)
	}
}

func manifestCSP(manifest artifacts.Manifest, filePath string) string {
	mode := artifacts.RenderModeStandalone
	if filePath == "embed.html" || strings.HasPrefix(filePath, "cards/") {
		mode = artifacts.RenderModeEmbed
	}
	for _, entry := range manifest.Security.CSP {
		if entry.RenderMode == mode {
			return formatCSP(entry.Policy)
		}
	}
	return "default-src 'none'; base-uri 'none'; frame-ancestors 'none'"
}

func formatCSP(policy artifacts.CSPPolicy) string {
	directives := []struct {
		name   string
		values []string
	}{
		{name: "default-src", values: policy.DefaultSrc},
		{name: "script-src", values: policy.ScriptSrc},
		{name: "style-src", values: policy.StyleSrc},
		{name: "connect-src", values: policy.ConnectSrc},
		{name: "img-src", values: policy.ImageSrc},
		{name: "font-src", values: policy.FontSrc},
		{name: "frame-ancestors", values: policy.FrameAncestors},
		{name: "object-src", values: policy.ObjectSrc},
		{name: "base-uri", values: policy.BaseURI},
		{name: "form-action", values: policy.FormAction},
	}
	parts := make([]string, 0, len(directives))
	for _, directive := range directives {
		if len(directive.values) > 0 {
			parts = append(parts, directive.name+" "+strings.Join(directive.values, " "))
		}
	}
	return strings.Join(parts, "; ")
}
