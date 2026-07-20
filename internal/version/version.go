package version

import "runtime/debug"

// Build-time parameters set via -ldflags
var Version = "unknown"

// A user may install pug using `go install github.com/opencode-ai/opencode@latest`.
// without -ldflags, in which case the version above is unset. As a workaround
// we use the embedded build version that *is* set when using `go install` (and
// is only set for `go install` and not for `go build`).
func init() {
	// GORILLA OVERRIDE: an explicit -ldflags version always wins. The
	// fallback below assumed only `go install` stamps a module version,
	// but Go ≥1.22 stamps VCS pseudo-versions for plain `go build` too,
	// which silently overwrote the release version with
	// "v0.0.0-<date>-<sha>+dirty".
	if Version != "unknown" {
		return
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		// < go v1.18
		return
	}
	mainVersion := info.Main.Version
	if mainVersion == "" || mainVersion == "(devel)" {
		// bin not built using `go install`
		return
	}
	// bin built using `go install`
	Version = mainVersion
}
