package tools

// Gap #6: view refused UTF-16 and ANSI text as binary. PowerShell 5.1's `>`
// writes UTF-16LE with a byte-order mark, as do .reg exports and logs.
// Test names end in a lowercase token (_utf16, _utf) because `go test -run`
// compiles each top-level alternative of the goal's pattern on its own.

import (
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const encSample = "Windows Registry Editor Version 5.00\r\n\r\n[HKEY_CURRENT_USER\\Software\\Café]\r\n\"Name\"=\"naïve ünïcode — 日本\"\r\n"

func utf16Bytes(s string, bigEndian bool) []byte {
	u := utf16.Encode([]rune(s))
	out := []byte{0xFF, 0xFE}
	if bigEndian {
		out = []byte{0xFE, 0xFF}
	}
	for _, c := range u {
		if bigEndian {
			out = append(out, byte(c>>8), byte(c))
		} else {
			out = append(out, byte(c), byte(c>>8))
		}
	}
	return out
}

func viewBytes(t *testing.T, name string, data []byte) ToolResponse {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(p, data, 0o644))
	return viewOf(t, viewInput(t, p))
}

func TestViewDecodesUTF16LE_utf16(t *testing.T) {
	r := viewBytes(t, "export.reg", utf16Bytes(encSample, false))
	require.False(t, r.IsError, r.Content)
	assert.Contains(t, r.Content, "Decoded from UTF-16LE")
	assert.Contains(t, r.Content, `"Name"="naïve ünïcode — 日本"`)
	assert.Contains(t, r.Content, `[HKEY_CURRENT_USER\Software\Café]`)
	assert.NotContains(t, r.Content, "\r", "CRLF must not leave a stray CR in the lines shown")
}

func TestViewDecodesUTF16BE_utf16(t *testing.T) {
	r := viewBytes(t, "be.txt", utf16Bytes(encSample, true))
	require.False(t, r.IsError, r.Content)
	assert.Contains(t, r.Content, "Decoded from UTF-16BE")
	assert.Contains(t, r.Content, "naïve ünïcode — 日本")
}

func TestViewDecodesUTF8BOM_utf(t *testing.T) {
	r := viewBytes(t, "bom.txt", append([]byte{0xEF, 0xBB, 0xBF}, encSample...))
	require.False(t, r.IsError, r.Content)
	assert.Contains(t, r.Content, "Decoded from UTF-8 with byte-order mark")
	assert.NotContains(t, r.Content, "\ufeff", "the BOM must not be shown as text")
	assert.Contains(t, r.Content, "naïve")
}

// Windows-1252: é = E9, € = 80, ’ = 92, — = 97.
func TestViewDecodesANSIWindows1252_utf(t *testing.T) {
	r := viewBytes(t, "ansi.txt", []byte("Prix: 5 \x80, caf\xe9 l\x92ami \x97 fin\r\nligne 2\r\n"))
	require.False(t, r.IsError, r.Content)
	assert.Contains(t, r.Content, "Decoded from Windows-1252")
	assert.Contains(t, r.Content, "Prix: 5 €, café l’ami — fin")
}

// Plain UTF-8 is shown without a note, and real binaries are still refused.
func TestViewPlainUTF8AndBinaryUnchanged_utf(t *testing.T) {
	r := viewBytes(t, "plain.txt", []byte("héllo\n"))
	require.False(t, r.IsError, r.Content)
	assert.NotContains(t, r.Content, "Decoded from")
	assert.Contains(t, r.Content, "héllo")

	bin := make([]byte, 64)
	for i := range bin {
		bin[i] = byte(i * 7)
	}
	r = viewBytes(t, "blob.bin", bin)
	assert.True(t, r.IsError, "a binary must still be refused: %s", r.Content)
	assert.Contains(t, r.Content, "binary file")
}

func TestDetectTextEncodingCutRune_utf(t *testing.T) {
	head := []byte("abc caf\xc3") // é cut at the end of the chunk read
	enc, ok := detectTextEncoding(head, true)
	assert.True(t, ok)
	assert.Equal(t, "", enc, "a rune cut by the read size is still UTF-8")
	enc, ok = detectTextEncoding([]byte("caf\xe9 au lait"), false)
	assert.True(t, ok)
	assert.Equal(t, encWin1252, enc)
	_, ok = detectTextEncoding([]byte("x\x81y\x8dz"), false)
	assert.False(t, ok, "bytes Windows-1252 leaves undefined are not text")
}
