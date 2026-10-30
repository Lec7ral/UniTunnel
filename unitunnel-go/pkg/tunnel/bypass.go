package tunnel

import (
	"net"
	"strings"
)

// IsCampusBypass determines if a host/address belongs to the UCI campus or local intranet,
// which must NEVER be forwarded through the upstream Squid proxy (otherwise Squid responds
// with 403 Forbidden or network errors when accessing internal campus servers).
func IsCampusBypass(hostOrTarget string) bool {
	h := strings.ToLower(strings.TrimSpace(hostOrTarget))
	if h == "" {
		return false
	}

	// Strip port if present
	if strings.Contains(h, ":") {
		if parsedHost, _, err := net.SplitHostPort(h); err == nil {
			h = parsedHost
		}
	}

	// Normalize wildcards and leading dots
	h = strings.TrimPrefix(h, "*")
	h = strings.TrimPrefix(h, ".")

	// UCI campus domains (*uci.cu, *.uci.cu, uci.cu, cuota.uci.cu, moodle.uci.cu, etc.)
	if h == "uci.cu" || strings.HasSuffix(h, ".uci.cu") || strings.Contains(h, "uci.cu") {
		return true
	}

	// Localhost and loopback
	if h == "localhost" || strings.HasPrefix(h, "127.") {
		return true
	}

	// RFC1918 private IPs (often campus subnets like 10.x.x.x)
	if ip := net.ParseIP(h); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() {
			return true
		}
	}

	return false
}
