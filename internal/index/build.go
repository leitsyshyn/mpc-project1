// Package index builds an inverted index of a text corpus with several
// synchronization strategies.
package index

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
)

// Options configures the parallel versions.
type Options struct {
	Threads int // worker goroutines
}

// BuildFunc builds the index of texts, where texts[i] is document i. The result
// may be split into parts with disjoint words.
type BuildFunc func(texts [][]byte, o Options) []Index

var versions = map[string]BuildFunc{
	"v0": sequential,
	"v1": unsynchronized,
}

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
func sequential(texts [][]byte, _ Options) []Index {
	ix := Index{}
	for doc, text := range texts {
		for w := range Words(text) {
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
	size := 0
	for _, text := range texts {
		size += len(text)
	}
	t := newRacyTable(size)
	parallel(texts, o, func(_, doc int) {
		for w := range Words(texts[doc]) {
			t.add(w, doc)
		}
	})
	return []Index{t.index()}
}
