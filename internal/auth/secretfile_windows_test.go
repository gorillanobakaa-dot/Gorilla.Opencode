package auth

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeLogin is a sign-in with recognizable, made-up values.
func fakeLogin() *ChatGPTCreds {
	return &ChatGPTCreds{
		AccessToken:  "made-up-access-value-for-the-dpapi-test",
		RefreshToken: "made-up-refresh-value-for-the-dpapi-test",
		TokenType:    "Bearer",
		Expiry:       time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC),
		Email:        "nobody@example.invalid",
	}
}

// useTempConfig points the sign-in files at a folder the test owns.
func useTempConfig(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
}

// TestDPAPIRoundTrip: sealed bytes do not contain the secret, and the same
// user opens them again.
func TestDPAPIRoundTrip(t *testing.T) {
	plain := []byte(`{"refresh_token":"made-up-refresh-value-for-the-dpapi-test"}`)
	blob, ok := seal(plain)
	if !ok {
		t.Fatal("CryptProtectData failed for the current user")
	}
	if bytes.Contains(blob, []byte("made-up-refresh")) {
		t.Fatal("the sealed blob holds the secret in clear")
	}
	got, err := unseal(blob)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("unseal = %q, %v", got, err)
	}
	blob[len(blob)/2] ^= 0xFF
	if _, err := unseal(blob); err == nil {
		t.Error("a damaged blob was opened")
	}
}

// TestDPAPIReadsAnOldPlainFile: a sign-in written by an older version
// (plain JSON) still loads: nobody is signed out by the upgrade.
func TestDPAPIReadsAnOldPlainFile(t *testing.T) {
	useTempConfig(t)
	path := ChatGPTCredsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	old := `{"access_token":"made-up-access-value-for-the-dpapi-test","refresh_token":"made-up-refresh-value-for-the-dpapi-test","token_type":"Bearer","expiry":"2030-01-02T03:04:05Z"}`
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := LoadChatGPTCreds()
	if err != nil || c == nil || c.RefreshToken != fakeLogin().RefreshToken {
		t.Fatalf("old plain file not read: %+v, %v", c, err)
	}
}

// TestDPAPIMigratesOnNextWrite: the next save seals the file in place (same
// name, same folder), the secret is gone from the bytes on disk, and the
// login reads back whole.
func TestDPAPIMigratesOnNextWrite(t *testing.T) {
	useTempConfig(t)
	path := ChatGPTCredsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"refresh_token":"made-up-refresh-value-for-the-dpapi-test"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := LoadChatGPTCreds()
	if err != nil {
		t.Fatal(err)
	}
	*c = *fakeLogin()
	if err := c.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(raw), sealedPrefix) {
		t.Fatalf("the file was not sealed on write: %q", raw[:min(len(raw), 40)])
	}
	if bytes.Contains(raw, []byte("made-up")) || bytes.Contains(raw, []byte("nobody@")) {
		t.Fatal("the sealed file still holds the sign-in in clear")
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("the temporary file was left behind: %v", err)
	}
	back, err := LoadChatGPTCreds()
	if err != nil || back == nil || *back != *fakeLogin() {
		t.Fatalf("read back %+v, %v; want %+v", back, err, fakeLogin())
	}
	// The other two sign-ins use the same path through the code.
	g := &GeminiCreds{RefreshToken: "made-up-refresh-value-for-the-dpapi-test"}
	if err := g.Save(); err != nil {
		t.Fatal(err)
	}
	if raw, _ := os.ReadFile(CredsPath()); !strings.HasPrefix(string(raw), sealedPrefix) {
		t.Error("the Gemini sign-in was not sealed")
	}
	a := &AntigravityCreds{RefreshToken: "made-up-refresh-value-for-the-dpapi-test"}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	if got, err := LoadAntigravityCreds(); err != nil || got.RefreshToken != a.RefreshToken {
		t.Errorf("Antigravity read back %+v, %v", got, err)
	}
}

// TestDPAPIForeignFileIsKept: a file sealed elsewhere (another user, another
// computer) is reported with what to do, and is not deleted or overwritten.
func TestDPAPIForeignFileIsKept(t *testing.T) {
	useTempConfig(t)
	path := ChatGPTCredsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	foreign := sealedPrefix + "AQAAANCMnd8BFdERjHoAwE/Cl+sBAAAA\n"
	if err := os.WriteFile(path, []byte(foreign), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadChatGPTCreds(); err == nil || !strings.Contains(err.Error(), "sign in again") {
		t.Fatalf("want an error that says to sign in again, got %v", err)
	}
	if raw, _ := os.ReadFile(path); string(raw) != foreign {
		t.Error("the unreadable file was changed")
	}
}
