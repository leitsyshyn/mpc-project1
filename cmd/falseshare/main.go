// Command falseshare measures the effect of false sharing on per-goroutine counters.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	threads := flag.Int("p", 1, "number of goroutines")
	flag.Parse()

	// TODO: compare packed vs cache-line padded counters.
	fmt.Fprintf(os.Stderr, "p=%d\n", *threads)
}
