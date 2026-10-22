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
	"unsafe"
)

// Options configures a build. Stripes is the number of locks K of V3. If
// MergeTime is not nil, V4 stores the duration of its merge phase in it. A nil
// Tokenize means Words.
type Options struct {
	Threads   int
	Stripes   int
	MergeTime *time.Duration
	Tokenize  TokenizeFunc
}

func (o Options) words(text []byte) iter.Seq[[]byte] {
	if o.Tokenize == nil {
		return Words(text)
	}
	return o.Tokenize(text)
}

// BuildFunc builds the index of texts, document i being texts[i], as parts
// with disjoint words.
type BuildFunc func(texts [][]byte, o Options) []Index

var versions = map[string]BuildFunc{
	"v0":      sequential,
	"v1":      shared(newRacyTable),
	"v2":      singleMutex,
	"v3":      striped,
	"v4seq":   localIndexes(mergeSequential),
	"v4tree":  localIndexes(mergeTree),
	"cas":     shared(newCASTable),
	"syncmap": shared(newSyncMapTable),
}

var tokenizers = map[string]TokenizeFunc{
	"simple": Words,
	"fast":   FastWords,
}

// Versions returns the names of the versions in order.
func Versions() []string { return names(versions) }

// Version returns the named version.
func Version(name string) (BuildFunc, error) { return lookup(versions, "version", name) }

// Tokenizers returns the names of the tokenizers in order.
func Tokenizers() []string { return names(tokenizers) }

// Tokenizer returns the named tokenizer.
func Tokenizer(name string) (TokenizeFunc, error) { return lookup(tokenizers, "tokenizer", name) }

func names[T any](m map[string]T) []string { return slices.Sorted(maps.Keys(m)) }

func lookup[T any](m map[string]T, kind, name string) (T, error) {
	v, ok := m[name]
	if !ok {
		return v, fmt.Errorf("unknown %s %q, want one of: %s", kind, name, strings.Join(names(m), ", "))
	}
	return v, nil
}

var seed = maphash.MakeSeed()

const cacheLine = 128

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

func sequential(texts [][]byte, o Options) []Index {
	ix := Index{}
	for doc, text := range texts {
		for w := range o.words(text) {
			ix.Add(w, doc)
		}
	}
	return []Index{ix}
}

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
