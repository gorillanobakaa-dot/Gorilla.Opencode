package tools

// Gap #7: PowerShell's read-only cmdlets asked permission every time,
// although the tool guidance tells the model to use them. Gap #22: $env:
// paths and registry drives in permission scoping. Test names end in a
// lowercase token because `go test -run` compiles each top-level
// alternative of the goal's pattern on its own.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
)

func TestPowerShellReadOnlyPipelines_readonly(t *testing.T) {
	list := powerShellReadOnlyList()
	readOnly := []string{
		`Get-ChildItem -Path "C:\Path\To\Folder"`,
		`Get-ChildItem -Path "C:\Path\To\Folder" -Recurse`,
		`Get-ChildItem -Path "C:\Path\To\Folder" -File`,
		`Get-ChildItem -Path "C:\Path\To\Folder" -Directory`,
		`Get-ChildItem -Path "C:\Path\To\Folder" -Hidden`,
		`Get-Item -Path "C:\Path\To\File.txt"`,
		`(Get-Item "C:\Path\To\File.txt").Attributes`,
		`Get-ItemProperty -Path "C:\Path\To\File.txt"`,
		`gci -Recurse -Filter *.go | Measure-Object`,
		`Get-Content .\README.md | Select-String TODO`,
		`gc notes.txt -Tail 20`,
		`Test-Path C:\Windows`,
		`Get-Command git`,
		`Get-Location`,
		`Resolve-Path .\x`,
		`Get-FileHash .\build.zip -Algorithm SHA256`,
		`Get-Acl C:\Users\Public | Format-List`,
		`dir | Sort-Object Length | Select-Object -First 5`,
		`sls -Pattern foo -Path *.txt`,
		`Get-ItemProperty HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion`,
	}
	for _, c := range readOnly {
		if !IsSafeReadOnly(c, list) {
			t.Errorf("read-only in PowerShell, but asks: %s", c)
		}
	}
	asks := []string{
		// A tail that writes makes the whole pipeline ask.
		`Get-ChildItem -File | Rename-Item -NewName { $_.Name -replace 'a', 'b' }`,
		`Get-ChildItem -File | Rename-Item -NewName x`,
		`Get-ChildItem *.tmp | Remove-Item`,
		`Get-Item x | Set-ItemProperty -Name Attributes -Value ReadOnly`,
		`Get-ChildItem; Remove-Item x`,
		`(Get-Item "C:\x.txt").IsReadOnly = $false`,
		`(Get-Item "C:\x.txt").Delete()`,
		`(Remove-Item C:\x).Name`,
		`Get-Content x > y.txt`,
		`Get-ChildItem $(Remove-Item x)`,
		`Get-Content x | iex`,
		// Credentials stay behind the prompt, however they are spelled.
		`Get-Content $env:USERPROFILE\.aws\credentials`,
		`type $env:USERPROFILE\.aws\credentials`,
		`cat ~\.ssh\id_rsa`,
		`gc %USERPROFILE%\.ssh\id_ed25519`,
		// Unix-only names are not on the PowerShell list.
		`head -5 x`,
		`wc -l x`,
		`which git`,
	}
	for _, c := range asks {
		if IsSafeReadOnly(c, list) {
			t.Errorf("must ask in PowerShell, but counted as read-only: %s", c)
		}
	}
}

// The bash list is unchanged: no PowerShell cmdlets leak into it, and its
// Unix entries are still there.
func TestPosixListUnchanged_readonly(t *testing.T) {
	posix := append(append([]string{}, safeReadOnlyCommands...), readOnlyAdditions...)
	posix = append(posix, unixOnlyReadOnly...)
	if IsSafeReadOnly("Get-ChildItem x", posix) {
		t.Error("a PowerShell cmdlet is read-only in bash")
	}
	if !IsSafeReadOnly("head -5 README.md", posix) || !IsSafeReadOnly("wc -l x", posix) {
		t.Error("bash lost a Unix read-only command")
	}
	if runtime.GOOS != "windows" {
		if !IsSafeReadOnly("head -5 README.md", readOnlyList()) {
			t.Error("readOnlyList dropped Unix commands outside PowerShell")
		}
	}
}

func TestUnwrapPropertyReads_readonly(t *testing.T) {
	cases := map[string]string{
		`(Get-Item "C:\a b.txt").Attributes`:       ` Get-Item "C:\a b.txt" `,
		`(gi x).Length | Format-List`:              ` gi x |` + ` Format-List`,
		`(Get-Item x).IsReadOnly = $false`:         `(Get-Item x).IsReadOnly = $false`,
		`(Get-Item x).Delete()`:                    `(Get-Item x).Delete()`,
		`(Get-Item $env:TEMP).Attributes`:          `(Get-Item $env:TEMP).Attributes`,
		`echo (Remove-Item -Recurse C:\x).Length;`: `echo (Remove-Item -Recurse C:\x).Length;`,
	}
	for in, want := range cases {
		if got := unwrapPropertyReads(in); got != want {
			t.Errorf("unwrapPropertyReads(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBashRequestPathResolvesEnvAndRegistrySecurity(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows environment spellings")
	}
	ws := config.WorkingDirectory()
	appdata := filepath.Join(t.TempDir(), "Roaming")
	t.Setenv("APPDATA", appdata)
	t.Setenv("LOCALAPPDATA", filepath.Join(t.TempDir(), "Local"))
	t.Setenv("ProgramData", `C:\ProgramData`)
	cases := map[string]string{
		`Remove-Item $env:APPDATA\Code\x.json`:               filepath.Join(appdata, `Code\x.json`),
		`Remove-Item ${env:APPDATA}\Code\x.json`:             filepath.Join(appdata, `Code\x.json`),
		`del %APPDATA%\Code\x.json`:                          filepath.Join(appdata, `Code\x.json`),
		`Remove-Item "$env:LOCALAPPDATA\Temp\y"`:             filepath.Join(os.Getenv("LOCALAPPDATA"), `Temp\y`),
		`Copy-Item a.txt $env:ProgramData\Vendor\a.txt`:      `C:\ProgramData\Vendor\a.txt`,
		`Remove-Item HKLM:\SOFTWARE\Vendor -Recurse`:         `HKLM:\SOFTWARE\Vendor`,
		`Set-ItemProperty HKCU:\Software\X -Name a -Value 1`: `HKCU:\Software\X`,
	}
	for cmd, want := range cases {
		if got := bashRequestPath(cmd); !strings.EqualFold(got, want) {
			t.Errorf("bashRequestPath(%q) = %q, want %q", cmd, got, want)
		}
	}
	if got := bashRequestPath(`Get-ChildItem src`); got != ws {
		t.Errorf("a workspace path should give the workspace, got %q", got)
	}
}
