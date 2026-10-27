package auth

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
)

// GORILLA FIX (2026-10-27, job 09, gap #17): sign-in files encrypted for the
// current Windows user.
//
// The OAuth files were plain JSON at mode 0600, and on Windows 0600 means
// nothing: any program running as anyone who can read the profile folder
// (a backup tool, a sync client, another admin) read the refresh tokens.
// On Windows they are now sealed with DPAPI (CryptProtectData, current user):
// only this Windows account on this machine can open them, with no key for
// this program to keep. Elsewhere nothing changes.
//
// Never lose a login: a file written by an older version (plain JSON) is
// still read, and is sealed on the next write. If sealing fails, the file is
// written plain as before rather than not at all, because a refreshed token
// that is not saved can end the sign-in. The file names and the folder stay
// the same, so everything that looks for them still finds them.

// sealedPrefix starts a sealed file; the base64 of the DPAPI blob follows.
const sealedPrefix = "GORILLA-DPAPI-1\n"

// readSecretFile returns the plain contents of a sign-in file, sealed or not.
// A missing file is returned as os.ReadFile returns it.
func readSecretFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if !bytes.HasPrefix(data, []byte(sealedPrefix)) {
		return data, nil // written by an older version, or not on Windows
	}
	blob, err := base64.StdEncoding.DecodeString(string(bytes.TrimSpace(data[len(sealedPrefix):])))
	if err != nil {
		return nil, fmt.Errorf("%s is damaged (%v); sign in again", path, err)
	}
	plain, err := unseal(blob)
	if err != nil {
		return nil, fmt.Errorf("%s is sealed for another Windows user or another computer, so it cannot be "+
			"opened here (%v); sign in again", path, err)
	}
	return plain, nil
}

// writeSecretFile writes a sign-in file atomically, sealed where the system
// can seal it.
func writeSecretFile(path string, plain []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	out := plain
	if blob, ok := seal(plain); ok {
		out = append([]byte(sealedPrefix), []byte(base64.StdEncoding.EncodeToString(blob)+"\n")...)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
