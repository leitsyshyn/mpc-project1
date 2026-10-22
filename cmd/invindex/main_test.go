package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "corpus")
	if err := os.Mkdir(in, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{"a.txt": "Hello hello world", "b.txt": "World!"} {
		if err := os.WriteFile(filepath.Join(in, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out := filepath.Join(dir, "index.tsv")
	csv := filepath.Join(dir, "results.csv")

	prof := filepath.Join(dir, "cpu.pprof")
	args := []string{"-v", "v0", "-in", in, "-out", out, "-repeat", "3", "-csv", csv, "-cpuprofile", prof}
	var log bytes.Buffer
	if err := run(args, &log); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log.String(), "occurrences: 4") {
		t.Errorf("log does not report the 4 occurrences:\n%s", &log)
	}

	if info, err := os.Stat(prof); err != nil || info.Size() == 0 {
		t.Errorf("no CPU profile written: %v", err)
	}

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if want := "hello\t2\tdoc0:2\nworld\t2\tdoc0:1,doc1:1\n"; string(got) != want {
		t.Errorf("index =\n%s\nwant\n%s", got, want)
	}

	rows, err := os.ReadFile(csv)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(rows)), "\n")
	if lines[0] != "program,version,threads,k,run,seconds" {
		t.Errorf("csv header = %q", lines[0])
	}
	if len(lines) != 4 {
		t.Fatalf("csv has %d rows, want header + 3 runs:\n%s", len(lines), rows)
	}
	if !strings.HasPrefix(lines[1], "invindex,v0,1,,1,") {
		t.Errorf("csv row = %q, want invindex,v0,1,,1,<seconds>", lines[1])
	}

	// Only V3 has a K, so only V3 records one.
	if err := run([]string{"-v", "v3", "-p", "2", "-k", "16", "-in", in, "-out", out, "-repeat", "1", "-csv", csv}, io.Discard); err != nil {
		t.Fatal(err)
	}
	rows, err = os.ReadFile(csv)
	if err != nil {
		t.Fatal(err)
	}
	lines = strings.Split(strings.TrimSpace(string(rows)), "\n")
	if last := lines[len(lines)-1]; !strings.HasPrefix(last, "invindex,v3,2,16,1,") {
		t.Errorf("csv row = %q, want invindex,v3,2,16,1,<seconds>", last)
	}
	if got, _ := os.ReadFile(out); string(got) != "hello\t2\tdoc0:2\nworld\t2\tdoc0:1,doc1:1\n" {
		t.Errorf("v3 index =\n%s", got)
	}

	// V4 also records its merge phase, as a separate pseudo-version.
	if err := run([]string{"-v", "v4tree", "-p", "2", "-in", in, "-out", out, "-repeat", "2", "-csv", csv}, io.Discard); err != nil {
		t.Fatal(err)
	}
	rows, err = os.ReadFile(csv)
	if err != nil {
		t.Fatal(err)
	}
	// A non-default tokenizer is recorded in the version name.
	if err := run([]string{"-v", "v0", "-tokenizer", "fast", "-in", in, "-out", out, "-repeat", "1", "-csv", csv}, io.Discard); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(out); string(got) != "hello\t2\tdoc0:2\nworld\t2\tdoc0:1,doc1:1\n" {
		t.Errorf("index with the fast tokenizer =\n%s", got)
	}
	rows, err = os.ReadFile(csv)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rows), "\ninvindex,v0-fast,1,,1,") {
		t.Errorf("csv has no v0-fast row:\n%s", rows)
	}

	merges := strings.Count(string(rows), "\ninvindex,v4tree-merge,2,,")
	builds := strings.Count(string(rows), "\ninvindex,v4tree,2,,")
	if merges != 2 || builds != 2 {
		t.Errorf("csv has %d v4tree and %d v4tree-merge rows, want 2 each:\n%s", builds, merges, rows)
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
