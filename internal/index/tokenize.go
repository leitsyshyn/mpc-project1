package index

import (
	"encoding/binary"
	"iter"
	"math/bits"
	"unicode"
	"unicode/utf8"
)

// minLen is the shortest kept token, in characters.
const minLen = 3

// ascii maps an ASCII letter or digit to its lowercase form and any other ASCII byte to 0.
var ascii = func() (t [utf8.RuneSelf]byte) {
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

// Words yields the normalized tokens of UTF-8 text: maximal runs of letters and
// digits, lowercased, at least minLen characters long. Any other character,
// including invalid UTF-8, is a separator. The yielded slice is only valid until
// the next iteration.
func Words(text []byte) iter.Seq[[]byte] {
	return func(yield func([]byte) bool) {
		word := make([]byte, 0, 64)
		n := 0 // characters in word
		flush := func() bool {
			ok := n < minLen || yield(word)
			word, n = word[:0], 0
			return ok
		}
		for i := 0; i < len(text); {
			if c := text[i]; c < utf8.RuneSelf { // fast path for ASCII
				i++
				if l := ascii[c]; l != 0 {
					word, n = append(word, l), n+1
				} else if !flush() {
					return
				}
				continue
			}
			r, size := utf8.DecodeRune(text[i:])
			i += size
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				word, n = utf8.AppendRune(word, unicode.ToLower(r)), n+1
			} else if !flush() {
				return
			}
		}
		flush()
	}
}

// FastWords yields the same tokens as Words, with less work per byte. It scans
// ASCII 8 bytes at a time, classifying all of them at once with arithmetic on
// a uint64 (SWAR, SIMD within a register), and yields a token that is already
// lowercase ASCII, the common case in English text, as a subslice of text
// instead of copying it. Tokens with uppercase or non-ASCII characters are
// copied into a buffer as in Words.
func FastWords(text []byte) iter.Seq[[]byte] {
	return func(yield func([]byte) bool) {
		word := make([]byte, 0, 64)
		for i := 0; i < len(text); {
			// Scan the ASCII part of the token starting at i, possibly empty.
			start, lower := i, true
			for i+8 <= len(text) {
				w := binary.LittleEndian.Uint64(text[i:])
				if w&high != 0 {
					break
				}
				tok, upper := classify(w)
				if stop := ^tok & high; stop != 0 {
					n := bits.TrailingZeros64(stop) / 8
					lower = lower && upper&(1<<(8*n)-1) == 0
					i += n
					break
				}
				lower = lower && upper == 0
				i += 8
			}
			for i < len(text) && text[i] < utf8.RuneSelf && ascii[text[i]] != 0 {
				lower = lower && ascii[text[i]] == text[i]
				i++
			}

			if i == len(text) || text[i] < utf8.RuneSelf { // an ASCII token, possibly empty
				if i-start >= minLen {
					tok := text[start:i]
					if !lower {
						word = lowerASCII(word[:0], tok)
						tok = word
					}
					if !yield(tok) {
						return
					}
				}
				i++ // skip the ASCII separator
				continue
			}

			// A non-ASCII character: finish the token rune by rune.
			word = lowerASCII(word[:0], text[start:i])
			n := i - start
			for i < len(text) {
				if c := text[i]; c < utf8.RuneSelf {
					if ascii[c] == 0 {
						break
					}
					word, n, i = append(word, ascii[c]), n+1, i+1
					continue
				}
				r, size := utf8.DecodeRune(text[i:])
				if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
					break
				}
				word, n, i = utf8.AppendRune(word, unicode.ToLower(r)), n+1, i+size
			}
			if n >= minLen && !yield(word) {
				return
			}
			if i < len(text) { // skip the separator
				_, size := utf8.DecodeRune(text[i:])
				i += size
			}
		}
	}
}

const (
	ones = 0x0101010101010101
	high = 0x8080808080808080
)

// classify returns, for the 8 ASCII bytes of w, the bit 0x80 set in every byte
// that is a letter or digit and in every byte that is an uppercase letter.
func classify(w uint64) (tok, upper uint64) {
	l := w | 0x20*ones // folds uppercase letters onto lowercase
	letter := atLeast(l, 'a') &^ atLeast(l, 'z'+1)
	digit := atLeast(w, '0') &^ atLeast(w, '9'+1)
	upper = atLeast(w, 'A') &^ atLeast(w, 'Z'+1)
	return letter | digit, upper
}

// atLeast sets the bit 0x80 in every byte of the ASCII w that is >= c. Every
// byte is at most 0x7f, so adding 0x80-c never carries into the next byte.
func atLeast(w uint64, c byte) uint64 { return (w + (0x80-uint64(c))*ones) & high }

// lowerASCII appends the lowercase form of the ASCII letters and digits in src to dst.
func lowerASCII(dst, src []byte) []byte {
	for _, c := range src {
		dst = append(dst, ascii[c])
	}
	return dst
}
