package index

import (
	"encoding/binary"
	"iter"
	"math/bits"
	"unicode"
	"unicode/utf8"
)

// TokenizeFunc splits a text into normalized words. The yielded slice is only
// valid until the next iteration.
type TokenizeFunc func(text []byte) iter.Seq[[]byte]

const minLen = 3

var tokenByte = func() (t [utf8.RuneSelf]byte) {
	for c := range byte(utf8.RuneSelf) {
		switch {
		case 'a' <= c && c <= 'z', '0' <= c && c <= '9':
			t[c] = c
		case 'A' <= c && c <= 'Z':
			t[c] = c + 'a' - 'A'
		}
	}
	return t
}()

// Words yields the maximal runs of letters and digits of UTF-8 text that are
// at least three characters long, in lowercase. Any other character, including
// invalid UTF-8, is a separator.
func Words(text []byte) iter.Seq[[]byte] {
	return func(yield func([]byte) bool) {
		word := make([]byte, 0, 64)
		runes := 0
		flush := func() bool {
			ok := runes < minLen || yield(word)
			word, runes = word[:0], 0
			return ok
		}
		for i := 0; i < len(text); {
			if c := text[i]; c < utf8.RuneSelf {
				i++
				if l := tokenByte[c]; l != 0 {
					word, runes = append(word, l), runes+1
				} else if !flush() {
					return
				}
				continue
			}
			r, size := utf8.DecodeRune(text[i:])
			i += size
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				word, runes = utf8.AppendRune(word, unicode.ToLower(r)), runes+1
			} else if !flush() {
				return
			}
		}
		flush()
	}
}

// FastWords yields the same words as Words. It classifies ASCII eight bytes at
// a time within a uint64 (SWAR) and yields words that are already lowercase
// ASCII as subslices of text without copying them.
func FastWords(text []byte) iter.Seq[[]byte] {
	return func(yield func([]byte) bool) {
		word := make([]byte, 0, 64)
		for i := 0; i < len(text); {
			start := i
			end, lower := scanASCII(text, i)
			if end == len(text) || text[end] < utf8.RuneSelf {
				if end-start >= minLen {
					tok := text[start:end]
					if !lower {
						word = appendLower(word[:0], tok)
						tok = word
					}
					if !yield(tok) {
						return
					}
				}
				i = end + 1
				continue
			}
			var runes int
			word, runes, i = appendRunes(appendLower(word[:0], text[start:end]), text, end)
			if end-start+runes >= minLen && !yield(word) {
				return
			}
			if i < len(text) {
				_, size := utf8.DecodeRune(text[i:])
				i += size
			}
		}
	}
}

func scanASCII(text []byte, i int) (end int, lower bool) {
	lower = true
	for i+8 <= len(text) {
		w := binary.LittleEndian.Uint64(text[i:])
		if w&high != 0 {
			break
		}
		tok, upper := classify(w)
		if stop := ^tok & high; stop != 0 {
			n := bits.TrailingZeros64(stop) / 8
			return i + n, lower && upper&(1<<(8*n)-1) == 0
		}
		lower = lower && upper == 0
		i += 8
	}
	for i < len(text) && text[i] < utf8.RuneSelf && tokenByte[text[i]] != 0 {
		lower = lower && tokenByte[text[i]] == text[i]
		i++
	}
	return i, lower
}

func appendRunes(word, text []byte, i int) (_ []byte, runes, end int) {
	for i < len(text) {
		r, size := utf8.DecodeRune(text[i:])
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			break
		}
		word, runes, i = utf8.AppendRune(word, unicode.ToLower(r)), runes+1, i+size
	}
	return word, runes, i
}

const (
	ones = 0x0101010101010101
	high = 0x8080808080808080
)

func classify(w uint64) (tok, upper uint64) {
	folded := w | 0x20*ones
	letter := atLeast(folded, 'a') &^ atLeast(folded, 'z'+1)
	digit := atLeast(w, '0') &^ atLeast(w, '9'+1)
	upper = atLeast(w, 'A') &^ atLeast(w, 'Z'+1)
	return letter | digit, upper
}

func atLeast(w uint64, c byte) uint64 { return (w + (0x80-uint64(c))*ones) & high }

func appendLower(dst, src []byte) []byte {
	for _, c := range src {
		dst = append(dst, tokenByte[c])
	}
	return dst
}
