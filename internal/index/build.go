// Package index builds an inverted index of a text corpus with several
// synchronization strategies.
package index

import (
	"fmt"
	"maps"
	"slices"
	"strings"
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
