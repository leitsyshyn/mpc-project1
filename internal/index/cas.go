package index

import (
	"hash/maphash"
	"sync/atomic"
)

type casTable struct {
	buckets []atomic.Pointer[casNode]
}

type casNode struct {
	hash  uint64
	doc   int
	word  string
	count atomic.Int64
	next  *casNode
}

func newCASTable(texts [][]byte) sharedTable {
	return &casTable{buckets: make([]atomic.Pointer[casNode], bucketCount(texts))}
}

func (t *casTable) add(word []byte, doc int) {
	h := maphash.Bytes(seed, word)
	t.inc(h, word, doc)
	t.inc(h, word, allDocs)
}

func (t *casTable) inc(hash uint64, word []byte, doc int) {
	b := &t.buckets[bucket(hash, doc, len(t.buckets))]
	var nd *casNode
	for {
		head := b.Load()
		for x := head; x != nil; x = x.next {
			if x.hash == hash && x.doc == doc && x.word == string(word) {
				x.count.Add(1)
				return
			}
		}
		if nd == nil {
			nd = &casNode{hash: hash, doc: doc, word: string(word)}
			nd.count.Store(1)
		}
		nd.next = head
		if b.CompareAndSwap(head, nd) {
			return
		}
	}
}

func (t *casTable) index() Index {
	ix := Index{}
	for i := range t.buckets {
		for nd := t.buckets[i].Load(); nd != nil; nd = nd.next {
			ix.addCount(nd.word, nd.doc, int(nd.count.Load()))
		}
	}
	return ix
}
