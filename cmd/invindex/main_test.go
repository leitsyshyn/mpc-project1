package main

import (
	"bytes"
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
		{"v0", []string{"-v", "v0", "-repeat", "3"}, map[string]int{"invindex,v0,1,,": 3}},
		{"v3 records k", []string{"-v", "v3", "-p", "2", "-k", "16", "-repeat", "1"}, map[string]int{"invindex,v3,2,16,": 1}},
		{"v4 records merges", []string{"-v", "v4tree", "-p", "2", "-repeat", "2"}, map[string]int{"invindex,v4tree,2,,": 2, "invindex,v4tree-merge,2,,": 2}},
		{"tokenizer in version", []string{"-tokenizer", "fast", "-repeat", "1"}, map[string]int{"invindex,v0-fast,1,,": 1}},
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
			if lines[0] != strings.Join(bench.Header, ",") {
				t.Errorf("csv header = %q", lines[0])
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

func TestRunRejectsBadArgs(t *testing.T) {
	for _, args := range [][]string{
		{"-v", "v9"},
		{"-repeat", "0"},
		{"-p", "0"},
		{"-v", "v3", "-k", "0"},
		{"-tokenizer", "regex"},
		{"-in", filepath.Join(t.TempDir(), "missing")},
	} {
		if err := run(args, io.Discard); err == nil {
			t.Errorf("run(%q) succeeded, want error", args)
		}
	}
}
