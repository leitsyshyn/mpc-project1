package index

import "sync"

func singleMutex(texts [][]byte, o Options) []Index {
	ix := Index{}
	var mu sync.Mutex
	parallel(texts, o, func(_, doc int) {
		for w := range o.words(texts[doc]) {
			mu.Lock()
			ix.Add(w, doc)
			mu.Unlock()
		}
	})
	return []Index{ix}
}
