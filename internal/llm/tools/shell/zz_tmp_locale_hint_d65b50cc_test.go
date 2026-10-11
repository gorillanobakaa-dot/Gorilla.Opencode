package shell

import (
	"strings"
	"testing"
)

// Temporary review probe (d65b50cc): does the '&&' hint fire when the
// PowerShell UI culture is not English? Deleted after the run.
func TestTmpD65b50ccLocaleHint(t *testing.T) {
	s := newTestShell(t)
	if !s.isPowerShell {
		t.Skip("PowerShell only")
	}
	// Baseline: English UI culture.
	_, stderrEn, codeEn := exec1(t, s, `cd C:\ && echo hi`)
	t.Logf("EN code=%d hint=%v stderr=%q", codeEn, strings.Contains(stderrEn, "separate commands with ';'"), stderrEn)

	// Switch the persistent shell's UI culture, then check it stuck across chunks.
	_, se, c := exec1(t, s, `[System.Threading.Thread]::CurrentThread.CurrentUICulture = [System.Globalization.CultureInfo]::GetCultureInfo('es-ES')`)
	t.Logf("set culture: code=%d stderr=%q", c, se)
	out, _, _ := exec1(t, s, `[System.Threading.Thread]::CurrentThread.CurrentUICulture.Name`)
	t.Logf("culture now: %q", strings.TrimSpace(out))

	_, stderrEs, codeEs := exec1(t, s, `cd C:\ && echo hi`)
	t.Logf("ES code=%d hint=%v stderr=%q", codeEs, strings.Contains(stderrEs, "separate commands with ';'"), stderrEs)
	if strings.TrimSpace(out) == "es-ES" && !strings.Contains(stderrEs, "separate commands with ';'") {
		t.Errorf("REPRODUCED: es-ES UI culture, hint absent; stderr=%q", stderrEs)
	}
	if out, _, c := exec1(t, s, "Write-Output still_alive"); !strings.Contains(out, "still_alive") || c != 0 {
		t.Errorf("shell not alive after: out=%q code=%d", out, c)
	}
}
