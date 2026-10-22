// Command falseshare measures false sharing: P goroutines increment their own
// counters, which lie a given number of bytes apart. A stride of 8 bytes is
// the packed array A of the assignment, a stride of 64 the padded struct B.
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
	threadsList := fs.String("p", "1,2,4,8", "comma-separated numbers of goroutines")
	stridesList := fs.String("stride", "8,16,32,40,64,128,256", "comma-separated distances between counters in bytes")
	n := fs.Int("n", 10_000_000, "increments per goroutine")
	warmup := fs.Int("warmup", 1, "untimed runs before measuring")
	repeat := fs.Int("repeat", 5, "timed runs")
	csvPath := fs.String("csv", "", "append the timings to this CSV file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	threads, err := positiveInts(*threadsList)
	if err != nil {
		return fmt.Errorf("-p: %w", err)
	}
	strides, err := positiveInts(*stridesList)
	if err != nil {
		return fmt.Errorf("-stride: %w", err)
	}
	for _, s := range strides {
		if s%8 != 0 {
			return fmt.Errorf("-stride: %d is not a multiple of 8", s)
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
				if err := bench.AppendCSV(*csvPath, rows); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func counters(p, stride int) []*int64 {
	words := stride / 8
	buf := make([]int64, p*words+align/8)
	skip := int((align - uintptr(unsafe.Pointer(&buf[0]))%align) % align / 8)
	cs := make([]*int64, p)
	for i := range cs {
		cs[i] = &buf[skip+i*words]
	}
	return cs
}

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
