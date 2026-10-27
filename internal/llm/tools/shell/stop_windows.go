//go:build windows

package shell

import (
	"fmt"
	"os"
	"sync/atomic"

	"golang.org/x/sys/windows"
)

// stopSignal is a named auto-reset event the PowerShell session waits on
// (GorillaStop in psSessionSetup). Setting it stops the pipeline that is
// running, the same as Ctrl+C at a console, so a cmdlet (Start-Sleep,
// Get-ChildItem -Recurse) can be interrupted without ending the session
// (gap #10). The name is per shell, in this logon session's Local\ namespace.
type stopSignal struct {
	name string
	h    windows.Handle
}

var stopSeq atomic.Int64

func newStopSignal() *stopSignal {
	name := fmt.Sprintf(`Local\gorilla-opencode-stop-%d-%d`, os.Getpid(), stopSeq.Add(1))
	p, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil
	}
	h, err := windows.CreateEvent(nil, 0, 0, p)
	if err != nil {
		return nil
	}
	return &stopSignal{name: name, h: h}
}

// armScript is the line that connects the session to this signal.
func (s *stopSignal) armScript() string {
	if s == nil {
		return ""
	}
	return "if ('GorillaStop' -as [type]) { [GorillaStop]::Arm([runspace]::DefaultRunspace, " + shellQuotePS(s.name) + ") }\n"
}

func (s *stopSignal) fire() bool {
	return s != nil && windows.SetEvent(s.h) == nil
}

func (s *stopSignal) close() {
	if s != nil && s.h != 0 {
		windows.CloseHandle(s.h)
		s.h = 0
	}
}
