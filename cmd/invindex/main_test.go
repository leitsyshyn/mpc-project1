package main

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"concurrent/internal/bench"
)

const wantIndex = "hello\t2\tdoc0:2\nworld\t2\tdoc0:1,doc1:1\n"

func corpus(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, text := range map[string]string{"a.txt": "Hello hello world", "b.txt": "World!"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRun(t *testing.T) {
	in := corpus(t)
	tests := []struct {
		name string
		args []string
		rows map[string]int
	}{
		{
			name: "v0",
			args: []string{"-v", "v0", "-repeat", "3"},
			rows: map[string]int{"invindex,v0,1,,": 3},
		},
		{
			name: "v3 records k",
			args: []string{"-v", "v3", "-p", "2", "-k", "16", "-repeat", "1"},
			rows: map[string]int{"invindex,v3,2,16,": 1},
		},
		{
			name: "v4 records merges",
			args: []string{"-v", "v4tree", "-p", "2", "-repeat", "2"},
			rows: map[string]int{"invindex,v4tree,2,,": 2, "invindex,v4tree-merge,2,,": 2},
		},
		{
			name: "non-default tokenizer in version",
			args: []string{"-tokenizer", "std", "-repeat", "1"},
			rows: map[string]int{"invindex,v0-std,1,,": 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			out, csv := filepath.Join(dir, "index.tsv"), filepath.Join(dir, "results.csv")
			var log bytes.Buffer
			if err := run(append(tt.args, "-in", in, "-out", out, "-csv", csv), &log); err != nil {
				t.Fatal(err)
			}
			if got := read(t, out); got != wantIndex {
				t.Errorf("index =\n%s\nwant\n%s", got, wantIndex)
			}
			if !strings.Contains(log.String(), "occurrences: 4") {
				t.Errorf("log does not report 4 occurrences:\n%s", &log)
			}

			lines := strings.Split(strings.TrimSpace(read(t, csv)), "\n")
			if want := strings.Join(bench.Header, ","); lines[0] != want {
				t.Errorf("csv header = %q, want %q", lines[0], want)
			}
			total := 0
			for prefix, want := range tt.rows {
				total += want
				got := 0
				for _, l := range lines[1:] {
					if strings.HasPrefix(l, prefix) {
						got++
					}
				}
				if got != want {
					t.Errorf("csv has %d rows starting with %q, want %d:\n%s", got, prefix, want, strings.Join(lines, "\n"))
				}
			}
			if len(lines)-1 != total {
				t.Errorf("csv has %d rows, want %d", len(lines)-1, total)
			}
		})
	}
}

func TestRunWritesCPUProfile(t *testing.T) {
	dir := t.TempDir()
	prof := filepath.Join(dir, "cpu.pprof")
	args := []string{"-in", corpus(t), "-out", filepath.Join(dir, "index.tsv"), "-repeat", "1", "-cpuprofile", prof}
	if err := run(args, io.Discard); err != nil {
		t.Fatal(err)
	}
	if read(t, prof) == "" {
		t.Error("CPU profile is empty")
	}
}

func TestRunHelp(t *testing.T) {
	if err := run([]string{"-h"}, io.Discard); !errors.Is(err, flag.ErrHelp) {
		t.Errorf("run(-h) = %v, want flag.ErrHelp", err)
	}
}

func TestRunRejectsBadArgs(t *testing.T) {
	for _, args := range [][]string{
		{"-v", "v9"},
		{"-repeat", "0"},
		{"-p", "0"},
		{"-v", "v3", "-k", "0"},
		{"-tokenizer", "regex"},
		{"-in", filepath.Join(t.TempDir(), "missing")},
	} {
		if err := run(args, io.Discard); err == nil || errors.Is(err, flag.ErrHelp) {
			t.Errorf("run(%q) = %v, want error", args, err)
		}
	}
}
