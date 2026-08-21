package domains

import (
	"database/sql"
	"errors"
	"fmt"
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
	requestNow := h.service.now().UTC()
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", http.MethodGet+", "+http.MethodHead)
		writePublicError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	_, host, err := NormalizeOrigin("https://" + hostHeader)
	if err != nil {
		writePublicError(w, http.StatusNotFound, "Not found")
		return
	}
	publication, claim, err := h.service.activePublicationForHostAt(r.Context(), host, requestNow)
	if errors.Is(err, sql.ErrNoRows) {
		writePublicError(w, http.StatusNotFound, "Not found")
		return
	}
	if err != nil {
		writePublicError(w, http.StatusServiceUnavailable, "Published card unavailable")
		return
	}
	built, err := h.service.artifacts.Open(r.Context(), publication.ContentHash)
	if err != nil || validateArtifactBindingForPublication(built, claim, publication) != nil ||
		artifacts.CheckManifestExpiry(built.Manifest, requestNow) != nil {
		writePublicError(w, http.StatusServiceUnavailable, "Published card unavailable")
		return
	}
	filePath := strings.TrimPrefix(r.URL.Path, "/")
	if filePath == "" {
		filePath = "index.html"
	} else if filePath == "embed" {
		filePath = "embed.html"
	} else if cleaned := path.Clean(filePath); cleaned != filePath || cleaned == "." || strings.HasPrefix(cleaned, "../") {
		writePublicError(w, http.StatusNotFound, "Not found")
		return
	}
	file, err := h.service.artifacts.ReadVerifiedFile(r.Context(), built, filePath)
	if err != nil {
		if errors.Is(err, artifacts.ErrArtifactFileNotFound) || errors.Is(err, artifacts.ErrArtifactNotFound) {
			writePublicError(w, http.StatusNotFound, "Not found")
		} else {
			writePublicError(w, http.StatusServiceUnavailable, "Published card unavailable")
		}
		return
	}
	switch path.Ext(file.Path) {
	case ".html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case ".css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case ".js":
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	default:
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	}
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

func writePublicError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(message + "\n"))
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
