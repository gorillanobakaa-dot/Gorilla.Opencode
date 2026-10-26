package tools

import (
	"bytes"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Text encodings view decodes instead of refusing (gap #6). Windows writes
// all of these: PowerShell 5.1's `>` and Out-File write UTF-16LE with a
// byte-order mark, as do .reg exports and many logs; Notepad and older
// tools write UTF-8 with a BOM or the ANSI code page (Windows-1252 on
// Western machines). Only a BOM decides UTF-16, never a guess.
const (
	encUTF8BOM  = "UTF-8 with byte-order mark"
	encUTF16LE  = "UTF-16LE (byte-order mark)"
	encUTF16BE  = "UTF-16BE (byte-order mark)"
	encWin1252  = "Windows-1252 (ANSI)"
	encUTF8Text = ""
)

// detectTextEncoding names the encoding of a file that starts with head, or
// "" for plain UTF-8. ok is false when head is not text in any encoding
// view decodes.
// more says the file goes on past head, so a rune may be cut at its end.
func detectTextEncoding(head []byte, more bool) (enc string, ok bool) {
	switch {
	case bytes.HasPrefix(head, []byte{0xEF, 0xBB, 0xBF}):
		return encUTF8BOM, true
	case bytes.HasPrefix(head, []byte{0xFF, 0xFE}):
		return encUTF16LE, true
	case bytes.HasPrefix(head, []byte{0xFE, 0xFF}):
		return encUTF16BE, true
	}
	if bytes.IndexByte(head, 0) >= 0 {
		return "", false
	}
	if utf8.Valid(head) || (more && validUTF8Prefix(head)) {
		return encUTF8Text, true
	}
	if looksLikeWin1252(head) {
		return encWin1252, true
	}
	return "", false
}

// validUTF8Prefix: head is valid UTF-8 except for a rune cut at the end of
// the chunk that was read.
func validUTF8Prefix(head []byte) bool {
	for i := 1; i <= utf8.UTFMax && i < len(head); i++ {
		if utf8.Valid(head[:len(head)-i]) {
			return !utf8.FullRune(head[len(head)-i:])
		}
	}
	return false
}

// looksLikeWin1252 is narrow on purpose: a binary file shown as text costs
// the conversation more than a text file refused. No byte Windows-1252
// leaves undefined, and almost no control characters.
func looksLikeWin1252(b []byte) bool {
	controls := 0
	for _, c := range b {
		switch c {
		case 0x81, 0x8D, 0x8F, 0x90, 0x9D:
			return false
		case '\t', '\n', '\r', '\f', 0x1b:
		default:
			if c < 0x20 || c == 0x7f {
				controls++
			}
		}
	}
	return controls*100 <= len(b)
}

// win1252High maps 0x80-0x9F; every other byte is the Latin-1 code point.
var win1252High = [32]rune{
	'€', 0xFFFD, '‚', 'ƒ', '„', '…', '†', '‡', 'ˆ', '‰', 'Š', '‹', 'Œ', 0xFFFD, 'Ž', 0xFFFD,
	0xFFFD, '‘', '’', '“', '”', '•', '–', '—', '˜', '™', 'š', '›', 'œ', 0xFFFD, 'ž', 'Ÿ',
}

// decodeText turns data in enc into UTF-8.
func decodeText(data []byte, enc string) string {
	switch enc {
	case encUTF8BOM:
		return string(bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF}))
	case encUTF16LE, encUTF16BE:
		data = data[2:]
		u := make([]uint16, len(data)/2)
		for i := range u {
			if enc == encUTF16LE {
				u[i] = uint16(data[2*i]) | uint16(data[2*i+1])<<8
			} else {
				u[i] = uint16(data[2*i])<<8 | uint16(data[2*i+1])
			}
		}
		return string(utf16.Decode(u))
	case encWin1252:
		var sb strings.Builder
		sb.Grow(len(data) + len(data)/8)
		for _, c := range data {
			switch {
			case c < 0x80:
				sb.WriteByte(c)
			case c < 0xA0:
				sb.WriteRune(win1252High[c-0x80])
			default:
				sb.WriteRune(rune(c))
			}
		}
		return sb.String()
	}
	return string(data)
}
