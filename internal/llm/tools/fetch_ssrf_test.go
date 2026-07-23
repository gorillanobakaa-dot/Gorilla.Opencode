package tools

import "testing"

// GORILLA OVERRIDE: guards the SSRF blocklist on the fetch tool. Upstream
// fetched any host the model named, including loopback / link-local / private
// LAN. These cases must stay blocked; public hosts must stay reachable.
func TestBlockedFetchTarget(t *testing.T) {
	t.Parallel()
	blocked := map[string]string{
		"http://169.254.169.254/latest/meta-data/": "cloud metadata (link-local)",
		"http://localhost:8080/admin":               "loopback name",
		"http://127.0.0.1/":                          "loopback IP",
		"http://10.0.0.5/internal":                   "private 10/8",
		"http://192.168.1.1/router":                  "private 192.168/16",
		"http://172.16.0.9/":                         "private 172.16/12",
		"http://[::1]/":                              "IPv6 loopback",
		"http://0.0.0.0/":                            "unspecified",
	}
	for u, why := range blocked {
		if blockedFetchTarget(u) == "" {
			t.Errorf("SSRF hole: should block %s (%s) but allowed", u, why)
		}
	}

	allowed := []string{
		"https://example.com/page",
		"https://api.github.com/repos/x/y",
		"http://93.184.216.34/", // public IP
	}
	for _, u := range allowed {
		if r := blockedFetchTarget(u); r != "" {
			t.Errorf("false positive: should allow %s but blocked (%s)", u, r)
		}
	}
}
