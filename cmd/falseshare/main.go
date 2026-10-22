// Command falseshare measures false sharing: P goroutines each increment their
// own counter, with the counters spaced a given number of bytes apart. There
// is no synchronization and no data race, since no counter is shared, yet
// counters that share a cache line slow each other down.
//
// A stride of 8 bytes is implementation A of the assignment (long counters[P]);
// 64 is implementation B (struct { long value; char pad[56]; }).
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"concurrent/internal/bench"
)

// align is the boundary the counters start at, a multiple of any cache line
// size of interest (128 bytes on Apple M1, 64 on most x86 CPUs).
const align = 128

func main() {
	if err := run(os.Args[1:], os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "falseshare:", err)
		os.Exit(1)
	}
}

func run(args []string, log io.Writer) error {
	fs := flag.NewFlagSet("falseshare", flag.ContinueOnError)
	fs.SetOutput(log)
	threadsFlag := fs.String("p", "1,2,4,8", "comma-separated numbers of goroutines")
	stridesFlag := fs.String("stride", "8,16,32,40,64,128,256", "comma-separated distances between counters, bytes")
	n := fs.Int("n", 10_000_000, "increments per goroutine")
	warmup := fs.Int("warmup", 1, "untimed runs before measuring")
	repeat := fs.Int("repeat", 5, "timed runs")
	csvPath := fs.String("csv", "", "append per-run timings to this CSV file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	threads, err := positiveInts(*threadsFlag)
	if err != nil {
		return fmt.Errorf("-p: %w", err)
	}
	strides, err := positiveInts(*stridesFlag)
	if err != nil {
		return fmt.Errorf("-stride: %w", err)
	}
	for _, s := range strides {
		if s%8 != 0 {
			return fmt.Errorf("-stride %d is not a multiple of the 8-byte counter", s)
		}
	}
	if *n < 1 || *repeat < 1 || *warmup < 0 {
		return errors.New("-n and -repeat must be positive and -warmup non-negative")
	}

	for _, stride := range strides {
		for _, p := range threads {
			times := bench.Measure(*warmup, *repeat, func() { increment(counters(p, stride), *n) })
			fmt.Fprintf(log, "stride %3d B, p=%d: %v\n", stride, p, bench.Summarize(times))
			if *csvPath != "" {
				rows := bench.Rows("falseshare", fmt.Sprintf("stride-%d", stride), p, 0, times)
				if err := bench.AppendCSV(*csvPath, bench.Header, rows); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// counters allocates p counters spaced stride bytes apart, the first one at an
// align-byte boundary.
func counters(p, stride int) []*int64 {
	words := stride / 8
	buf := make([]int64, p*words+align/8)
	skip := (align - uintptr(unsafe.Pointer(&buf[0]))%align) % align / 8
	cs := make([]*int64, p)
	for i := range cs {
		cs[i] = &buf[int(skip)+i*words]
	}
	return cs
}

// increment runs one goroutine per counter, each adding 1 to its own counter n
// times.
func increment(cs []*int64, n int) {
	var wg sync.WaitGroup
	for _, c := range cs {
		wg.Go(func() {
			for range n {
				*c++
			}
		})
	}
	wg.Wait()
}

func positiveInts(list string) ([]int, error) {
	var ns []int
	for f := range strings.SplitSeq(list, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(f))
		if err != nil {
			return nil, err
		}
		if n < 1 {
			return nil, fmt.Errorf("%d is not positive", n)
		}
		ns = append(ns, n)
	}
	return ns, nil
}
