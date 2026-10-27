package tools

import (
	"runtime"
	"testing"
)

// TestURIDiagnosticsSameFilePath: a diagnostic's URI, turned back into a
// path, matches the file the tool was asked about however Windows spells it.
func TestURIDiagnosticsSameFilePath(t *testing.T) {
	if runtime.GOOS != "windows" {
		if !sameFilePath("/a/b.go", "/a/./b.go") || sameFilePath("/a/B.go", "/a/b.go") {
			t.Error("POSIX paths compare after Clean, case-sensitively")
		}
		return
	}
	if !sameFilePath(`C:\Proj\main.go`, `c:\proj\MAIN.go`) || !sameFilePath(`C:\Proj\main.go`, `C:/Proj/main.go`) {
		t.Error("Windows paths must compare case-insensitively and by either slash")
	}
	if sameFilePath(`C:\Proj\main.go`, `C:\Proj\other.go`) {
		t.Error("different files compared equal")
	}
}
