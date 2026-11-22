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

func TestFilterFoldersAndFiles(t *testing.T) {
	entries := []Entry{{Name: "20261114-222940-work-however-you-like", Dir: true}, {Name: "work.flac"}, {Name: "walk-home.mp4"}, {Name: "unrelated.png"}}
	got := Filter(entries, "WORK")
	if len(got) != 2 || got[0].Name != "work.flac" {
		t.Fatalf("substring priority: %+v", got)
	}
	got = Filter(entries, "wrk hwevr")
	if len(got) != 1 || !got[0].Dir {
		t.Fatalf("fuzzy directory search: %+v", got)
	}
	if got := Filter(entries, "zzzz"); len(got) != 0 {
		t.Fatal("unmatched filter returned entries")
	}
	if len(Filter(entries, "")) != len(entries) {
		t.Fatal("clearing filter lost entries")
	}
}
