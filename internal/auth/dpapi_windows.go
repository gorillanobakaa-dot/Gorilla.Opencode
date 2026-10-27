package auth

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// dpapiEntropy ties a sealed file to this program: another program running
// as the same user cannot open it with a plain CryptUnprotectData call.
var dpapiEntropy = []byte("gorilla-opencode sign-in v1")

func blobOf(b []byte) *windows.DataBlob {
	if len(b) == 0 {
		return &windows.DataBlob{}
	}
	return &windows.DataBlob{Size: uint32(len(b)), Data: &b[0]}
}

// takeBlob copies a DPAPI output buffer and frees it.
func takeBlob(b windows.DataBlob) []byte {
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(b.Data)))
	return append([]byte(nil), unsafe.Slice(b.Data, b.Size)...)
}

// seal encrypts for the current Windows user (CryptProtectData, no UI).
func seal(plain []byte) ([]byte, bool) {
	var out windows.DataBlob
	if err := windows.CryptProtectData(blobOf(plain), nil, blobOf(dpapiEntropy), 0, nil,
		windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, false
	}
	return takeBlob(out), true
}

// unseal decrypts what seal wrote, as the same user on the same computer.
func unseal(blob []byte) ([]byte, error) {
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(blobOf(blob), nil, blobOf(dpapiEntropy), 0, nil,
		windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, err
	}
	return takeBlob(out), nil
}
