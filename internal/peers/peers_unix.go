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
	"crypto/sha256"
	"encoding/hex"
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

// endpointFor: the socket sits beside the register entry when that path fits. A long state folder (a long
// TMPDIR or XDG_STATE_HOME) would push it past sun_path (108 bytes on Linux, 104 on macOS) and the peers feature
// silently did not start, so then the socket goes to a short private folder instead
// ($XDG_RUNTIME_DIR, else /tmp/gorilla-opencode-<uid>), named by a hash of the register folder. The result depends
// only on (dir, key), so the register's endpoint check and its clean-up find the same path.
func endpointFor(dir, key string) string {
	natural := filepath.Join(dir, key+".sock")
	if len(natural) <= maxSocketPath {
		return natural
	}
	sum := sha256.Sum256([]byte(dir))
	return filepath.Join(shortSocketDir(), hex.EncodeToString(sum[:5])+"-"+key+".sock")
}

// shortSocketDir is a short folder for sockets whose natural path is too long.
func shortSocketDir() string {
	if x := os.Getenv("XDG_RUNTIME_DIR"); x != "" && filepath.IsAbs(x) && len(x) <= 40 {
		return filepath.Join(x, "gorilla-opencode")
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("gorilla-opencode-%d", os.Getuid()))
}

// ensureSocketDir makes the socket's folder when it is not the register folder: 0700, a real folder (never a
// symlink), owned by this account. /tmp is shared, so another account's folder of that name is refused.
func ensureSocketDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	st, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return fmt.Errorf("the socket folder %s is not a folder", dir)
	}
	if sys, ok := st.Sys().(*syscall.Stat_t); ok && int(sys.Uid) != os.Getuid() {
		return fmt.Errorf("the socket folder %s belongs to another account", dir)
	}
	if st.Mode().Perm() != 0o700 {
		if err := os.Chmod(dir, 0o700); err != nil {
			return err
		}
	}
	return nil
}

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
	if err := ensureSocketDir(filepath.Dir(endpoint)); err != nil {
		return nil, err
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
