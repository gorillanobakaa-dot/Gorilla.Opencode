//go:build windows

// The pseudoconsole wrapper below (pty, newPty, Close) is adapted from
// microsoft/hcsshim internal/conpty, tag v0.15.0, commit
// 900183417d29765de2947de22a6e72c22c195550 (third_party/PINS.md).
//
//	The MIT License (MIT)
//	Copyright (c) 2015 Microsoft
//	Full text: third_party/hcsshim/LICENSE
//
// Changed: x/sys/windows calls instead of hcsshim's internal winapi package,
// no resize (the console is used for one command), and Close drains the
// output while ClosePseudoConsole runs, because on older Windows builds
// ClosePseudoConsole blocks until the output pipe is read.

package shell

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/procjob"
	"golang.org/x/sys/windows"
)

// ConsoleSupported reports whether RunInConsole works on this platform.
func ConsoleSupported() bool { return true }

type pty struct {
	mu      sync.Mutex
	hpc     windows.Handle
	inPipe  *os.File // ours: what we write is typed into the console
	outPipe *os.File // ours: what the console draws
}

func newPty(width, height int16) (*pty, error) {
	ptyIn, inOurs, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("pseudo console pipes: %w", err)
	}
	outOurs, ptyOut, err := os.Pipe()
	if err != nil {
		ptyIn.Close()
		inOurs.Close()
		return nil, fmt.Errorf("pseudo console pipes: %w", err)
	}
	var hpc windows.Handle
	err = windows.CreatePseudoConsole(windows.Coord{X: width, Y: height},
		windows.Handle(ptyIn.Fd()), windows.Handle(ptyOut.Fd()), 0, &hpc)
	// The console host has its own duplicates of these two ends now.
	ptyIn.Close()
	ptyOut.Close()
	if err != nil {
		inOurs.Close()
		outOurs.Close()
		return nil, fmt.Errorf("CreatePseudoConsole: %w", err)
	}
	return &pty{hpc: hpc, inPipe: inOurs, outPipe: outOurs}, nil
}

// close ends the pseudoconsole. The caller must still be reading outPipe.
func (p *pty) close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.hpc == 0 {
		return
	}
	windows.ClosePseudoConsole(p.hpc)
	p.hpc = 0
	p.inPipe.Close()
}

// ConsoleResult is what a console command left behind.
type ConsoleResult struct {
	Output      string // the console's text, escape sequences removed
	ExitCode    int
	Interrupted bool // stopped by the context or the timeout
}

// consoleCommandLine is the command line for one command in shellPath.
// PowerShell gets it as -EncodedCommand, which no quoting can break.
func consoleCommandLine(shellPath, command string) string {
	if looksLikePowerShell(shellPath) {
		u := utf16.Encode([]rune(command))
		b := make([]byte, 2*len(u))
		for i, r := range u {
			b[2*i], b[2*i+1] = byte(r), byte(r>>8)
		}
		args := PowerShellArgs("-EncodedCommand", base64.StdEncoding.EncodeToString(b))
		return windows.ComposeCommandLine(append([]string{shellPath}, args...))
	}
	if strings.Contains(strings.ToLower(shellPath), "cmd") {
		return windows.ComposeCommandLine([]string{shellPath, "/d", "/s", "/c"}) + ` "` + command + `"`
	}
	return windows.ComposeCommandLine([]string{shellPath, "-c", command})
}

// RunInConsole runs one command in a pseudoconsole (ConPTY, gap #11): the
// program sees a real console, so Read-Host, a y/n prompt and anything that
// checks isatty work. input is typed into the console as it starts, one line
// per answer. It is a separate process in its own Job Object, not the
// persistent session: variables set there do not carry over, and the whole
// tree is killed when it times out or the context ends.
func RunInConsole(ctx context.Context, shellPath, command, cwd, input string, timeout time.Duration) (ConsoleResult, error) {
	if shellPath == "" {
		if cfg := config.Get(); cfg != nil {
			shellPath = cfg.Shell.Path
		}
	}
	if shellPath == "" {
		shellPath = resolveShellPath()
	}
	p, err := newPty(200, 50)
	if err != nil {
		return ConsoleResult{}, err
	}
	defer p.close()

	attrs, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return ConsoleResult{}, err
	}
	defer attrs.Delete()
	// The attribute's value IS the HPCON (not a pointer to it). Read through
	// a pointer so no uintptr becomes an unsafe.Pointer (go vet).
	hpcon := *(*unsafe.Pointer)(unsafe.Pointer(&p.hpc))
	if err := attrs.Update(windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, hpcon, unsafe.Sizeof(p.hpc)); err != nil {
		return ConsoleResult{}, fmt.Errorf("attach the pseudo console: %w", err)
	}
	si := &windows.StartupInfoEx{ProcThreadAttributeList: attrs.List()}
	si.Cb = uint32(unsafe.Sizeof(*si))
	// Empty standard handles on purpose: without STARTF_USESTDHANDLES a
	// child of a process whose own output is redirected (a test runner, a
	// headless run) writes to those redirections, not to the console.
	si.Flags = windows.STARTF_USESTDHANDLES

	cmdLine, err := windows.UTF16PtrFromString(consoleCommandLine(shellPath, command))
	if err != nil {
		return ConsoleResult{}, err
	}
	var dir *uint16
	// A folder that is gone would fail CreateProcess ("The directory name
	// is invalid"); run in this process's own folder instead, as the
	// persistent shell does.
	if st, serr := os.Stat(cwd); cwd != "" && (serr != nil || !st.IsDir()) {
		cwd = ""
	}
	if cwd != "" {
		if dir, err = windows.UTF16PtrFromString(cwd); err != nil {
			return ConsoleResult{}, err
		}
	}
	env := envBlock(shellChildEnv(append(os.Environ(), "GIT_EDITOR=true"), true))

	job, err := procjob.New()
	if err != nil {
		return ConsoleResult{}, err
	}
	defer job.Close()

	var pi windows.ProcessInformation
	flags := uint32(windows.EXTENDED_STARTUPINFO_PRESENT | windows.CREATE_UNICODE_ENVIRONMENT | windows.CREATE_SUSPENDED)
	if err := windows.CreateProcess(nil, cmdLine, nil, nil, false, flags, &env[0], dir, &si.StartupInfo, &pi); err != nil {
		return ConsoleResult{}, fmt.Errorf("start %s in a console: %w", shellPath, err)
	}
	defer windows.CloseHandle(pi.Process)
	if err := windows.AssignProcessToJobObject(job.Handle(), pi.Process); err != nil {
		windows.TerminateProcess(pi.Process, 1)
		windows.CloseHandle(pi.Thread)
		return ConsoleResult{}, fmt.Errorf("assign the console command to its job: %w", err)
	}
	windows.ResumeThread(pi.Thread)
	windows.CloseHandle(pi.Thread)

	var out bytes.Buffer
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		_, _ = io.Copy(&out, io.LimitReader(p.outPipe, consoleOutputMax))
		_, _ = io.Copy(io.Discard, p.outPipe) // keep draining past the cap
	}()
	if input != "" {
		go func() { _, _ = p.inPipe.Write([]byte(consoleKeys(input))) }()
	}

	if timeout <= 0 {
		timeout = time.Minute
	}
	deadline := time.Now().Add(timeout)
	interrupted := false
	for {
		ev, _ := windows.WaitForSingleObject(pi.Process, 50)
		if ev == windows.WAIT_OBJECT_0 {
			break
		}
		if ctx.Err() != nil || time.Now().After(deadline) {
			interrupted = true
			_ = job.Terminate()
			windows.WaitForSingleObject(pi.Process, 5000)
			break
		}
	}
	var code uint32
	_ = windows.GetExitCodeProcess(pi.Process, &code)
	_ = job.Close() // anything the command left running in the background

	// The output pipe only ends once the pseudoconsole is closed, and the
	// close can wait for the output to be read: read and close together.
	closed := make(chan struct{})
	go func() { p.close(); close(closed) }()
	select {
	case <-readDone:
	case <-time.After(5 * time.Second):
	}
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
	}
	p.outPipe.Close()
	<-readDone

	res := ConsoleResult{Output: cleanConsoleOutput(out.String()), ExitCode: int(code), Interrupted: interrupted}
	if out.Len() >= consoleOutputMax {
		res.Output += fmt.Sprintf("\n[console output capped at %d bytes]", consoleOutputMax)
	}
	if interrupted {
		res.ExitCode = 143
	}
	return res, nil
}

const consoleOutputMax = 1 << 20

// consoleKeys turns text into what typing it looks like: Enter is CR.
func consoleKeys(input string) string {
	s := strings.ReplaceAll(input, "\r\n", "\n")
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return strings.ReplaceAll(s, "\n", "\r")
}

// envBlock is a CreateProcess environment: NUL-separated, double NUL end.
func envBlock(env []string) []uint16 {
	var b []uint16
	for _, kv := range env {
		if strings.IndexByte(kv, 0) >= 0 {
			continue
		}
		b = append(b, utf16.Encode([]rune(kv))...)
		b = append(b, 0)
	}
	return append(b, 0, 0)
}

var (
	// CSI (colors, cursor moves, erase), OSC (window title), and the
	// two-byte escapes; then what is left of the screen drawing.
	consoleCSI = regexp.MustCompile("\x1b\\[[0-?]*[ -/]*[@-~]")
	consoleOSC = regexp.MustCompile("\x1b\\][^\x07\x1b]*(\x07|\x1b\\\\)")
	consoleESC = regexp.MustCompile("\x1b[@-Z\\\\-_]")
)

// cleanConsoleOutput turns the console's drawing into plain text: escape
// sequences removed, CRLF to LF, trailing blanks per line dropped.
func cleanConsoleOutput(s string) string {
	s = consoleOSC.ReplaceAllString(s, "")
	s = consoleCSI.ReplaceAllString(s, "")
	s = consoleESC.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
