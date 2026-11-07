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

// Options configures a build. Threads and Stripes must be positive; Stripes
// is the number of locks K of v3. If MergeTime is not nil, v4 stores the
// duration of its merge phase in it. A nil Tokenize means [FastWords].
type Options struct {
	Threads   int
	Stripes   int
	MergeTime *time.Duration
	Tokenize  TokenizeFunc
}

// BuildFunc builds the index of texts, document i being texts[i], as parts
// with disjoint words.
type BuildFunc func(texts [][]byte, o Options) []Index

var (
	versions = map[string]BuildFunc{
		"v0":      sequential,
		"v1":      shared(newRacyTable),
		"v2":      singleMutex,
		"v3":      striped,
		"v4seq":   localIndexes(mergeSequential),
		"v4tree":  localIndexes(mergeTree),
		"cas":     shared(newCASTable),
		"syncmap": shared(newSyncMapTable),
	}
	tokenizers = map[string]TokenizeFunc{
		"std":  Words,
		"swar": FastWords,
	}
	seed = maphash.MakeSeed()
)

// Versions returns the names of the versions in order.
func Versions() []string { return names(versions) }

// Version returns the named version.
func Version(name string) (BuildFunc, error) { return lookup(versions, "version", name) }

// Tokenizers returns the names of the tokenizers in order.
func Tokenizers() []string { return names(tokenizers) }

// Tokenizer returns the named tokenizer.
func Tokenizer(name string) (TokenizeFunc, error) { return lookup(tokenizers, "tokenizer", name) }

func names[V any](m map[string]V) []string { return slices.Sorted(maps.Keys(m)) }

func lookup[V any](m map[string]V, kind, name string) (V, error) {
	v, ok := m[name]
	if !ok {
		return v, fmt.Errorf("unknown %s %q, want one of: %s", kind, name, strings.Join(names(m), ", "))
	}
	return v, nil
}

func (o Options) words(text []byte) iter.Seq[[]byte] {
	if o.Tokenize == nil {
		return FastWords(text)
	}
	return o.Tokenize(text)
}

func parallel(texts [][]byte, o Options, work func(worker, doc int)) {
	var next atomic.Int64
	var wg sync.WaitGroup
	for worker := range o.Threads {
		wg.Go(func() {
			for doc := int(next.Add(1)) - 1; doc < len(texts); doc = int(next.Add(1)) - 1 {
				work(worker, doc)
			}
		})
	}
	wg.Wait()
}
