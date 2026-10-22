package index

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"b.txt":        "bravo",
		"a.txt":        "alpha",
		"sub/c.txt":    "charlie",
		"sub/deep/d.x": "delta",
		".DS_Store":    "hidden files are not documents",
		".git/HEAD":    "neither are hidden directories",
	}
	for name, text := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	texts, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, text := range texts {
		got = append(got, string(text))
	}
	if want := []string{"alpha", "bravo", "charlie", "delta"}; !slices.Equal(got, want) {
		t.Errorf("Load = %q, want %q", got, want)
	}
}

func TestLoadMissingDir(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("Load of a missing directory succeeded, want error")
	}
}
