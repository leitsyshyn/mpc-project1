// Package index builds an inverted index of a text corpus with several
// synchronization strategies.
package index

import (
	"bufio"
	"cmp"
	"io"
	"slices"
	"strconv"
)

// Entry holds a word's number of occurrences in the corpus and per document.
type Entry struct {
	Total int
	Docs  map[int]int
}

// Index maps each word to its entry.
type Index map[string]*Entry

const allDocs = -1

// Add records one occurrence of word in document doc. It does not retain word.
func (ix Index) Add(word []byte, doc int) {
	e := ix[string(word)]
	if e == nil {
		e = newEntry()
		ix[string(word)] = e
	}
	e.Total++
	e.Docs[doc]++
}

func newEntry() *Entry { return &Entry{Docs: map[int]int{}} }

func (ix Index) merge(src Index) {
	for word, e := range src {
		d := ix[word]
		if d == nil {
			ix[word] = e
			continue
		}
		d.Total += e.Total
		for doc, n := range e.Docs {
			d.Docs[doc] += n
		}
	}
}

func (ix Index) addCount(word string, doc, n int) {
	e := ix[word]
	if e == nil {
		e = newEntry()
		ix[word] = e
	}
	if doc == allDocs {
		e.Total += n
	} else {
		e.Docs[doc] += n
	}
}

// Occurrences returns the total number of word occurrences in the parts.
func Occurrences(parts ...Index) int {
	n := 0
	for _, ix := range parts {
		for _, e := range ix {
			n += e.Total
		}
	}
	return n
}

// Write writes the parts, which must have disjoint words, one line per word:
//
//	word<TAB>total<TAB>docN:count,docM:count,...
//
// Lines are ordered by the word's UTF-8 bytes, postings by descending count,
// then ascending document number.
func Write(w io.Writer, parts ...Index) error {
	type line struct {
		word string
		*Entry
	}
	type posting struct{ doc, n int }

	n := 0
	for _, ix := range parts {
		n += len(ix)
	}
	lines := make([]line, 0, n)
	for _, ix := range parts {
		for word, e := range ix {
			lines = append(lines, line{word: word, Entry: e})
		}
	}
	slices.SortFunc(lines, func(a, b line) int { return cmp.Compare(a.word, b.word) })

	bw := bufio.NewWriterSize(w, 1<<20)
	var buf []byte
	var ps []posting
	for _, l := range lines {
		ps = ps[:0]
		for doc, n := range l.Docs {
			ps = append(ps, posting{doc: doc, n: n})
		}
		slices.SortFunc(ps, func(a, b posting) int {
			return cmp.Or(cmp.Compare(b.n, a.n), cmp.Compare(a.doc, b.doc))
		})

		buf = append(buf[:0], l.word...)
		buf = append(buf, '\t')
		buf = strconv.AppendInt(buf, int64(l.Total), 10)
		sep := byte('\t')
		for _, p := range ps {
			buf = append(buf, sep, 'd', 'o', 'c')
			sep = ','
			buf = strconv.AppendInt(buf, int64(p.doc), 10)
			buf = append(buf, ':')
			buf = strconv.AppendInt(buf, int64(p.n), 10)
		}
		buf = append(buf, '\n')
		if _, err := bw.Write(buf); err != nil {
			return err
		}
	}
	return bw.Flush()
}
