// Package index builds an inverted index of a text corpus with several
// synchronization strategies.
package index

import (
	"fmt"
	"hash/maphash"
	"iter"
	"maps"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Options configures the parallel versions.
type Options struct {
	Threads int // worker goroutines
	Stripes int // V3: number of locks K

	// Merge, if set, receives the duration of V4's merge phase.
	Merge *time.Duration

	// Tokenize splits a document into words; nil means Words.
	Tokenize func(text []byte) iter.Seq[[]byte]
}

func (o Options) words(text []byte) iter.Seq[[]byte] {
	if o.Tokenize == nil {
		return Words(text)
	}
	return o.Tokenize(text)
}

// BuildFunc builds the index of texts, where texts[i] is document i. The result
// may be split into parts with disjoint words.
type BuildFunc func(texts [][]byte, o Options) []Index

var versions = map[string]BuildFunc{
	"v0": sequential,
	"v1": unsynchronized,
	"v2": singleLock,
	"v3": striped,
	"v4seq": func(texts [][]byte, o Options) []Index {
		return localThenMerge(texts, o, mergeSequential)
	},
	"v4tree": func(texts [][]byte, o Options) []Index {
		return localThenMerge(texts, o, mergeTree)
	},
	"cas":     lockFree,
	"syncmap": syncMapBuild,
}

// seed keys the hash that picks V1's buckets and V3's stripes.
var seed = maphash.MakeSeed()

// cacheLine is the cache line size of the target machine (Apple M1; 64 bytes
// on most x86 CPUs, so 128 covers both).
const cacheLine = 128

// Versions returns the names of all versions in order.
func Versions() []string { return slices.Sorted(maps.Keys(versions)) }

// Version returns the build function of the named version.
func Version(name string) (BuildFunc, error) {
	build, ok := versions[name]
	if !ok {
		return nil, fmt.Errorf("unknown version %q, want one of: %s", name, strings.Join(Versions(), ", "))
	}
	return build, nil
}

// sequential is V0: one goroutine and no synchronization.
func sequential(texts [][]byte, o Options) []Index {
	ix := Index{}
	for doc, text := range texts {
		for w := range o.words(text) {
			ix.Add(w, doc)
		}
	}
	return []Index{ix}
}

// parallel calls work for every document on o.Threads goroutines. Work is
// split by whole files: each goroutine repeatedly takes the next unprocessed
// document from a shared counter, which balances files of different sizes.
func parallel(texts [][]byte, o Options, work func(worker, doc int)) {
	var next atomic.Int64
	var wg sync.WaitGroup
	for w := range o.Threads {
		wg.Go(func() {
			for doc := int(next.Add(1)) - 1; doc < len(texts); doc = int(next.Add(1)) - 1 {
				work(w, doc)
			}
		})
	}
	wg.Wait()
}

// unsynchronized is V1: goroutines update one shared index without any mutual
// exclusion, so concurrent updates get lost. See racyTable.
func unsynchronized(texts [][]byte, o Options) []Index {
	t := &racyTable{buckets: make([]*node, bucketCount(texts))}
	parallel(texts, o, func(_, doc int) {
		for w := range o.words(texts[doc]) {
			t.add(w, doc)
		}
	})
	return []Index{t.index()}
}

// singleLock is V2: one shared index guarded by a single mutex taken for every
// update.
func singleLock(texts [][]byte, o Options) []Index {
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

// stripe is one of V3's K independent index parts with its own lock, padded so
// that neighboring locks do not share a cache line.
type stripe struct {
	mu sync.Mutex
	ix Index
	_  [cacheLine - 16]byte
}

// striped is V3: the index is split into K stripes and a word belongs to
// stripe hash(word) mod K, so updates of different stripes do not contend.
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

// localThenMerge is V4: every goroutine builds its own index without
// synchronization, then the local indexes are merged into one.
func localThenMerge(texts [][]byte, o Options, merge func([]Index) Index) []Index {
	locals := make([]Index, o.Threads)
	for i := range locals {
		locals[i] = Index{}
	}
	parallel(texts, o, func(worker, doc int) {
		ix := locals[worker]
		for w := range o.words(texts[doc]) {
			ix.Add(w, doc)
		}
	})

	start := time.Now()
	ix := merge(locals)
	if o.Merge != nil {
		*o.Merge = time.Since(start)
	}
	return []Index{ix}
}

// mergeSequential merges the local indexes one after another into the first:
// P-1 sequential steps.
func mergeSequential(locals []Index) Index {
	for _, ix := range locals[1:] {
		locals[0].merge(ix)
	}
	return locals[0]
}

// mergeTree merges the local indexes pairwise along a balanced binary tree:
// each round merges disjoint pairs in parallel and halves the number of
// indexes, so there are ceil(log2 P) sequential steps.
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
