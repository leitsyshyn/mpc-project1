// Command invindex builds an inverted index over a directory of text files and
// reports how long the build takes.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"iter"
	"os"
	"runtime/pprof"
	"strings"
	"time"

	"concurrent/internal/bench"
	"concurrent/internal/index"
)

func main() {
	if err := run(os.Args[1:], os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "invindex:", err)
		os.Exit(1)
	}
}

func run(args []string, log io.Writer) error {
	fs := flag.NewFlagSet("invindex", flag.ContinueOnError)
	fs.SetOutput(log)
	version := fs.String("v", "v0", "version: "+strings.Join(index.Versions(), ", "))
	threads := fs.Int("p", 1, "number of worker goroutines (v0 ignores it)")
	stripes := fs.Int("k", 1024, "number of locks K (v3 only)")
	in := fs.String("in", "data", "corpus directory")
	out := fs.String("out", "index.tsv", "output file")
	warmup := fs.Int("warmup", 1, "untimed runs before measuring")
	repeat := fs.Int("repeat", 5, "timed runs")
	csvPath := fs.String("csv", "", "append per-run timings to this CSV file")
	cpuProfile := fs.String("cpuprofile", "", "write a CPU profile of the timed runs to this file")
	tokenizer := fs.String("tokenizer", "simple", "tokenizer: simple (Words) or fast (FastWords)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	build, err := index.Version(*version)
	if err != nil {
		return err
	}
	tokenizers := map[string]func([]byte) iter.Seq[[]byte]{"simple": index.Words, "fast": index.FastWords}
	tokenize, ok := tokenizers[*tokenizer]
	if !ok {
		return fmt.Errorf("unknown tokenizer %q, want simple or fast", *tokenizer)
	}
	label := *version // the version as recorded in the CSV
	if *tokenizer != "simple" {
		label += "-" + *tokenizer
	}
	if *threads < 1 || *stripes < 1 || *repeat < 1 || *warmup < 0 {
		return errors.New("-p, -k and -repeat must be positive and -warmup non-negative")
	}

	// The corpus is read into memory before timing, so I/O is not measured.
	docs, err := index.Load(*in)
	if err != nil {
		return err
	}
	texts := make([][]byte, len(docs))
	size := 0
	for i, d := range docs {
		texts[i] = d.Text
		size += len(d.Text)
	}
	fmt.Fprintf(log, "corpus: %d files, %.1f MiB\n", len(docs), float64(size)/(1<<20))

	// Only V4 sets merge; each run's merge time is recorded separately.
	var merge time.Duration
	var merges []time.Duration
	opts := index.Options{Threads: *threads, Stripes: *stripes, Merge: &merge, Tokenize: tokenize}
	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			return err
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			return err
		}
		defer pprof.StopCPUProfile()
	}
	var parts []index.Index
	times := bench.Measure(*warmup, *repeat, func() {
		merge = 0
		parts = build(texts, opts)
		if merge != 0 {
			merges = append(merges, merge)
		}
	})
	if len(merges) > 0 {
		merges = merges[*warmup:]
	}
	k := 0 // only V3 has a K
	if *version == "v3" {
		k = *stripes
	}
	fmt.Fprintf(log, "%s p=%d k=%d: %v over %d runs\n", label, *threads, k, bench.Summarize(times), *repeat)
	if len(merges) > 0 {
		fmt.Fprintf(log, "  of which merge: %v\n", bench.Summarize(merges))
	}
	fmt.Fprintf(log, "occurrences: %d\n", index.Occurrences(parts...))

	if *csvPath != "" {
		rows := bench.Rows("invindex", label, *threads, k, times)
		rows = append(rows, bench.Rows("invindex", label+"-merge", *threads, k, merges)...)
		if err := bench.AppendCSV(*csvPath, bench.Header, rows); err != nil {
			return err
		}
	}
	return writeIndex(*out, parts)
}

func writeIndex(path string, parts []index.Index) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	return index.Write(f, parts...)
}
