package index

import (
	"slices"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

func collect(text string) []string { return collectWith(Words, text) }

func collectWith(words TokenizeFunc, text string) []string {
	var got []string
	for w := range words([]byte(text)) {
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
		{"length counts characters not bytes", "їх він", []string{"він"}},
		{"non-latin scripts", "Ελληνικά 日本語", []string{"ελληνικά", "日本語"}},
		{"lowercasing changes byte length", "\u212AELVIN", []string{"kelvin"}},
		{"invalid utf-8 separates", "abc\xffdef", []string{"abc", "def"}},
		{"combining mark separates", "cafe\u0301s", []string{"cafe"}},
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
	for name, words := range tokenizers {
		var got []string
		for w := range words([]byte("one Two три four")) {
			got = append(got, string(w))
			if len(got) == 3 {
				break
			}
		}
		if want := []string{"one", "two", "три"}; !slices.Equal(got, want) {
			t.Errorf("%s: got %q, want %q", name, got, want)
		}
	}
}

func BenchmarkWords(b *testing.B) {
	text := []byte("The quick brown Fox jumps over the lazy dog. Швидка руда лисиця! ")
	for name, words := range tokenizers {
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(text)))
			for b.Loop() {
				for range words(text) {
				}
			}
		})
	}
}

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
		want := naiveWords(text)
		for name, words := range tokenizers {
			if got := collectWith(words, text); !slices.Equal(got, want) {
				t.Errorf("%s(%q) = %q, want %q", name, text, got, want)
			}
		}
	})
}

func BenchmarkWordsCorpus(b *testing.B) {
	texts, err := Load("../../data")
	if err != nil || len(texts) == 0 {
		b.Skip("no corpus in data/")
	}
	text := slices.Concat(texts[:min(20, len(texts))]...)
	for name, words := range tokenizers {
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(text)))
			for b.Loop() {
				for range words(text) {
				}
			}
		})
	}
}

func TestClassify(t *testing.T) {
	for c := range byte(utf8.RuneSelf) {
		for pos := range 8 {
			w := uint64(0x41_2e_61_20_39_5f_7a_2d) &^ (0xff << (8 * pos))
			w |= uint64(c) << (8 * pos)
			tok, upper := classify(w)
			bit := uint64(0x80) << (8 * pos)
			if got, want := tok&bit != 0, tokenByte[c] != 0; got != want {
				t.Errorf("classify: byte %q at %d: token = %v, want %v", c, pos, got, want)
			}
			if got, want := upper&bit != 0, 'A' <= c && c <= 'Z'; got != want {
				t.Errorf("classify: byte %q at %d: upper = %v, want %v", c, pos, got, want)
			}
		}
	}
}
