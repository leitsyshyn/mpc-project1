// Command invindex builds an inverted index over a directory of text files.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	var (
		version = flag.String("v", "v0", "implementation: v0 (sequential), v1 (no sync), v2 (single mutex), v3 (striped locks), v4 (local + merge)")
		threads = flag.Int("p", 1, "number of goroutines")
		stripes = flag.Int("k", 64, "number of locks for v3")
		in      = flag.String("in", "data", "input directory")
		out     = flag.String("out", "index.tsv", "output file")
	)
	flag.Parse()

	// TODO: read files, build index with the selected version, write output.
	fmt.Fprintf(os.Stderr, "version=%s p=%d k=%d in=%s out=%s\n", *version, *threads, *stripes, *in, *out)
}
