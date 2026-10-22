package index

import (
	"slices"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

func collect(text string) []string {
	var got []string
	for w := range Words([]byte(text)) {
		got = append(got, string(w))
	}
	return got
}

func TestWords(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{"empty", "", nil},
		{"only separators", " \t\n.,;!?-—«»", nil},
		{"lowercases", "Hello WORLD", []string{"hello", "world"}},
		{"punctuation separates", "foo,bar.baz!qux", []string{"foo", "bar", "baz", "qux"}},
		{"underscore and hyphen separate", "snake_case kebab-case", []string{"snake", "case", "kebab", "case"}},
		{"apostrophe separates", "don't м'ята", []string{"don", "ята"}},
		{"drops tokens shorter than three", "a an the of all", []string{"the", "all"}},
		{"digits are token characters", "12 123 4567 ab12", []string{"123", "4567", "ab12"}},
		{"last token at end of input", "abc def", []string{"abc", "def"}},
		{"cyrillic lowercased", "ПРИВІТ Світ", []string{"привіт", "світ"}},
		// Length is counted in characters, not bytes: "їх" is 4 bytes but 2 letters.
		{"length counts runes", "їх він", []string{"він"}},
		{"non-latin scripts", "Ελληνικά 日本語", []string{"ελληνικά", "日本語"}},
		// Lowercasing may change the byte length of a rune (Kelvin sign is 3 bytes, 'k' is 1).
		{"lowercase changes byte length", "KELVIN", []string{"kelvin"}},
		{"invalid utf-8 separates", "abc\xffdef", []string{"abc", "def"}},
		// A combining mark is neither a letter nor a digit.
		{"combining mark separates", "cafés", []string{"cafe"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := collect(tt.text); !slices.Equal(got, tt.want) {
				t.Errorf("Words(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestWordsStopsEarly(t *testing.T) {
	var got []string
	for w := range Words([]byte("one two three four")) {
		got = append(got, string(w))
		if len(got) == 2 {
			break
		}
	}
	if want := []string{"one", "two"}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func BenchmarkWords(b *testing.B) {
	text := []byte("The quick brown Fox jumps over the lazy dog. Швидка руда лисиця! ")
	b.SetBytes(int64(len(text)))
	for b.Loop() {
		for range Words(text) {
		}
	}
}

// naiveWords is a straightforward reference implementation of the normalization rules.
func naiveWords(text string) []string {
	var words []string
	for _, f := range strings.FieldsFunc(text, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if utf8.RuneCountInString(f) >= 3 {
			words = append(words, strings.Map(unicode.ToLower, f))
		}
	}
	return words
}

func FuzzWords(f *testing.F) {
	for _, s := range []string{"Hello, World!", "ПРИВІТ їх", "\u212AELVIN", "abc\xffdef", "cafe\u0301s", "a1_b2-c3d4"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, text string) {
		if got, want := collect(text), naiveWords(text); !slices.Equal(got, want) {
			t.Errorf("Words(%q) = %q, want %q", text, got, want)
		}
	})
}
