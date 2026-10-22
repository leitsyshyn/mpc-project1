package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"
)

func TestCountersLayout(t *testing.T) {
	for _, stride := range []int{8, 40, 64, 128} {
		cs := counters(4, stride)
		if addr := uintptr(unsafe.Pointer(cs[0])); addr%cacheLine != 0 {
			t.Errorf("stride %d: first counter at %#x, not %d-byte aligned", stride, addr, cacheLine)
		}
		for i := 1; i < len(cs); i++ {
			if gap := uintptr(unsafe.Pointer(cs[i])) - uintptr(unsafe.Pointer(cs[i-1])); gap != uintptr(stride) {
				t.Errorf("stride %d: counters %d and %d are %d bytes apart", stride, i-1, i, gap)
			}
		}
	}
}

func TestIncrement(t *testing.T) {
	cs := counters(8, 8)
	increment(cs, 100_000)
	for i, c := range cs {
		if *c != 100_000 {
			t.Errorf("counter %d = %d, want 100000", i, *c)
		}
	}
}

func TestRun(t *testing.T) {
	csv := filepath.Join(t.TempDir(), "results.csv")
	args := []string{"-p", "1,2", "-stride", "8,64", "-n", "1000", "-repeat", "2", "-csv", csv}
	if err := run(args, io.Discard); err != nil {
		t.Fatal(err)
	}
	rows, err := os.ReadFile(csv)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(rows)), "\n")
	if len(lines) != 1+2*2*2 {
		t.Fatalf("csv has %d lines, want header + 2 strides x 2 thread counts x 2 runs:\n%s", len(lines), rows)
	}
	for _, prefix := range []string{"falseshare,stride-8,1,,1,", "falseshare,stride-64,2,,2,"} {
		if !strings.Contains(string(rows), "\n"+prefix) {
			t.Errorf("csv has no row starting with %q:\n%s", prefix, rows)
		}
	}
}

func TestRunRejectsBadArgs(t *testing.T) {
	for _, args := range [][]string{
		{"-stride", "12"},
		{"-stride", "0"},
		{"-p", "0"},
		{"-p", "two"},
		{"-n", "0"},
	} {
		if err := run(args, io.Discard); err == nil {
			t.Errorf("run(%q) succeeded, want error", args)
		}
	}
}
