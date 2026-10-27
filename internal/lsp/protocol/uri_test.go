package protocol

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestURIFromPathWindows checks the file URIs a Windows path becomes:
// three slashes, forward slashes, upper-case drive, escaped spaces and
// non-ASCII, and the RFC 8089 host form for UNC. Before the fix the client
// sent "file://C:\x" (two slashes, backslashes), which no server matches.
func TestURIFromPathWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows paths")
	}
	cases := []struct{ path, want string }{
		{`C:\Users\me\proj\main.go`, "file:///C:/Users/me/proj/main.go"},
		{`c:\x\y.go`, "file:///C:/x/y.go"},
		{`C:\My Projects\a b.go`, "file:///C:/My%20Projects/a%20b.go"},
		{`C:\Users\José\café.go`, "file:///C:/Users/Jos%C3%A9/caf%C3%A9.go"},
		{`\\server\share\dir\f.go`, "file://server/share/dir/f.go"},
	}
	for _, c := range cases {
		if got := string(URIFromPath(c.path)); got != c.want {
			t.Errorf("URIFromPath(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

// TestURIRoundTripWindows: path -> URI -> path gives the same path back,
// including after a server re-encodes the drive colon or lower-cases it.
func TestURIRoundTripWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows paths")
	}
	for _, p := range []string{
		`C:\Users\me\proj\main.go`,
		`C:\My Projects\a b.go`,
		`D:\données\été 2026\naïve.go`,
		`\\server\share\dir\f.go`,
		`\\server\share\a b\é.go`,
	} {
		uri := URIFromPath(p)
		got, err := uri.Path()
		if err != nil {
			t.Fatalf("Path(%q): %v", uri, err)
		}
		if got != p {
			t.Errorf("round trip %q -> %q -> %q", p, uri, got)
		}
		// What a server may send back for the same file.
		var parsed DocumentUri
		if err := parsed.UnmarshalText([]byte(uri)); err != nil {
			t.Fatalf("UnmarshalText(%q): %v", uri, err)
		}
		if parsed != uri {
			t.Errorf("UnmarshalText(%q) = %q, want it unchanged", uri, parsed)
		}
	}
	// VS Code style: lower-case drive and an escaped colon.
	var parsed DocumentUri
	if err := parsed.UnmarshalText([]byte("file:///c%3A/My%20Projects/a%20b.go")); err != nil {
		t.Fatal(err)
	}
	if parsed != URIFromPath(`C:\My Projects\a b.go`) {
		t.Errorf("server form %q does not match the client form %q", parsed, URIFromPath(`C:\My Projects\a b.go`))
	}
	// A server echoing a UNC URI keeps its host.
	if err := parsed.UnmarshalText([]byte("file://server/share/f.go")); err != nil {
		t.Fatal(err)
	}
	if p, _ := parsed.Path(); p != `\\server\share\f.go` {
		t.Errorf("UNC from a server = %q, want \\\\server\\share\\f.go", p)
	}
}

// TestURIPathPosix keeps the POSIX behavior the Linux and Mac builds rely on.
func TestURIPathPosix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX paths")
	}
	for _, p := range []string{"/home/me/a.go", "/home/me/a b/é.go"} {
		uri := URIFromPath(p)
		if !strings.HasPrefix(string(uri), "file:///") {
			t.Errorf("URIFromPath(%q) = %q", p, uri)
		}
		got, err := uri.Path()
		if err != nil || got != p {
			t.Errorf("round trip %q -> %q -> %q, %v", p, uri, got, err)
		}
	}
}

// TestURIPathBadURINoPanic: a bad URI is an error, never a panic.
func TestURIPathBadURINoPanic(t *testing.T) {
	for _, bad := range []DocumentUri{
		"http://example.com/x.go",
		"file:///C:/bad%zzescape.go",
		"::not a uri",
	} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Path(%q) panicked: %v", bad, r)
				}
			}()
			if _, err := bad.Path(); err == nil {
				t.Errorf("Path(%q) returned no error", bad)
			}
		}()
	}
	if p, err := DocumentUri("").Path(); p != "" || err != nil {
		t.Errorf(`Path("") = %q, %v`, p, err)
	}
	if _, err := PathFromURI("file:///C:/bad%zz"); err == nil {
		t.Error("PathFromURI accepted a bad escape")
	}
	d, err := URIFromPath(filepath.Join(t.TempDir(), "x.go")).DirPath()
	if err != nil || d == "" {
		t.Errorf("DirPath = %q, %v", d, err)
	}
}
