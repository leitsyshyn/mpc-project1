package index

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"runtime"
	"strings"
	"testing"
	"time"
	"unsafe"
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

func firstDiff(got, want string) string {
	g, w := strings.Split(got, "\n"), strings.Split(want, "\n")
	for i := range min(len(g), len(w)) {
		if g[i] != w[i] {
			return fmt.Sprintf("line %d = %q, want %q", i+1, g[i], w[i])
		}
	}
	return fmt.Sprintf("%d lines, want %d", len(g), len(w))
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

func TestUnknownNames(t *testing.T) {
	if _, err := Version("v9"); err == nil {
		t.Error("Version(v9) succeeded, want error")
	}
	if _, err := Tokenizer("regex"); err == nil {
		t.Error("Tokenizer(regex) succeeded, want error")
	}
}

var correctVersions = []string{"v2", "v3", "v4seq", "v4tree", "cas", "syncmap"}

func TestCorrectVersionsMatchV0(t *testing.T) {
	texts := zipfCorpus(60)
	want := write(t, build(t, "v0", texts, Options{})...)
	for _, v := range correctVersions {
		for _, p := range []int{1, 2, 3, 8, 100} {
			for _, k := range []int{1, 7, 4096} {
				t.Run(fmt.Sprintf("%s_p%d_k%d", v, p, k), func(t *testing.T) {
					if got := write(t, build(t, v, texts, Options{Threads: p, Stripes: k})...); got != want {
						t.Errorf("%s differs from v0: %s", v, firstDiff(got, want))
					}
				})
			}
		}
	}
}

func TestTokenizersGiveSameIndex(t *testing.T) {
	texts := zipfCorpus(60)
	want := write(t, build(t, "v0", texts, Options{})...)
	for _, v := range append([]string{"v0"}, correctVersions...) {
		if got := write(t, build(t, v, texts, Options{Threads: 4, Stripes: 64, Tokenize: Words})...); got != want {
			t.Errorf("%s with Words differs from v0 with FastWords: %s", v, firstDiff(got, want))
		}
	}
}

func TestV3PartsAreDisjoint(t *testing.T) {
	parts := build(t, "v3", zipfCorpus(20), Options{Threads: 4, Stripes: 16})
	if len(parts) != 16 {
		t.Fatalf("v3 with 16 stripes returned %d parts, want 16", len(parts))
	}
	seen := map[string]int{}
	for i, ix := range parts {
		for word := range ix {
			if j, ok := seen[word]; ok {
				t.Errorf("word %q is in parts %d and %d", word, j, i)
			}
			seen[word] = i
		}
	}
}

func TestV4ReportsMergeTime(t *testing.T) {
	for _, v := range []string{"v4seq", "v4tree"} {
		var merge time.Duration
		build(t, v, zipfCorpus(20), Options{Threads: 4, MergeTime: &merge})
		if merge <= 0 {
			t.Errorf("%s: merge time = %v, want > 0", v, merge)
		}
	}
}

func TestStripeFillsCacheLine(t *testing.T) {
	if size := unsafe.Sizeof(stripe{}); size != cacheLine {
		t.Errorf("stripe is %d bytes, want %d", size, cacheLine)
	}
}

func TestV1SingleThreadIsCorrect(t *testing.T) {
	texts := zipfCorpus(20)
	want := write(t, build(t, "v0", texts, Options{})...)
	if got := write(t, build(t, "v1", texts, Options{Threads: 1})...); got != want {
		t.Errorf("v1 with one thread differs from v0: %s", firstDiff(got, want))
	}
}

func TestV1OnlyLosesUpdates(t *testing.T) {
	if raceEnabled {
		t.Skip("v1 races by design")
	}
	texts := zipfCorpus(60)
	want := build(t, "v0", texts, Options{})[0]
	for word, e := range build(t, "v1", texts, Options{Threads: 8})[0] {
		w := want[word]
		if w == nil {
			t.Errorf("v1 invented word %q", word)
			continue
		}
		if e.Total > w.Total {
			t.Errorf("v1 total of %q = %d, want at most %d", word, e.Total, w.Total)
		}
		for doc, n := range e.Docs {
			if n > w.Docs[doc] {
				t.Errorf("v1 count of %q in doc %d = %d, want at most %d", word, doc, n, w.Docs[doc])
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
	texts := make([][]byte, 32)
	for i := range texts {
		texts[i] = bytes.Repeat([]byte("the "), 50_000)
	}
	want := len(texts) * 50_000
	for range 5 {
		if got := Occurrences(build(t, "v1", texts, Options{Threads: 8})...); got < want {
			t.Logf("lost %d of %d updates", want-got, want)
			return
		}
	}
	t.Errorf("v1 lost no updates in 5 runs")
}
