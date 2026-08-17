package oauth

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	publicOAuthWindow             = 15 * time.Minute
	maximumPublicOAuthSources     = 4096
	OAuthRegistrationAttemptLimit = 5
	OAuthLoginAttemptLimit        = 10
)

type oauthRateLimitEntry struct {
	started  time.Time
	attempts int
}

// oauthPublicRateLimiter bounds both each public request window and its in-memory accounting.
type oauthPublicRateLimiter struct {
	mu         sync.Mutex
	entries    map[string]oauthRateLimitEntry
	window     time.Duration
	maxEntries int
	now        func() time.Time
}

func newOAuthPublicRateLimiter(window time.Duration, maxEntries int) *oauthPublicRateLimiter {
	return &oauthPublicRateLimiter{entries: make(map[string]oauthRateLimitEntry), window: window, maxEntries: maxEntries, now: time.Now}
}

func (l *oauthPublicRateLimiter) Allow(operation, source string, maximumAttempts int) (bool, time.Duration) {
	now := l.now().UTC()
	key := operation + "\x00" + source
	l.mu.Lock()
	defer l.mu.Unlock()
	for candidate, entry := range l.entries {
		if !entry.started.Add(l.window).After(now) {
			delete(l.entries, candidate)
		}
	}
	entry, exists := l.entries[key]
	if !exists {
		if len(l.entries) >= l.maxEntries {
			return false, l.window
		}
		l.entries[key] = oauthRateLimitEntry{started: now, attempts: 1}
		return true, 0
	}
	if entry.attempts >= maximumAttempts {
		return false, entry.started.Add(l.window).Sub(now)
	}
	entry.attempts++
	l.entries[key] = entry
	return true, 0
}

func oauthRequestSource(r *http.Request) string {
	remote := strings.TrimSpace(r.RemoteAddr)
	if host, _, err := net.SplitHostPort(remote); err == nil && host != "" {
		return host
	}
	if remote != "" {
		return remote
	}
	return "unknown"
}
