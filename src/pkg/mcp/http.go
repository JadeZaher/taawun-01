package mcp

import (
	"net/http"
	"strings"
)

// ProtectExactHTTP rejects browser origins and Host headers outside configured allowlists.
func ProtectExactHTTP(allowedOrigins, allowedHosts []string, next http.Handler) http.Handler {
	origins := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		origins[origin] = struct{}{}
	}
	hosts := make(map[string]struct{}, len(allowedHosts))
	for _, host := range allowedHosts {
		hosts[strings.ToLower(host)] = struct{}{}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Origin")
		if _, allowed := hosts[strings.ToLower(r.Host)]; !allowed {
			http.Error(w, "MCP host is not allowed", http.StatusForbidden)
			return
		}
		originHeaders := r.Header.Values("Origin")
		if len(originHeaders) > 1 {
			http.Error(w, "MCP origin is not allowed", http.StatusForbidden)
			return
		}
		if len(originHeaders) == 1 {
			origin := originHeaders[0]
			if strings.Contains(origin, ",") {
				http.Error(w, "MCP origin is not allowed", http.StatusForbidden)
				return
			}
			if _, allowed := origins[origin]; !allowed {
				http.Error(w, "MCP origin is not allowed", http.StatusForbidden)
				return
			}
		} else if strings.EqualFold(r.Header.Get("Sec-Fetch-Site"), "cross-site") {
			http.Error(w, "cross-site MCP requests require an allowed Origin", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
