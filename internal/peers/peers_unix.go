//go:build unix

package peers

// GORILLA (2026-10-10): the Linux and macOS endpoint, a Unix socket inside the
// register folder:
//
//	<state>/gorilla-opencode/peers/<pid>.sock
//
// The folder is 0700 and must belong to this account (Registry.ensure refuses
// it otherwise), so no other account can reach the socket at all; the socket
// itself is chmod 0600 as a second lock. A Unix socket has no network side.

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// maxSocketPath stays under the smallest sun_path in use (104 bytes on macOS).
const maxSocketPath = 100

func endpointFor(dir, key string) string { return filepath.Join(dir, key+".sock") }

func removeEndpointFile(dir, key string) { _ = os.Remove(endpointFor(dir, key)) }

// checkDirOwner refuses a register folder that belongs to another account.
func checkDirOwner(path string) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if sys, ok := st.Sys().(*syscall.Stat_t); ok && int(sys.Uid) != os.Getuid() {
		return fmt.Errorf("the register folder %s belongs to another account", path)
	}
	return nil
}

// pidAlive: signal 0 checks existence without sending anything. EPERM means
// the process exists under another account.
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

type unixListener struct{ l net.Listener }

func listen(endpoint string) (listener, error) {
	if len(endpoint) > maxSocketPath {
		return nil, fmt.Errorf("the socket path %s is %d characters, too long for this system", endpoint, len(endpoint))
	}
	// The name carries this process's own number, so anything already there is
	// left over from an earlier process that had it.
	if err := os.Remove(endpoint); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	l, err := net.Listen("unix", endpoint)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(endpoint, 0o600); err != nil {
		_ = l.Close()
		return nil, err
	}
	return &unixListener{l: l}, nil
}

func (u *unixListener) Accept() (conn, error) {
	c, err := u.l.Accept()
	if err != nil {
		if errors.Is(err, net.ErrClosed) {
			return nil, errClosed
		}
		return nil, err
	}
	return unixConn{c}, nil
}

func (u *unixListener) Close() error { return u.l.Close() }

func (u *unixListener) free() {}

type unixConn struct{ net.Conn }

func (unixConn) PeerPID() int { return 0 }

func dial(endpoint string, deadline time.Time) (conn, error) {
	c, err := net.DialTimeout("unix", endpoint, time.Until(deadline))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOENT) || errors.Is(err, syscall.ECONNREFUSED) {
			return nil, errNotRunning
		}
		return nil, err
	}
	return unixConn{c}, nil
}
