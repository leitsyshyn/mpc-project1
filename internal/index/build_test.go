package index

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"runtime"
	"strings"
	"testing"
)

var sample = [][]byte{
	[]byte("The cat sat on the mat. The CAT!"),
	[]byte("A dog and the cat."),
}

const sampleIndex = "and\t1\tdoc1:1\n" +
	"cat\t3\tdoc0:2,doc1:1\n" +
	"dog\t1\tdoc1:1\n" +
	"mat\t1\tdoc0:1\n" +
	"sat\t1\tdoc0:1\n" +
	"the\t4\tdoc0:3,doc1:1\n"

// zipfCorpus generates documents of varying length whose word frequencies
// follow Zipf's law, as in natural language, with mixed case, Cyrillic words
// and assorted separators.
func zipfCorpus(docs int) [][]byte {
	r := rand.New(rand.NewPCG(1, 2))
	zipf := rand.NewZipf(r, 1.1, 1, 20_000)
	seps := []string{" ", " ", " ", ", ", ". ", "!\n", " — ", " a "}
	texts := make([][]byte, docs)
	for d := range texts {
		var b bytes.Buffer
		for range 100 + r.IntN(3000) {
			word := fmt.Sprintf("w%dq", zipf.Uint64())
			if r.IntN(7) == 0 {
				word = "слово" + word
			}
			if r.IntN(10) == 0 {
				word = strings.ToUpper(word)
			}
			b.WriteString(word)
			b.WriteString(seps[r.IntN(len(seps))])
		}
		texts[d] = b.Bytes()
	}
	return texts
}

func build(t *testing.T, version string, texts [][]byte, o Options) []Index {
	t.Helper()
	b, err := Version(version)
	if err != nil {
		t.Fatal(err)
	}
	return b(texts, o)
}

func TestBuildV0(t *testing.T) {
	if got := write(t, build(t, "v0", sample, Options{})...); got != sampleIndex {
		t.Errorf("v0 index =\n%s\nwant\n%s", got, sampleIndex)
	}
}

func TestUnknownVersion(t *testing.T) {
	if _, err := Version("v9"); err == nil {
		t.Error("Version(v9) succeeded, want error")
	}
}

// correct lists the versions whose output must match V0 byte for byte.
var correct = []string{}

func TestCorrectVersionsMatchV0(t *testing.T) {
	texts := zipfCorpus(60)
	want := write(t, build(t, "v0", texts, Options{})...)
	for _, v := range correct {
		for _, p := range []int{1, 2, 3, 8, 100} { // more threads than documents too
			t.Run(fmt.Sprintf("%s/p=%d", v, p), func(t *testing.T) {
				if got := write(t, build(t, v, texts, Options{Threads: p})...); got != want {
					t.Errorf("output differs from v0")
				}
			})
		}
	}
}

func TestV1SingleThreadIsCorrect(t *testing.T) {
	texts := zipfCorpus(20)
	want := write(t, build(t, "v0", texts, Options{})...)
	if got := write(t, build(t, "v1", texts, Options{Threads: 1})...); got != want {
		t.Error("v1 with one thread differs from v0")
	}
}

// TestV1OnlyLosesUpdates checks that races in V1 can lose updates but never
// invent words or documents, nor count more occurrences than there are.
func TestV1OnlyLosesUpdates(t *testing.T) {
	if raceEnabled {
		t.Skip("v1 races by design")
	}
	texts := zipfCorpus(60)
	want := build(t, "v0", texts, Options{})[0]
	for word, e := range build(t, "v1", texts, Options{Threads: 8})[0] {
		w := want[word]
		if w == nil {
			t.Fatalf("v1 invented word %q", word)
		}
		if e.Total > w.Total {
			t.Errorf("%q: total %d > %d", word, e.Total, w.Total)
		}
		for doc, n := range e.Docs {
			if n > w.Docs[doc] {
				t.Errorf("%q in doc %d: %d > %d", word, doc, n, w.Docs[doc])
			}
		}
	}
}

func TestV1LosesUpdates(t *testing.T) {
	if raceEnabled {
		t.Skip("v1 races by design")
	}
	if runtime.NumCPU() < 2 {
		t.Skip("needs parallel hardware")
	}
	// Every thread hammers the total counter of the same word.
	texts := make([][]byte, 32)
	for i := range texts {
		texts[i] = bytes.Repeat([]byte("the "), 50_000)
	}
	want := len(texts) * 50_000
	for range 5 { // the loss is not deterministic, so allow a few attempts
		if got := Occurrences(build(t, "v1", texts, Options{Threads: 8})...); got < want {
			t.Logf("lost %d of %d updates", want-got, want)
			return
		}
	}
	t.Errorf("v1 lost no updates in 5 runs")
}

func TestOccurrences(t *testing.T) {
	a, b := Index{}, Index{}
	a.Add([]byte("cat"), 0)
	a.Add([]byte("cat"), 1)
	b.Add([]byte("dog"), 2)
	if got := Occurrences(a, b); got != 3 {
		t.Errorf("Occurrences = %d, want 3", got)
	}
}
