package index

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Doc is one corpus file.
type Doc struct {
	Name string // path relative to the corpus directory, slash-separated
	Text []byte
}

// Load reads every regular file under dir into memory, recursing into
// subdirectories and skipping hidden entries. Documents are numbered in walk
// (lexical) order.
func Load(dir string) ([]Doc, error) {
	var docs []Doc
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case path != dir && strings.HasPrefix(d.Name(), "."):
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		case !d.Type().IsRegular():
			return nil
		}
		text, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		docs = append(docs, Doc{filepath.ToSlash(rel), text})
		return err
	})
	return docs, err
}
