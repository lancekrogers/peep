package browser

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lancekrogers/peep/internal/media"
)

type Entry struct {
	Path, Name string
	Dir        bool
	Kind       media.Kind
}

func Read(path string, hidden bool, filter string) ([]Entry, error) {
	dirs, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}
	entries := []Entry{}
	for _, d := range dirs {
		if !hidden && strings.HasPrefix(d.Name(), ".") {
			continue
		}
		if !strings.Contains(strings.ToLower(d.Name()), strings.ToLower(filter)) {
			continue
		}
		full := filepath.Join(path, d.Name())
		dir := d.IsDir()
		if d.Type()&os.ModeSymlink != 0 {
			if stat, e := os.Stat(full); e == nil {
				dir = stat.IsDir()
			}
		}
		kind := media.Classify(full)
		if !dir && kind == "" {
			continue
		}
		entries = append(entries, Entry{Path: full, Name: d.Name(), Dir: dir, Kind: kind})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Dir != entries[j].Dir {
			return entries[i].Dir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries, nil
}
