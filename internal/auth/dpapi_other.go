//go:build !windows

package auth

import "errors"

// Outside Windows sign-in files stay plain JSON at mode 0600, which the
// system enforces there (secretfile.go).

func seal([]byte) ([]byte, bool) { return nil, false }

func unseal([]byte) ([]byte, error) {
	return nil, errors.New("this sign-in file was sealed with Windows DPAPI and can only be opened on that Windows account")
}
