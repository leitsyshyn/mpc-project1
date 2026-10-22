package index

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Load reads every regular file under dir, skipping hidden entries. Document i
// is the i-th file in lexical walk order.
func Load(dir string) ([][]byte, error) {
	var texts [][]byte
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
		texts = append(texts, text)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return texts, nil
}
