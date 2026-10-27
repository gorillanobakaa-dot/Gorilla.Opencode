package tools

import (
	"os"
	"strings"
)

// expandShellPathWord resolves the spellings a command uses to reach a
// person's own folders and the system's (gap #22): ~, $HOME, and on Windows
// $env:NAME, ${env:NAME} and %NAME% for any variable (APPDATA, LOCALAPPDATA,
// ProgramData, TEMP, USERPROFILE ...). It reads the environment; it does not
// run anything. A variable that is not set leaves the word as written.
func expandShellPathWord(w, home string) string {
	low := strings.ToLower(w)
	if home != "" {
		for _, h := range []string{"~", "$home", "${home}"} {
			if low == h || strings.HasPrefix(low, h+"/") || strings.HasPrefix(low, h+`\`) {
				return home + w[len(h):]
			}
		}
	}
	var name, rest string
	switch {
	case strings.HasPrefix(low, "${env:"):
		end := strings.IndexByte(w, '}')
		if end < 0 {
			return w
		}
		name, rest = w[len("${env:"):end], w[end+1:]
	case strings.HasPrefix(low, "$env:"):
		end := strings.IndexAny(w[len("$env:"):], `/\`)
		if end < 0 {
			end = len(w) - len("$env:")
		}
		name, rest = w[len("$env:"):len("$env:")+end], w[len("$env:")+end:]
	case strings.HasPrefix(w, "%"):
		end := strings.IndexByte(w[1:], '%')
		if end < 0 {
			return w
		}
		name, rest = w[1:1+end], w[2+end:]
	default:
		return w
	}
	if rest != "" && rest[0] != '/' && rest[0] != '\\' {
		return w
	}
	v := os.Getenv(name)
	if v == "" && strings.EqualFold(name, "userprofile") {
		v = home
	}
	if v == "" {
		return w
	}
	return v + rest
}

// registryPath reports a PowerShell registry path (HKLM:\..., HKCU:\...,
// Registry::HKEY_...). It is not a file, but it is a place a command can
// change, and a permission request about it must name it rather than the
// working directory.
func registryPath(w string) bool {
	low := strings.ToLower(w)
	for _, p := range []string{"hklm:", "hkcu:", "hkcr:", "hku:", "hkcc:", "registry::"} {
		if strings.HasPrefix(low, p) {
			return true
		}
	}
	return false
}
