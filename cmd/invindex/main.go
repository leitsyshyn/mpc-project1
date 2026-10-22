// Command invindex builds the inverted index of a directory of text files with
// one of the versions of the index package and reports the build time.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/pprof"
	"slices"
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
	threads := fs.Int("p", 1, "number of goroutines, ignored by v0")
	stripes := fs.Int("k", 1024, "number of locks K of v3")
	tokenizer := fs.String("tokenizer", "simple", "tokenizer: "+strings.Join(index.Tokenizers(), ", "))
	in := fs.String("in", "data", "corpus directory")
	out := fs.String("out", "index.tsv", "output file")
	warmup := fs.Int("warmup", 1, "untimed runs before measuring")
	repeat := fs.Int("repeat", 5, "timed runs")
	csvPath := fs.String("csv", "", "append the timings to this CSV file")
	cpuProfile := fs.String("cpuprofile", "", "write a CPU profile of the runs to this file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	build, err := index.Version(*version)
	if err != nil {
		return err
	}
	tokenize, err := index.Tokenizer(*tokenizer)
	if err != nil {
		return err
	}
	if *threads < 1 || *stripes < 1 || *repeat < 1 || *warmup < 0 {
		return errors.New("-p, -k and -repeat must be positive and -warmup non-negative")
	}
	label, k := *version, 0
	if *tokenizer != "simple" {
		label += "-" + *tokenizer
	}
	if *version == "v3" {
		k = *stripes
	}

	texts, err := index.Load(*in)
	if err != nil {
		return err
	}
	size := 0
	for _, text := range texts {
		size += len(text)
	}
	fmt.Fprintf(log, "corpus: %d files, %.1f MiB\n", len(texts), float64(size)/(1<<20))

	stop, err := startProfile(*cpuProfile)
	if err != nil {
		return err
	}
	var parts []index.Index
	var merge time.Duration
	var merges []time.Duration
	opts := index.Options{Threads: *threads, Stripes: *stripes, MergeTime: &merge, Tokenize: tokenize}
	times := bench.Measure(*warmup, *repeat, func() {
		parts = build(texts, opts)
		merges = append(merges, merge)
	})
	merges = merges[*warmup:]
	if err := stop(); err != nil {
		return err
	}

	fmt.Fprintf(log, "%s p=%d k=%d: %v over %d runs\n", label, *threads, k, bench.Summarize(times), *repeat)
	rows := bench.Rows("invindex", label, *threads, k, times)
	if slices.Max(merges) > 0 {
		fmt.Fprintf(log, "  of which merge: %v\n", bench.Summarize(merges))
		rows = append(rows, bench.Rows("invindex", label+"-merge", *threads, k, merges)...)
	}
	fmt.Fprintf(log, "occurrences: %d\n", index.Occurrences(parts...))

	if *csvPath != "" {
		if err := bench.AppendCSV(*csvPath, rows); err != nil {
			return err
		}
	}
	return writeIndex(*out, parts)
}

func startProfile(path string) (stop func() error, err error) {
	if path == "" {
		return func() error { return nil }, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		return nil, errors.Join(err, f.Close())
	}
	return func() error {
		pprof.StopCPUProfile()
		return f.Close()
	}, nil
}

func writeIndex(path string, parts []index.Index) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	return index.Write(f, parts...)
}
