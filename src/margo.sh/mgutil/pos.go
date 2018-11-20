package mgutil

import (
	"unicode/utf8"
)

// RepositionLeft moves pos left-wards through src until cond returns false
func RepositionLeft(src []byte, pos int, cond func(rune) bool) int {
	for 0 <= pos && pos < len(src) {
		r, n := utf8.DecodeLastRune(src[:pos])
		if n < 1 || !cond(r) {
			break
		}
		pos -= n
	}
	return pos
}

// RepositionRight moves pos right-wards through src until cond returns false
func RepositionRight(src []byte, pos int, cond func(rune) bool) int {
	for 0 <= pos && pos < len(src) {
		r, n := utf8.DecodeRune(src[pos:])
		if n < 1 || !cond(r) {
			break
		}
		pos += n
	}
	return pos
}
