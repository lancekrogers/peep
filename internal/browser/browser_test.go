package browser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRead(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"b.mp3", "a.PNG", ".hidden.jpg", "ignore.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "folder"), 0700); err != nil {
		t.Fatal(err)
	}
	entries, err := Read(dir, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 || !entries[0].Dir || entries[1].Name != "a.PNG" {
		t.Fatalf("unexpected listing: %+v", entries)
	}
	entries, err = Read(dir, true, "HIDDEN")
	if err != nil || len(entries) != 1 {
		t.Fatalf("filter/hidden: %+v %v", entries, err)
	}
	if _, err = Read(filepath.Join(dir, "missing"), false, ""); err == nil {
		t.Fatal("missing directory accepted")
	}
}
