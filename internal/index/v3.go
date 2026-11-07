package index

import (
	"hash/maphash"
	"sync"
	"unsafe"
)

const cacheLine = 128

type stripe struct {
	mu sync.Mutex
	ix Index
	_  [cacheLine - unsafe.Sizeof(sync.Mutex{}) - unsafe.Sizeof(Index{})]byte
}

func striped(texts [][]byte, o Options) []Index {
	stripes := make([]stripe, o.Stripes)
	for i := range stripes {
		stripes[i].ix = Index{}
	}
	parallel(texts, o, func(_, doc int) {
		for w := range o.words(texts[doc]) {
			s := &stripes[maphash.Bytes(seed, w)%uint64(len(stripes))]
			s.mu.Lock()
			s.ix.Add(w, doc)
			s.mu.Unlock()
		}
	})
	parts := make([]Index, len(stripes))
	for i := range stripes {
		parts[i] = stripes[i].ix
	}
	return parts
}
