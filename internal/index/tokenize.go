package index

import (
	"iter"
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
