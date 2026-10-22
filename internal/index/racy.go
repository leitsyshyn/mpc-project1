package index

import (
	"hash/maphash"
	"math/bits"
)

// racyTable is the shared index of V1. Go's built-in map cannot serve here: it
// aborts the program on concurrent writes ("fatal error: concurrent map
// writes") instead of losing updates. So V1 uses a chained hash table with a
// fixed number of buckets that tolerates unsynchronized writers. A node is
// fully written by the goroutine that creates it; afterwards other goroutines
// only touch its word-sized fields n and next. Races on them lose updates but
// cannot corrupt memory: by the Go memory model a racy word-sized read observes
// some write.
//
// Each word has a node with its total (doc == allDocs) and a node per document
// it occurs in, mirroring Entry.Total and Entry.Docs.
type racyTable struct {
	buckets []*node
	mask    uint64
}

type node struct {
	hash uint64 // of word; nodes are matched by hash and doc only, never by word
	doc  int
	word string
	n    int
	next *node
}

const allDocs = -1

// newRacyTable sizes the table to one bucket per 64 bytes of input, which
// keeps chains short for natural text.
func newRacyTable(size int) *racyTable {
	n := 1 << bits.Len(uint(size/64))
	return &racyTable{buckets: make([]*node, n), mask: uint64(n - 1)}
}

func (t *racyTable) add(word []byte, doc int) {
	h := maphash.Bytes(seed, word)
	t.inc(h, word, doc)
	t.inc(h, word, allDocs)
}

func (t *racyTable) inc(h uint64, word []byte, doc int) {
	b := &t.buckets[(h+uint64(doc+1)*0x9e3779b97f4a7c15)&t.mask]
	for nd := *b; nd != nil; nd = nd.next {
		if nd.hash == h && nd.doc == doc {
			nd.n++ // DATA RACE: load, add, store; a concurrent increment between them is lost
			return
		}
	}
	*b = &node{hash: h, doc: doc, word: string(word), n: 1, next: *b} // DATA RACE: a concurrent insert into the bucket is lost
}

// index converts the table into an Index. Call it after all writers finish.
func (t *racyTable) index() Index {
	ix := Index{}
	for _, nd := range t.buckets {
		for ; nd != nil; nd = nd.next {
			e := ix[nd.word]
			if e == nil {
				e = &Entry{Docs: map[int]int{}}
				ix[nd.word] = e
			}
			if nd.doc == allDocs {
				e.Total += nd.n
			} else {
				e.Docs[nd.doc] += nd.n
			}
		}
	}
	return ix
}
