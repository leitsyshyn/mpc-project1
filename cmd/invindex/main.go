// Command invindex builds an inverted index over a directory of text files and
// reports how long the build takes.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

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
	in := fs.String("in", "data", "corpus directory")
	out := fs.String("out", "index.tsv", "output file")
	warmup := fs.Int("warmup", 1, "untimed runs before measuring")
	repeat := fs.Int("repeat", 5, "timed runs")
	csvPath := fs.String("csv", "", "append per-run timings to this CSV file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	build, err := index.Version(*version)
	if err != nil {
		return err
	}
	if *threads < 1 || *repeat < 1 || *warmup < 0 {
		return errors.New("-p and -repeat must be positive and -warmup non-negative")
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

	opts := index.Options{Threads: *threads}
	var parts []index.Index
	times := bench.Measure(*warmup, *repeat, func() { parts = build(texts, opts) })
	fmt.Fprintf(log, "%s p=%d: %v over %d runs\n", *version, *threads, bench.Summarize(times), *repeat)

	if *csvPath != "" {
		rows := bench.Rows("invindex", *version, *threads, 0, times)
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
