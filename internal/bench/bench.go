// Package bench implements the measurement methodology shared by all
// experiments: untimed warmup runs, repeated timed runs, median and range, and
// a common results.csv schema.
package bench

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"runtime"
	"slices"
	"strconv"
	"time"
)

// Measure calls fn warmup times untimed, then repeat times timed, and returns
// the timed durations. It collects garbage before every call so that leftovers
// of one run do not slow down the next.
func Measure(warmup, repeat int, fn func()) []time.Duration {
	times := make([]time.Duration, 0, repeat)
	for i := range warmup + repeat {
		runtime.GC()
		start := time.Now()
		fn()
		if i >= warmup {
			times = append(times, time.Since(start))
		}
	}
	return times
}

// Summary is the median and range of a set of timings.
type Summary struct {
	Median, Min, Max time.Duration
}

// Summarize computes the summary of a non-empty set of timings.
func Summarize(times []time.Duration) Summary {
	s := slices.Sorted(slices.Values(times))
	n := len(s)
	return Summary{Median: (s[(n-1)/2] + s[n/2]) / 2, Min: s[0], Max: s[n-1]}
}

func (s Summary) String() string {
	return fmt.Sprintf("median %v (min %v, max %v)", s.Median, s.Min, s.Max)
}

// Header is the schema of results.csv.
var Header = []string{"program", "version", "threads", "k", "run", "seconds"}

// Rows formats the timings of one configuration as results.csv rows. A zero k
// means the configuration has no K and leaves the column empty.
func Rows(program, version string, threads, k int, times []time.Duration) [][]string {
	ks := ""
	if k != 0 {
		ks = strconv.Itoa(k)
	}
	rows := make([][]string, len(times))
	for i, d := range times {
		rows[i] = []string{program, version, strconv.Itoa(threads), ks, strconv.Itoa(i + 1), fmt.Sprintf("%.6f", d.Seconds())}
	}
	return rows
}

// AppendCSV appends rows to the CSV file at path, writing header first if the
// file is new or empty.
func AppendCSV(path string, header []string, rows [][]string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		return errors.Join(err, f.Close())
	}
	w := csv.NewWriter(f)
	if info.Size() == 0 {
		rows = append([][]string{header}, rows...)
	}
	return errors.Join(w.WriteAll(rows), f.Close())
}
