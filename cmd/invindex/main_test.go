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

	args := []string{"-v", "v0", "-in", in, "-out", out, "-repeat", "3", "-csv", csv}
	var log bytes.Buffer
	if err := run(args, &log); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log.String(), "occurrences: 4") {
		t.Errorf("log does not report the 4 occurrences:\n%s", &log)
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
}

func TestRunRejectsBadArgs(t *testing.T) {
	for _, args := range [][]string{
		{"-v", "v9"},
		{"-repeat", "0"},
		{"-p", "0"},
		{"-in", filepath.Join(t.TempDir(), "missing")},
	} {
		if err := run(args, io.Discard); err == nil {
			t.Errorf("run(%q) succeeded, want error", args)
		}
	}
}
