package tools

import (
	"net/url"
	"strings"

	"github.com/opencode-ai/opencode/internal/config"
)

// maxQueryData is the longest query string (or search query) that still counts
// as "looking something up". Longer is the shape of data being carried out in
// a URL, so it asks.
const maxQueryData = 300

// webNeedsAsk reports whether a web_fetch of raw must go to the permission
// service. permissions.web=allow (default) waves a public http(s) GET through
// in every mode except plan; everything else is asked as before.
func webNeedsAsk(raw string) bool {
	if config.WebPermission() != config.WebAllow || config.PermissionMode() == config.PermissionPlan {
		return true
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return true
	}
	if blockedFetchTarget(raw) != "" {
		return true
	}
	if len(u.RawQuery) > maxQueryData || u.User != nil {
		return true
	}
	return false
}

// webSearchNeedsAsk is the same decision for web_search.
func webSearchNeedsAsk(query string) bool {
	if config.WebPermission() != config.WebAllow || config.PermissionMode() == config.PermissionPlan {
		return true
	}
	return len(strings.TrimSpace(query)) > maxQueryData
}
