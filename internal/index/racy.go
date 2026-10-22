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
	buckets []*node // len is a power of two
}

type node struct {
	hash uint64 // of word; nodes are matched by hash and doc only, never by word
	doc  int
	word string
	n    int
	next *node
}

const allDocs = -1

// bucketCount sizes the fixed-size tables of V1 and the lock-free version: a
// power of two near one bucket per 64 bytes of input, which keeps chains short
// for natural text.
func bucketCount(texts [][]byte) int {
	size := 0
	for _, text := range texts {
		size += len(text)
	}
	return 1 << bits.Len(uint(size/64))
}

// bucket returns the bucket of the node for word hash h and document doc.
func bucket(h uint64, doc, buckets int) int {
	return int((h + uint64(doc+1)*0x9e3779b97f4a7c15) & uint64(buckets-1))
}

// addCount adds n occurrences of word in doc, or n to its total if doc is
// allDocs, to an index being assembled from table nodes.
func (ix Index) addCount(word string, doc, n int) {
	e := ix[word]
	if e == nil {
		e = &Entry{Docs: map[int]int{}}
		ix[word] = e
	}
	if doc == allDocs {
		e.Total += n
	} else {
		e.Docs[doc] += n
	}
}

func (t *racyTable) add(word []byte, doc int) {
	h := maphash.Bytes(seed, word)
	t.inc(h, word, doc)
	t.inc(h, word, allDocs)
}

func (t *racyTable) inc(h uint64, word []byte, doc int) {
	b := &t.buckets[bucket(h, doc, len(t.buckets))]
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
			ix.addCount(nd.word, nd.doc, nd.n)
		}
	}
	return ix
}
