package service

import (
	"strings"
	"unicode"
)

// stripFormatRunes removes Unicode format chars (Cf: word joiner, zero-width
// marks, BOM, soft hyphen) that copy-pasted names carry and that silently
// break exact-match username/email lookups.
func stripFormatRunes(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, s)
}
