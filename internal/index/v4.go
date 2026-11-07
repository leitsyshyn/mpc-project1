package index

import (
	"sync"
	"time"
)

func localIndexes(merge func([]Index) Index) BuildFunc {
	return func(texts [][]byte, o Options) []Index {
		locals := make([]Index, o.Threads)
		for i := range locals {
			locals[i] = Index{}
		}
		parallel(texts, o, func(worker, doc int) {
			for w := range o.words(texts[doc]) {
				locals[worker].Add(w, doc)
			}
		})

		start := time.Now()
		ix := merge(locals)
		if o.MergeTime != nil {
			*o.MergeTime = time.Since(start)
		}
		return []Index{ix}
	}
}

func mergeSequential(locals []Index) Index {
	for _, ix := range locals[1:] {
		locals[0].merge(ix)
	}
	return locals[0]
}

func mergeTree(locals []Index) Index {
	for step := 1; step < len(locals); step *= 2 {
		var wg sync.WaitGroup
		for i := 0; i+step < len(locals); i += 2 * step {
			wg.Go(func() { locals[i].merge(locals[i+step]) })
		}
		wg.Wait()
	}
	return locals[0]
}
