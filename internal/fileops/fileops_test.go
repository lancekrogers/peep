package fileops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, name, body string) Target {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	v, err := Capture(path)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func TestMoveNoOverwrite(t *testing.T) {
	target := fixture(t, "source.txt", "source")
	dest := filepath.Join(filepath.Dir(target.Path), "dest.txt")
	if err := os.WriteFile(dest, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Move(context.Background(), target, dest); err == nil {
		t.Fatal("overwrote destination")
	}
	b, _ := os.ReadFile(dest)
	if string(b) != "keep" {
		t.Fatal("destination changed")
	}
	if err := os.Remove(dest); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("absent", dest); err != nil {
		t.Fatal(err)
	}
	if err := Move(context.Background(), target, dest); err == nil {
		t.Fatal("replaced dangling symlink")
	}
	os.Remove(dest)
	if err := Move(context.Background(), target, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target.Path); !os.IsNotExist(err) {
		t.Fatal("old path retained")
	}
	b, _ = os.ReadFile(dest)
	if string(b) != "source" {
		t.Fatal("content lost")
	}
}
func TestDeleteAndChangedTarget(t *testing.T) {
	target := fixture(t, "file", "data")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Delete(ctx, target); err == nil {
		t.Fatal("ignored cancellation")
	}
	old := target.Path + ".old"
	os.Rename(target.Path, old)
	os.WriteFile(target.Path, []byte("replacement"), 0600)
	if err := Delete(context.Background(), target); err == nil {
		t.Fatal("deleted replacement")
	}
	link := target.Path + ".link"
	os.Symlink(target.Path, link)
	lt, err := Capture(link)
	if err != nil {
		t.Fatal(err)
	}
	if err := Delete(context.Background(), lt); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target.Path); err != nil {
		t.Fatal("followed symlink")
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "child"), []byte("keep"), 0600)
	dt, _ := Capture(dir)
	if err := Delete(context.Background(), dt); err == nil {
		t.Fatal("deleted nonempty directory")
	}
	if _, err := os.Stat(filepath.Join(dir, "child")); err != nil {
		t.Fatal("child lost")
	}
	os.Remove(filepath.Join(dir, "child"))
	if err := Delete(context.Background(), dt); err != nil {
		t.Fatal(err)
	}
}
func TestDestinationAndEditor(t *testing.T) {
	target := fixture(t, "name with spaces.txt", "text")
	if _, err := Destination(target, "../escape", true); err == nil {
		t.Fatal("rename allowed path")
	}
	dest, err := Destination(target, "../other.txt", false)
	if err != nil || dest != filepath.Join(filepath.Dir(filepath.Dir(target.Path)), "other.txt") {
		t.Fatal(dest, err)
	}
	folder := t.TempDir()
	dest, err = Destination(target, folder, false)
	if err != nil || dest != filepath.Join(folder, filepath.Base(target.Path)) {
		t.Fatal(dest, err)
	}
	t.Setenv("VISUAL", `editor --flag "argument with spaces"`)
	cmd, err := Editor(context.Background(), target.Path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cmd.Args) != 4 || cmd.Args[2] != "argument with spaces" || cmd.Args[3] != target.Path {
		t.Fatal(cmd.Args)
	}
	args, err := SplitEditor(`'path with spaces/editor' '$HOME' "$(touch bad)"`)
	if err != nil || len(args) != 3 || !strings.Contains(args[2], "$(") {
		t.Fatal(args, err)
	}
	if _, err := SplitEditor(`editor "unclosed`); err == nil {
		t.Fatal("accepted malformed editor")
	}
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "fallback-editor -f")
	cmd, err = Editor(context.Background(), target.Path)
	if err != nil || cmd.Args[0] != "fallback-editor" {
		t.Fatal(cmd, err)
	}
}
