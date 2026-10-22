package index

import (
	"maps"
	"strings"
	"testing"
)

func write(t *testing.T, parts ...Index) string {
	t.Helper()
	var sb strings.Builder
	if err := Write(&sb, parts...); err != nil {
		t.Fatal(err)
	}
	return sb.String()
}

func TestAdd(t *testing.T) {
	ix := Index{}
	for _, wd := range []struct {
		word string
		doc  int
	}{{"cat", 0}, {"cat", 0}, {"cat", 2}, {"dog", 1}} {
		ix.Add([]byte(wd.word), wd.doc)
	}

	if len(ix) != 2 {
		t.Errorf("index has %d words, want 2", len(ix))
	}
	if cat := ix["cat"]; cat == nil || cat.Total != 3 || !maps.Equal(cat.Docs, map[int]int{0: 2, 2: 1}) {
		t.Errorf("cat = %+v, want total 3, docs {0:2 2:1}", cat)
	}
	if dog := ix["dog"]; dog == nil || dog.Total != 1 || !maps.Equal(dog.Docs, map[int]int{1: 1}) {
		t.Errorf("dog = %+v, want total 1, docs {1:1}", dog)
	}
}

func TestAddCopiesWord(t *testing.T) {
	ix := Index{}
	buf := []byte("cat")
	ix.Add(buf, 0)
	copy(buf, "dog")
	if _, ok := ix["cat"]; !ok {
		t.Errorf("key changed after mutating the input buffer: %v", ix)
	}
}

func TestMerge(t *testing.T) {
	dst, src := Index{}, Index{}
	for _, wd := range []struct {
		ix   Index
		word string
		doc  int
	}{
		{dst, "both", 0}, {dst, "both", 0}, {dst, "dst", 1},
		{src, "both", 2}, {src, "both", 0}, {src, "src", 3},
	} {
		wd.ix.Add([]byte(wd.word), wd.doc)
	}

	dst.merge(src)

	want := map[string]Entry{
		"both": {Total: 4, Docs: map[int]int{0: 3, 2: 1}},
		"dst":  {Total: 1, Docs: map[int]int{1: 1}},
		"src":  {Total: 1, Docs: map[int]int{3: 1}},
	}
	if len(dst) != len(want) {
		t.Errorf("merged %d words, want %d", len(dst), len(want))
	}
	for word, w := range want {
		if e := dst[word]; e == nil || e.Total != w.Total || !maps.Equal(e.Docs, w.Docs) {
			t.Errorf("%s = %+v, want %+v", word, e, w)
		}
	}
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

func TestWriteFormat(t *testing.T) {
	ix := Index{}
	add := func(word string, doc, n int) {
		for range n {
			ix.Add([]byte(word), doc)
		}
	}
	add("zebra", 3, 1)
	add("apple", 41, 1)
	add("apple", 17, 12)
	add("apple", 3, 5)
	add("тест", 0, 2)
	add("ábc", 1, 1)
	add("tie", 10, 2)
	add("tie", 2, 2)
	add("tie", 7, 3)

	want := "apple\t18\tdoc17:12,doc3:5,doc41:1\n" +
		"tie\t7\tdoc7:3,doc2:2,doc10:2\n" +
		"zebra\t1\tdoc3:1\n" +
		"ábc\t1\tdoc1:1\n" +
		"тест\t2\tdoc0:2\n"
	if got := write(t, ix); got != want {
		t.Errorf("Write =\n%s\nwant\n%s", got, want)
	}
}

func TestWriteMergesDisjointParts(t *testing.T) {
	a, b := Index{}, Index{}
	a.Add([]byte("bbb"), 0)
	b.Add([]byte("aaa"), 1)
	b.Add([]byte("ccc"), 2)

	want := "aaa\t1\tdoc1:1\nbbb\t1\tdoc0:1\nccc\t1\tdoc2:1\n"
	if got := write(t, a, b); got != want {
		t.Errorf("Write =\n%s\nwant\n%s", got, want)
	}
}

func TestWriteEmpty(t *testing.T) {
	if got := write(t, Index{}); got != "" {
		t.Errorf("Write = %q, want empty", got)
	}
}
