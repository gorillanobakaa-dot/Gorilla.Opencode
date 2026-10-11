package tools

import "unicode/utf8"

// cutAtRune returns at most the first n bytes of s, moved back to the start of
// a character so a multi-byte one is never split.
//
// GORILLA FIX (2026-10-11): truncations here sliced bytes (s[:n]). On text in
// Romanian, Greek, Cyrillic, CJK or anything with an emoji, the cut landed
// inside a character half the time and left an invalid UTF-8 byte at the end
// of what the model, the transcript and the findings file received.
func cutAtRune(s string, n int) string {
	if n >= len(s) {
		return s
	}
	if n <= 0 {
		return ""
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
