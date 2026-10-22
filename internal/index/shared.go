package index

import (
	"hash/maphash"
	"math/bits"
	"sync"
	"sync/atomic"
)

type sharedTable interface {
	add(word []byte, doc int)
	index() Index
}

func shared[T sharedTable](newTable func(texts [][]byte) T) BuildFunc {
	return func(texts [][]byte, o Options) []Index {
		t := newTable(texts)
		parallel(texts, o, func(_, doc int) {
			for w := range o.words(texts[doc]) {
				t.add(w, doc)
			}
		})
		return []Index{t.index()}
	}
}

const allDocs = -1

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

func bucketCount(texts [][]byte) int {
	size := 0
	for _, text := range texts {
		size += len(text)
	}
	return 1 << bits.Len(uint(size/64))
}

func bucket(hash uint64, doc, buckets int) int {
	return int((hash + uint64(doc+1)*0x9e3779b97f4a7c15) & uint64(buckets-1))
}

type racyTable struct {
	buckets []*racyNode
}

type racyNode struct {
	hash  uint64
	doc   int
	word  string
	count int
	next  *racyNode
}

func newRacyTable(texts [][]byte) *racyTable {
	return &racyTable{buckets: make([]*racyNode, bucketCount(texts))}
}

func (t *racyTable) add(word []byte, doc int) {
	h := maphash.Bytes(seed, word)
	t.inc(h, word, doc)
	t.inc(h, word, allDocs)
}

func (t *racyTable) inc(hash uint64, word []byte, doc int) {
	b := &t.buckets[bucket(hash, doc, len(t.buckets))]
	for nd := *b; nd != nil; nd = nd.next {
		if nd.hash == hash && nd.doc == doc {
			nd.count++
			return
		}
	}
	*b = &racyNode{hash: hash, doc: doc, word: string(word), count: 1, next: *b}
}

func (t *racyTable) index() Index {
	ix := Index{}
	for _, nd := range t.buckets {
		for ; nd != nil; nd = nd.next {
			ix.addCount(nd.word, nd.doc, nd.count)
		}
	}
	return ix
}

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

func newCASTable(texts [][]byte) *casTable {
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

type syncMapTable struct {
	m sync.Map
}

type wordDoc struct {
	word string
	doc  int
}

func newSyncMapTable([][]byte) *syncMapTable { return &syncMapTable{} }

func (t *syncMapTable) add(word []byte, doc int) {
	w := string(word)
	t.inc(wordDoc{w, doc})
	t.inc(wordDoc{w, allDocs})
}

func (t *syncMapTable) inc(key wordDoc) {
	c, ok := t.m.Load(key)
	if !ok {
		c, _ = t.m.LoadOrStore(key, new(atomic.Int64))
	}
	c.(*atomic.Int64).Add(1)
}

func (t *syncMapTable) index() Index {
	ix := Index{}
	t.m.Range(func(key, c any) bool {
		k := key.(wordDoc)
		ix.addCount(k.word, k.doc, int(c.(*atomic.Int64).Load()))
		return true
	})
	return ix
}
