package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const maxClientMetadataBytes = 64 << 10

type cachedClient struct {
	client    *Client
	expiresAt time.Time
}

type clientMetadataResolver struct {
	repository *Repository
	now        func() time.Time
	mu         sync.Mutex
	cache      map[string]cachedClient
	limit      chan struct{}
}

type clientMetadataDocument struct {
	ClientID                string   `json:"client_id"`
	ClientName              string   `json:"client_name"`
	ClientURI               string   `json:"client_uri"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
}

func newClientMetadataResolver(repository *Repository, now func() time.Time) *clientMetadataResolver {
	return &clientMetadataResolver{repository: repository, now: now, cache: make(map[string]cachedClient), limit: make(chan struct{}, 8)}
}

func isMetadataClientID(id string) bool {
	u, err := url.Parse(id)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && u.Path != "" && u.Path != "/"
}

func (r *clientMetadataResolver) resolve(ctx context.Context, id string) (*Client, error) {
	if !isMetadataClientID(id) {
		return nil, errors.New("invalid Client ID Metadata Document URL")
	}
	now := r.now().UTC()
	r.mu.Lock()
	entry, ok := r.cache[id]
	r.mu.Unlock()
	if ok && entry.expiresAt.After(now) {
		copy := *entry.client
		return &copy, nil
	}

	select {
	case r.limit <- struct{}{}:
		defer func() { <-r.limit }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	client, ttl, err := fetchClientMetadata(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := r.repository.upsertMetadataClient(*client, now); err != nil {
		return nil, err
	}
	if ttl > 0 {
		r.mu.Lock()
		r.cache[id] = cachedClient{client: client, expiresAt: now.Add(ttl)}
		r.mu.Unlock()
	}
	return client, nil
}

func fetchClientMetadata(parent context.Context, id string) (*Client, time.Duration, error) {
	u, _ := url.Parse(id)
	ctx, cancel := context.WithTimeout(parent, 4*time.Second)
	defer cancel()
	transport := &http.Transport{
		Proxy:                 nil,
		DisableCompression:    true,
		ForceAttemptHTTP2:     true,
		ResponseHeaderTimeout: 3 * time.Second,
		DialContext:           publicDialer(u.Hostname()),
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport: transport,
		Timeout:   4 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("Client ID Metadata Document redirects are not allowed")
		},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, id, nil)
	if err != nil {
		return nil, 0, err
	}
	request.Header.Set("Accept", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to fetch client metadata: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("client metadata returned HTTP %d", response.StatusCode)
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || (mediaType != "application/json" && mediaType != "application/client-metadata+json") {
		return nil, 0, errors.New("client metadata must be JSON")
	}
	limited := io.LimitReader(response.Body, maxClientMetadataBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, 0, err
	}
	if len(data) > maxClientMetadataBytes {
		return nil, 0, errors.New("client metadata document is too large")
	}
	var document clientMetadataDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, 0, errors.New("client metadata is not valid JSON")
	}
	if document.ClientID != id {
		return nil, 0, errors.New("client metadata client_id does not match its URL")
	}
	resolved := Client{ID: id, Name: strings.TrimSpace(document.ClientName), ClientURI: strings.TrimSpace(document.ClientURI), RedirectURIs: document.RedirectURIs, GrantTypes: document.GrantTypes, ResponseTypes: document.ResponseTypes, TokenEndpointAuthMethod: document.TokenEndpointAuthMethod}
	validated, err := validateClientDefinition(resolved)
	if err != nil {
		return nil, 0, err
	}
	return &validated, metadataCacheTTL(response.Header), nil
}

func publicDialer(expectedHost string) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil || !strings.EqualFold(strings.TrimSuffix(host, "."), strings.TrimSuffix(expectedHost, ".")) {
			return nil, errors.New("unexpected client metadata host")
		}
		addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil || len(addresses) == 0 {
			return nil, errors.New("client metadata host did not resolve")
		}
		var lastErr error
		dialer := net.Dialer{Timeout: 3 * time.Second}
		for _, address := range addresses {
			if !publicIP(address.IP) {
				return nil, errors.New("client metadata host resolves to a non-public address")
			}
			connection, err := dialer.DialContext(ctx, network, net.JoinHostPort(address.IP.String(), port))
			if err == nil {
				return connection, nil
			}
			lastErr = err
		}
		return nil, lastErr
	}
}

func publicIP(ip net.IP) bool {
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	if v4 := ip.To4(); v4 != nil && v4[0] == 100 && v4[1]&0xc0 == 64 {
		return false
	}
	return true
}

func metadataCacheTTL(header http.Header) time.Duration {
	if strings.Contains(strings.ToLower(header.Get("Cache-Control")), "no-store") {
		return 0
	}
	for _, directive := range strings.Split(header.Get("Cache-Control"), ",") {
		parts := strings.SplitN(strings.TrimSpace(directive), "=", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "max-age") {
			seconds, err := strconv.Atoi(strings.Trim(parts[1], `"`))
			if err == nil && seconds >= 0 {
				if seconds > 3600 {
					seconds = 3600
				}
				return time.Duration(seconds) * time.Second
			}
		}
	}
	return 5 * time.Minute
}
