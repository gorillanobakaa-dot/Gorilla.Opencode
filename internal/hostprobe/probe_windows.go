//go:build windows

package hostprobe

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"github.com/opencode-ai/opencode/internal/llm/tools/shell"
)

func probeOS(f *Facts) {
	f.OSName = windowsName()
	if windows.GetCurrentProcessToken().IsElevated() {
		f.Admin = "yes"
	} else {
		f.Admin = "no"
	}
	f.Shell = ShellEdition(shell.ShellPath())
	f.PowerShells = installedPowerShells()
	f.LongPaths = onOff(regDword(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\FileSystem`, "LongPathsEnabled"))
	f.DevMode = onOff(regDword(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\AppModelUnlock`, "AllowDevelopmentWithoutDevLicense"))
	f.MSVC, f.MSVCDevCmd = msvc(filepath.Join(os.Getenv("ProgramData"), "Microsoft", "VisualStudio", "Packages", "_Instances"))
	_, err := exec.LookPath("wt.exe")
	f.Terminal = err == nil
	f.PowerToys, f.PowerToysCLIs = powerToys(powerToysDirs())
	f.Sysinternals = lookSysinternals()
}

// windowsName is "Windows 11 Home 25H2 (build 26200.6899)". ProductName still
// says "Windows 10" on Windows 11 (Microsoft never changed the value), so the
// build number decides: 22000 is the first Windows 11 build.
func windowsName() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	product, _, _ := k.GetStringValue("ProductName")
	display, _, _ := k.GetStringValue("DisplayVersion")
	build, _, _ := k.GetStringValue("CurrentBuildNumber")
	ubr, _, _ := k.GetIntegerValue("UBR")
	return windowsNameFrom(product, display, build, ubr)
}

func windowsNameFrom(product, display, build string, ubr uint64) string {
	var n int
	fmt.Sscanf(build, "%d", &n)
	if n >= 22000 && strings.HasPrefix(product, "Windows 10") {
		product = "Windows 11" + strings.TrimPrefix(product, "Windows 10")
	}
	s := product
	if display != "" {
		s += " " + display
	}
	if build != "" {
		s += " (build " + build
		if ubr > 0 {
			s += fmt.Sprintf(".%d", ubr)
		}
		s += ")"
	}
	return strings.TrimSpace(s)
}

func regDword(root registry.Key, path, name string) int {
	k, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
	if err != nil {
		return -1
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue(name)
	if err != nil {
		return -1
	}
	return int(v)
}

func onOff(v int) string {
	switch {
	case v < 0:
		return "off" // absent means the Windows default, which is off for both
	case v == 0:
		return "off"
	}
	return "on"
}

// winPowerShellVersion is Windows PowerShell's own version, e.g. 5.1.26100.1.
func winPowerShellVersion() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\PowerShell\3\PowerShellEngine`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	v, _, _ := k.GetStringValue("PowerShellVersion")
	return v
}

// pwshVersion is the highest PowerShell 7 the MSI installer registered, e.g.
// 7.5.0. A pwsh from a zip or a store install has no key; the caller then says
// "7+".
func pwshVersion() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\PowerShellCore\InstalledVersions`, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return ""
	}
	defer k.Close()
	subs, _ := k.ReadSubKeyNames(-1)
	var versions []string
	for _, s := range subs {
		sk, err := registry.OpenKey(k, s, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		if v, _, err := sk.GetStringValue("SemanticVersion"); err == nil && v != "" {
			versions = append(versions, v)
		}
		sk.Close()
	}
	if len(versions) == 0 {
		return ""
	}
	sort.Strings(versions)
	return versions[len(versions)-1]
}

func installedPowerShells() []string {
	var out []string
	if _, err := exec.LookPath("pwsh.exe"); err == nil {
		out = append(out, ShellEdition("pwsh.exe"))
	}
	if _, err := exec.LookPath("powershell.exe"); err == nil {
		out = append(out, ShellEdition("powershell.exe"))
	}
	return out
}

// msvc reads Visual Studio's own instance records (the files vswhere.exe
// reads) and names the newest instance that has the C++ toolset (VC\Tools\MSVC),
// with its VsDevCmd.bat when that file is there.
func msvc(instances string) (string, string) {
	entries, err := os.ReadDir(instances)
	if err != nil {
		return "", ""
	}
	best, bestVer, bestCmd := "", "", ""
	for _, e := range entries {
		raw, err := os.ReadFile(filepath.Join(instances, e.Name(), "state.json"))
		if err != nil {
			continue
		}
		var st struct {
			InstallationPath    string `json:"installationPath"`
			InstallationVersion string `json:"installationVersion"`
			CatalogInfo         struct {
				ProductDisplayVersion string `json:"productDisplayVersion"`
				ProductLineVersion    string `json:"productLineVersion"`
			} `json:"catalogInfo"`
			LocalizedResources []struct {
				Language string `json:"language"`
				Title    string `json:"title"`
			} `json:"localizedResources"`
		}
		if json.Unmarshal(raw, &st) != nil || st.InstallationPath == "" {
			continue
		}
		if fi, err := os.Stat(filepath.Join(st.InstallationPath, "VC", "Tools", "MSVC")); err != nil || !fi.IsDir() {
			continue
		}
		title := "Visual Studio " + st.CatalogInfo.ProductLineVersion
		for _, r := range st.LocalizedResources {
			if strings.EqualFold(r.Language, "en-us") && r.Title != "" {
				title = r.Title
			}
		}
		name := strings.TrimSpace(title + " " + st.CatalogInfo.ProductDisplayVersion)
		if st.InstallationVersion > bestVer {
			best, bestVer, bestCmd = name, st.InstallationVersion, ""
			if c := filepath.Join(st.InstallationPath, "Common7", "Tools", "VsDevCmd.bat"); fileExists(c) {
				bestCmd = c
			}
		}
	}
	return best, bestCmd
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

// powerToysDirs are where the per-user and the machine-wide installers put it.
func powerToysDirs() []string {
	return []string{
		filepath.Join(os.Getenv("LOCALAPPDATA"), "PowerToys"),
		filepath.Join(os.Getenv("ProgramFiles"), "PowerToys"),
	}
}

// powerToys returns the first folder holding PowerToys.exe and the
// command-line programs found in it. Which modules have a CLI is read from the
// installed files (a name with "CLI" in it, and Awake, whose own exe takes
// arguments), never from memory: modules come and go between releases.
func powerToys(dirs []string) (string, []string) {
	for _, d := range dirs {
		if _, err := os.Stat(filepath.Join(d, "PowerToys.exe")); err != nil {
			continue
		}
		var clis []string
		for _, sub := range []string{d, filepath.Join(d, "WinUI3Apps")} {
			entries, _ := os.ReadDir(sub)
			for _, e := range entries {
				n := e.Name()
				ln := strings.ToLower(n)
				if e.IsDir() || !strings.HasSuffix(ln, ".exe") {
					continue
				}
				if strings.Contains(ln, "cli") || ln == "powertoys.awake.exe" {
					clis = append(clis, n)
				}
			}
		}
		return d, sortedUnique(clis)
	}
	return "", nil
}
