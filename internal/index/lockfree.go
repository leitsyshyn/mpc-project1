package index

import (
	"hash/maphash"
	"sync"
	"sync/atomic"
)

// casTable is the shared index of the lock-free version: the layout of V1's
// racyTable with both of its races fixed by atomic operations. A bucket's head
// is replaced by compare-and-swap, so a concurrent insert makes the loser
// retry instead of being lost, and counters are incremented atomically. The
// other fields of a node never change once it is published.
type casTable struct {
	buckets []atomic.Pointer[casNode] // len is a power of two
}

type casNode struct {
	hash uint64
	doc  int
	word string
	n    atomic.Int64
	next *casNode
}

func (t *casTable) inc(h uint64, word []byte, doc int) {
	b := &t.buckets[bucket(h, doc, len(t.buckets))]
	var nd *casNode // allocated on the first failed lookup, reused on retries
	for {
		head := b.Load()
		for x := head; x != nil; x = x.next {
			if x.hash == h && x.doc == doc && x.word == string(word) {
				x.n.Add(1)
				return
			}
		}
		if nd == nil {
			nd = &casNode{hash: h, doc: doc, word: string(word)}
			nd.n.Store(1)
		}
		nd.next = head
		if b.CompareAndSwap(head, nd) {
			return
		}
		// Another goroutine changed the bucket, maybe inserting this very key: look again.
	}
}

// lockFree builds the index in a casTable without any locks.
func lockFree(texts [][]byte, o Options) []Index {
	t := &casTable{buckets: make([]atomic.Pointer[casNode], bucketCount(texts))}
	parallel(texts, o, func(_, doc int) {
		for w := range o.words(texts[doc]) {
			h := maphash.Bytes(seed, w)
			t.inc(h, w, doc)
			t.inc(h, w, allDocs)
		}
	})
	ix := Index{}
	for i := range t.buckets {
		for nd := t.buckets[i].Load(); nd != nil; nd = nd.next {
			ix.addCount(nd.word, nd.doc, int(nd.n.Load()))
		}
	}
	return []Index{ix}
}

// wordDoc keys the sync.Map version: a word and a document, or allDocs for the
// word's total, as in the tables of V1 and the lock-free version.
type wordDoc struct {
	word string
	doc  int
}

// syncMapBuild builds the index in the standard library's concurrent map,
// sync.Map, from (word, document) to an atomic counter.
func syncMapBuild(texts [][]byte, o Options) []Index {
	var m sync.Map
	inc := func(word string, doc int) {
		k := wordDoc{word, doc}
		c, ok := m.Load(k)
		if !ok {
			c, _ = m.LoadOrStore(k, new(atomic.Int64))
		}
		c.(*atomic.Int64).Add(1)
	}
	parallel(texts, o, func(_, doc int) {
		for w := range o.words(texts[doc]) {
			word := string(w) // keys are interfaces, so every occurrence copies the word
			inc(word, doc)
			inc(word, allDocs)
		}
	})
	ix := Index{}
	m.Range(func(k, c any) bool {
		ix.addCount(k.(wordDoc).word, k.(wordDoc).doc, int(c.(*atomic.Int64).Load()))
		return true
	})
	return []Index{ix}
}
