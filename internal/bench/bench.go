// Package bench implements the measurement methodology of the experiments:
// untimed warmup runs, repeated timed runs, median and range, and results.csv.
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

// Header is the header of results.csv.
var Header = []string{"program", "version", "threads", "k", "run", "seconds"}

// Measure runs fn warmup times untimed, then repeat times timed, collecting
// garbage before every run, and returns the timed durations.
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

// Summarize returns the summary of a non-empty set of timings.
func Summarize(times []time.Duration) Summary {
	s := slices.Sorted(slices.Values(times))
	n := len(s)
	return Summary{Median: (s[(n-1)/2] + s[n/2]) / 2, Min: s[0], Max: s[n-1]}
}

// String formats s as "median M (min A, max B)".
func (s Summary) String() string {
	return fmt.Sprintf("median %v (min %v, max %v)", s.Median, s.Min, s.Max)
}

// Rows formats the timings of one configuration as results.csv rows, leaving
// the k column empty when k is 0.
func Rows(program, version string, threads, k int, times []time.Duration) [][]string {
	ks := ""
	if k != 0 {
		ks = strconv.Itoa(k)
	}
	rows := make([][]string, len(times))
	for i, d := range times {
		rows[i] = []string{
			program,
			version,
			strconv.Itoa(threads),
			ks,
			strconv.Itoa(i + 1),
			strconv.FormatFloat(d.Seconds(), 'f', 6, 64),
		}
	}
	return rows
}

// AppendCSV appends rows to the CSV file at path, starting a new file with
// [Header].
func AppendCSV(path string, rows [][]string) (err error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		rows = append([][]string{Header}, rows...)
	}
	return csv.NewWriter(f).WriteAll(rows)
}
