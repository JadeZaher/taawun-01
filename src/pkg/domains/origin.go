package domains

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// NormalizeOrigin returns the one HTTPS origin represented by a DNS hostname.
func NormalizeOrigin(value string) (origin, host string, err error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "", ErrInvalidOrigin
	}
	if !strings.Contains(value, "://") {
		value = "https://" + value
	}
	parsed, parseErr := url.Parse(value)
	if parseErr != nil || !strings.EqualFold(parsed.Scheme, "https") || parsed.User != nil {
		return "", "", ErrInvalidOrigin
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return "", "", ErrInvalidOrigin
	}
	if port := parsed.Port(); port != "" {
		parsedPort, portErr := strconv.Atoi(port)
		if portErr != nil || parsedPort != 443 {
			return "", "", ErrInvalidOrigin
		}
	}
	host = strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if !validDNSHost(host) {
		return "", "", ErrInvalidOrigin
	}
	return "https://" + host, host, nil
}

func validDNSHost(host string) bool {
	if host == "" || len(host) > 253 || net.ParseIP(host) != nil || strings.Contains(host, "*") {
		return false
	}
	labels := strings.Split(host, ".")
	if len(labels) < 2 {
		return false
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
				return false
			}
		}
	}
	return true
}

func challengeRecord(host string) string {
	return fmt.Sprintf("_taawun.%s", host)
}
