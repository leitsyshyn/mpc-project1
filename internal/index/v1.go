package index

import "hash/maphash"

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

func newRacyTable(texts [][]byte) sharedTable {
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
