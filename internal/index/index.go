package index

import (
	"bufio"
	"cmp"
	"io"
	"slices"
	"strconv"
	"strings"
)

// Entry is the posting list of one word.
type Entry struct {
	Total int         // occurrences in the whole corpus
	Docs  map[int]int // document number -> occurrences
}

// Index maps a word to its posting list.
type Index map[string]*Entry

// Add records one occurrence of word in document doc. It does not retain word.
func (ix Index) Add(word []byte, doc int) {
	e := ix[string(word)] // the conversion does not allocate for a lookup
	if e == nil {
		e = &Entry{Docs: map[int]int{}}
		ix[string(word)] = e
	}
	e.Total++
	e.Docs[doc]++
}

// merge adds the postings of src to ix. Entries of words missing from ix are
// moved, not copied, so src must not be used afterwards.
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

// Write writes the index, one line per word:
//
//	word<TAB>total<TAB>docN:count,docM:count,...
//
// Lines are ordered by the word's UTF-8 bytes and postings by descending count,
// then ascending document number. The parts must have disjoint words.
func Write(w io.Writer, parts ...Index) error {
	type line struct {
		word string
		*Entry
	}
	type posting struct{ doc, n int }

	var lines []line
	for _, ix := range parts {
		for word, e := range ix {
			lines = append(lines, line{word, e})
		}
	}
	slices.SortFunc(lines, func(a, b line) int { return strings.Compare(a.word, b.word) })

	bw := bufio.NewWriterSize(w, 1<<20)
	var buf []byte
	var ps []posting
	for _, l := range lines {
		ps = ps[:0]
		for doc, n := range l.Docs {
			ps = append(ps, posting{doc, n})
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
