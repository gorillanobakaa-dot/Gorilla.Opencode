//go:build windows

package peers

// GORILLA (2026-10-10): the Windows endpoint, a named pipe.
//
//	\\.\pipe\LOCAL\gorilla-opencode-<pid>
//
// Its security descriptor is built from this process's own token as
//
//	D:P(A;;GA;;;<current user SID>)
//
// a protected DACL with one entry: the current user, full access. Nobody else
// is named, so nobody else is granted anything — not another account, not
// Administrators, not SYSTEM, not Everyone. TestPipeGrantsOnlyTheCurrentUser
// reads the DACL back from the live pipe and checks exactly that.
//
// PIPE_REJECT_REMOTE_CLIENTS refuses connections arriving through the SMB
// redirector, so the pipe cannot be reached from another machine even by the
// same account. FILE_FLAG_FIRST_PIPE_INSTANCE on the first instance makes
// opening fail if something else already holds the name, rather than joining
// a pipe somebody planted.
//
// Clients connect with SECURITY_SQOS_PRESENT|SECURITY_IDENTIFICATION, so the
// server can learn who called but cannot act as them, and refuse to talk to a
// server that is not the process the register names
// (GetNamedPipeServerProcessId). The server records the client's process
// (GetNamedPipeClientProcessId) and refuses a request that claims another.
//
// All I/O is overlapped, so every read, write and wait has a deadline and Close
// can stop a pending accept.

import (
	"fmt"
	"io"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const pipePrefix = `\\.\pipe\LOCAL\gorilla-opencode-`

// stillActive is STILL_ACTIVE, the exit code of a process that has not exited.
const stillActive = 259

func endpointFor(dir, key string) string { return pipePrefix + key }

func removeEndpointFile(dir, key string) {}

func checkDirOwner(path string) error { return nil }

// pidAlive reports whether a process with this number exists. A process that
// exists but cannot be queried (another account's) counts as alive.
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return err == windows.ERROR_ACCESS_DENIED
	}
	defer windows.CloseHandle(h)
	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code == stillActive
}

// currentUserSID is the SID of the account this process runs as.
func currentUserSID() (*windows.SID, error) {
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	return u.User.Sid, nil
}

// pipeSDDL is the security descriptor for the pipe: the current user only.
func pipeSDDL(sid string) string { return "D:P(A;;GA;;;" + sid + ")" }

type pipeListener struct {
	name *uint16
	sd   *windows.SECURITY_DESCRIPTOR // kept alive: sa points into it
	sa   *windows.SecurityAttributes
	stop windows.Handle // manual-reset event, set by Close

	mu     sync.Mutex
	closed bool
	next   windows.Handle // the instance the next Accept waits on
}

func listen(endpoint string) (listener, error) {
	sid, err := currentUserSID()
	if err != nil {
		return nil, fmt.Errorf("reading this account's identity: %w", err)
	}
	sd, err := windows.SecurityDescriptorFromString(pipeSDDL(sid.String()))
	if err != nil {
		return nil, fmt.Errorf("building the pipe's access list: %w", err)
	}
	name, err := windows.UTF16PtrFromString(endpoint)
	if err != nil {
		return nil, err
	}
	stop, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return nil, err
	}
	l := &pipeListener{name: name, sd: sd, stop: stop}
	l.sa = &windows.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		SecurityDescriptor: sd,
		InheritHandle:      0,
	}
	h, err := l.instance(true)
	if err != nil {
		windows.CloseHandle(stop)
		return nil, err
	}
	l.next = h
	return l, nil
}

// instance creates one pipe instance. Only the first may create the name.
func (l *pipeListener) instance(first bool) (windows.Handle, error) {
	flags := uint32(windows.PIPE_ACCESS_DUPLEX | windows.FILE_FLAG_OVERLAPPED)
	if first {
		flags |= windows.FILE_FLAG_FIRST_PIPE_INSTANCE
	}
	mode := uint32(windows.PIPE_TYPE_BYTE | windows.PIPE_READMODE_BYTE | windows.PIPE_WAIT | windows.PIPE_REJECT_REMOTE_CLIENTS)
	return windows.CreateNamedPipe(l.name, flags, mode, windows.PIPE_UNLIMITED_INSTANCES, 64*1024, 64*1024, 0, l.sa)
}

func (l *pipeListener) Accept() (conn, error) {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil, errClosed
	}
	h := l.next
	l.next = 0
	l.mu.Unlock()
	if h == 0 {
		var err error
		if h, err = l.instance(false); err != nil {
			return nil, err
		}
	}
	if err := l.connect(h); err != nil {
		windows.CloseHandle(h)
		return nil, err
	}
	// The next instance is created before this one is handed over, so the name
	// never disappears while the server runs: a client arriving now finds a free
	// instance or, at worst, a busy one it retries, never "not found".
	if nh, err := l.instance(false); err == nil {
		l.mu.Lock()
		if l.closed || l.next != 0 {
			windows.CloseHandle(nh)
		} else {
			l.next = nh
		}
		l.mu.Unlock()
	}
	var pid uint32
	if err := windows.GetNamedPipeClientProcessId(h, &pid); err != nil || pid == 0 {
		// Fail closed: a caller this program cannot identify is not served.
		windows.CloseHandle(h)
		return nil, fmt.Errorf("could not identify the connecting process: %v", err)
	}
	return &pipeConn{h: h, peer: int(pid)}, nil
}

// connect waits for a client on h, or for Close.
func (l *pipeListener) connect(h windows.Handle) error {
	ev, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(ev)
	ov := &windows.Overlapped{HEvent: ev}
	err = windows.ConnectNamedPipe(h, ov)
	if err == windows.ERROR_PIPE_CONNECTED {
		return nil // a client arrived between creating the instance and this call
	}
	_, err = waitIO(h, ov, err, time.Time{}, l.stop)
	return err
}

func (l *pipeListener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil
	}
	l.closed = true
	_ = windows.SetEvent(l.stop)
	if l.next != 0 {
		windows.CloseHandle(l.next)
		l.next = 0
	}
	return nil
}

func (l *pipeListener) free() { windows.CloseHandle(l.stop) }

// waitIO completes one overlapped operation started with result opErr, waiting
// until deadline (zero: no deadline) or until stop is set (zero: no stop).
// A wait that ends early cancels the operation and waits for the cancellation
// to land, so the kernel never writes into an Overlapped that has gone.
func waitIO(h windows.Handle, ov *windows.Overlapped, opErr error, deadline time.Time, stop windows.Handle) (uint32, error) {
	if opErr != nil && opErr != windows.ERROR_IO_PENDING {
		return 0, opErr
	}
	if opErr == windows.ERROR_IO_PENDING {
		ms := uint32(windows.INFINITE)
		if !deadline.IsZero() {
			d := time.Until(deadline)
			if d < 0 {
				d = 0
			}
			ms = uint32(d / time.Millisecond)
		}
		handles := []windows.Handle{ov.HEvent}
		if stop != 0 {
			handles = append(handles, stop)
		}
		ev, err := windows.WaitForMultipleObjects(handles, false, ms)
		if err != nil || ev != windows.WAIT_OBJECT_0 {
			_ = windows.CancelIoEx(h, ov)
			var n uint32
			_ = windows.GetOverlappedResult(h, ov, &n, true)
			switch {
			case err != nil:
				return 0, err
			case ev == uint32(windows.WAIT_TIMEOUT):
				return 0, errTimeout
			default:
				return 0, errClosed
			}
		}
	}
	var n uint32
	if err := windows.GetOverlappedResult(h, ov, &n, false); err != nil {
		return n, err
	}
	return n, nil
}

type pipeConn struct {
	h    windows.Handle
	peer int

	mu       sync.Mutex
	deadline time.Time
	once     sync.Once
}

func (c *pipeConn) SetDeadline(t time.Time) error {
	c.mu.Lock()
	c.deadline = t
	c.mu.Unlock()
	return nil
}

func (c *pipeConn) currentDeadline() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.deadline
}

func (c *pipeConn) PeerPID() int { return c.peer }

// do runs one overlapped operation against the connection's deadline.
func (c *pipeConn) do(op func(ov *windows.Overlapped) error) (int, error) {
	ev, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(ev)
	ov := &windows.Overlapped{HEvent: ev}
	n, err := waitIO(c.h, ov, op(ov), c.currentDeadline(), 0)
	return int(n), err
}

func (c *pipeConn) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	n, err := c.do(func(ov *windows.Overlapped) error {
		var done uint32
		return windows.ReadFile(c.h, p, &done, ov)
	})
	if err == windows.ERROR_BROKEN_PIPE || err == windows.ERROR_PIPE_NOT_CONNECTED {
		return n, io.EOF
	}
	if err != nil {
		return n, err
	}
	if n == 0 {
		return 0, io.EOF
	}
	return n, nil
}

func (c *pipeConn) Write(p []byte) (int, error) {
	total := 0
	for total < len(p) {
		chunk := p[total:]
		n, err := c.do(func(ov *windows.Overlapped) error {
			var done uint32
			return windows.WriteFile(c.h, chunk, &done, ov)
		})
		total += n
		if err != nil {
			return total, err
		}
		if n == 0 {
			return total, io.ErrShortWrite
		}
	}
	return total, nil
}

func (c *pipeConn) Close() error {
	var err error
	c.once.Do(func() { err = windows.CloseHandle(c.h) })
	return err
}

// dial connects to a session's pipe and learns which process serves it.
func dial(endpoint string, deadline time.Time) (conn, error) {
	name, err := windows.UTF16PtrFromString(endpoint)
	if err != nil {
		return nil, err
	}
	for {
		h, err := windows.CreateFile(name,
			windows.GENERIC_READ|windows.GENERIC_WRITE,
			0, nil, windows.OPEN_EXISTING,
			windows.FILE_FLAG_OVERLAPPED|windows.SECURITY_SQOS_PRESENT|windows.SECURITY_IDENTIFICATION,
			0)
		if err == nil {
			var pid uint32
			if err := windows.GetNamedPipeServerProcessId(h, &pid); err != nil || pid == 0 {
				windows.CloseHandle(h)
				return nil, fmt.Errorf("could not tell which process answers at %s: %v", endpoint, err)
			}
			return &pipeConn{h: h, peer: int(pid)}, nil
		}
		switch {
		case err == windows.ERROR_FILE_NOT_FOUND || err == windows.ERROR_PATH_NOT_FOUND:
			return nil, errNotRunning
		case err == windows.ERROR_PIPE_BUSY && time.Now().Before(deadline):
			time.Sleep(20 * time.Millisecond)
		default:
			return nil, err
		}
	}
}
