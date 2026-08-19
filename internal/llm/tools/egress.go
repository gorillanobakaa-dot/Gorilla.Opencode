// GORILLA OVERRIDE: this file did not exist upstream.
//
// fetch.go has a genuinely good three-layer SSRF guard: a string check on the
// literal URL, a dialer Control hook that sees the RESOLVED address (so DNS
// rebinding fails), and a redirect hook that re-runs the check on every hop.
//
// Nothing else in the program used any of it. `client.NewSSEMCPClient` took an
// arbitrary URL straight out of config and dialled it, so
// `http://169.254.169.254/latest/meta-data/` was a valid MCP server address on
// 2026-08-19.
package tools

import (
	"net"
	"net/url"
	"strings"
)

// BlockedMCPTarget checks an MCP server URL and returns why it is refused, or
// "" if it is allowed.
//
// It is DELIBERATELY weaker than blockedFetchTarget, and the difference is the
// point. A fetch URL is chosen by the model, possibly under the influence of a
// page it just read. An MCP server URL is chosen by the human, in a config
// file, before the program started — and `http://localhost:3000` is the single
// most common MCP setup there is. Refusing loopback and private addresses here
// would break the normal case in order to defend against a threat that
// requires the attacker to already be editing your config.
//
// What is refused is the class that has no legitimate MCP use and a very
// specific illegitimate one: link-local, which is where every cloud provider
// parks its unauthenticated credential endpoint, plus the unspecified and
// multicast ranges, plus any non-http scheme.
//
// Stated plainly so it can be disagreed with: this closes credential theft via
// a shared or generated config, and does not attempt to sandbox MCP.
func BlockedMCPTarget(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "could not parse the MCP server URL"
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "an MCP SSE server URL must be http or https"
	}
	host := u.Hostname()
	if host == "" {
		return "no host in the MCP server URL"
	}
	// Names are resolved as well as literals: metadata.google.internal is a
	// hostname, not an IP, and resolves to a link-local address.
	ips := []net.IP{}
	if ip := net.ParseIP(host); ip != nil {
		ips = append(ips, ip)
	} else {
		resolved, err := net.LookupIP(host)
		if err != nil {
			// Do not refuse on a lookup failure: the server may simply be
			// down, and the dial will report that far more usefully than a
			// security message would.
			return ""
		}
		ips = resolved
	}
	for _, ip := range ips {
		if reason := blockedMetadataIP(ip); reason != "" {
			return reason
		}
	}
	return ""
}

// blockedMetadataIP is blockedIP minus loopback and private, which MCP needs.
func blockedMetadataIP(ip net.IP) string {
	if v4 := ip.To4(); v4 != nil && !ip.Equal(v4) {
		ip = v4
	}
	switch {
	case ip.IsLinkLocalUnicast(), ip.IsLinkLocalMulticast():
		return "link-local address (this is where cloud metadata services such as 169.254.169.254 live) is not a valid MCP server"
	case ip.IsUnspecified():
		return "unspecified address (0.0.0.0/::) is not a valid MCP server"
	case ip.IsInterfaceLocalMulticast(), ip.IsMulticast():
		return "multicast address is not a valid MCP server"
	}
	return ""
}
