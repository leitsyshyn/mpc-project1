package index

import (
	"sync"
	"sync/atomic"
)

type syncMapTable struct {
	m sync.Map
}

type wordDoc struct {
	word string
	doc  int
}

func newSyncMapTable([][]byte) sharedTable { return &syncMapTable{} }

func (t *syncMapTable) add(word []byte, doc int) {
	w := string(word)
	t.inc(wordDoc{word: w, doc: doc})
	t.inc(wordDoc{word: w, doc: allDocs})
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
