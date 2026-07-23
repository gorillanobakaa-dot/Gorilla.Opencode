// GORILLA OVERRIDE: this file did not exist upstream. It stores the
// user-adjustable request pace-setter ("Dial 1" in the context menu).
//
// Why user-adjustable instead of a fixed number: free-tier request limits are
// undocumented, variable, and load/time-of-day dependent. NVIDIA NIM advertises
// only "up to 40 rpm" — in practice anywhere from single digits to ~30. No
// hard-coded value is right across accounts and times, so we expose the dial and
// let the user tune it until requests "cruise" just under the ceiling. See
// PHILOSOPHY.md: measure, show the user, give them the switch.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

const rateLimitFileName = "ratelimit.json"

// RPMPresets is the ladder the /context dial steps through. 0 == unlimited
// (no pacing). Ordered high→low so "step down" = safer/slower.
var RPMPresets = []int{0, 40, 35, 30, 25, 20, 15, 12, 10, 8, 6, 5, 4, 3, 2}

// DefaultRPM is the shipped starting point: conservative enough to glide under
// NIM's "up to 40" on a typical day, high enough not to feel throttled. The
// user is expected to tune from here.
const DefaultRPM = 25

var (
	rateLimitRPM  = DefaultRPM
	rateLimitOnce sync.Once
	rateLimitMu   sync.RWMutex
)

func rateLimitPath() string {
	return filepath.Join(loadoutConfigBase(), rateLimitFileName)
}

type rateLimitFile struct {
	RequestsPerMinute int `json:"requests_per_minute"`
}

func initRateLimit() {
	rateLimitOnce.Do(func() {
		if data, err := os.ReadFile(rateLimitPath()); err == nil {
			var f rateLimitFile
			if json.Unmarshal(data, &f) == nil && f.RequestsPerMinute >= 0 {
				rateLimitMu.Lock()
				rateLimitRPM = f.RequestsPerMinute
				rateLimitMu.Unlock()
			}
		}
	})
}

// RateLimitRPM returns the current requests-per-minute cap. 0 means unlimited
// (no pacing). Read by the provider layer before every outbound request.
func RateLimitRPM() int {
	initRateLimit()
	rateLimitMu.RLock()
	defer rateLimitMu.RUnlock()
	return rateLimitRPM
}

// SetRateLimitRPM sets and persists the cap. Values < 0 are clamped to 0
// (unlimited).
func SetRateLimitRPM(n int) {
	initRateLimit()
	if n < 0 {
		n = 0
	}
	rateLimitMu.Lock()
	rateLimitRPM = n
	rateLimitMu.Unlock()
	saveRateLimit(n)
}

// StepRateLimitRPM moves the cap one rung along RPMPresets. dir<0 = slower
// (down the ladder / safer), dir>0 = faster (up the ladder). Snaps the current
// value to the nearest preset first so stepping is always predictable.
func StepRateLimitRPM(dir int) int {
	cur := RateLimitRPM()
	// find nearest preset index
	idx := 0
	best := 1 << 30
	for i, p := range RPMPresets {
		d := p - cur
		if d < 0 {
			d = -d
		}
		if d < best {
			best, idx = d, i
		}
	}
	// RPMPresets is high→low, so "faster" (dir>0) means a LOWER index.
	if dir > 0 {
		idx--
	} else if dir < 0 {
		idx++
	}
	if idx < 0 {
		idx = 0
	}
	if idx >= len(RPMPresets) {
		idx = len(RPMPresets) - 1
	}
	SetRateLimitRPM(RPMPresets[idx])
	return RPMPresets[idx]
}

func saveRateLimit(n int) {
	_ = os.MkdirAll(loadoutConfigBase(), 0o755)
	data, _ := json.MarshalIndent(rateLimitFile{RequestsPerMinute: n}, "", " ")
	_ = os.WriteFile(rateLimitPath(), data, 0o600)
}
