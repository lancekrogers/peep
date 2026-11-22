package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/lancekrogers/peep/internal/browser"
	"os"
	"path/filepath"
	"testing"
)

func TestFileConfirmationAndCancel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "quit.txt")
	os.WriteFile(path, []byte("keep"), 0600)
	m := imageModel()
	m.dir = dir
	m.entries = []browser.Entry{{Name: "quit.txt", Path: path}}
	m.beginFileOperation("delete")
	if m.filePrompt == nil {
		t.Fatal("missing confirmation")
	}
	if cmd := m.key(tea.KeyMsg{Type: tea.KeyEnter}); cmd != nil {
		t.Fatal("empty confirmation executed")
	}
	m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m.filePrompt.input != "q" {
		t.Fatal("q did not type into confirmation")
	}
	m.key(tea.KeyMsg{Type: tea.KeyEsc})
	if m.filePrompt != nil {
		t.Fatal("cancel failed")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("cancel deleted file")
	}
	m.beginFileOperation("delete")
	m.filePrompt.input = "quit.txt"
	cmd := m.key(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("confirmed delete did nothing")
	}
	msg := cmd().(fileOperationMsg)
	if msg.err != nil {
		t.Fatal(msg.err)
	}
	m.applyFileOperation(msg)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("file retained")
	}
	m.entries = []browser.Entry{{Name: "..", Path: dir, Dir: true}}
	m.beginFileOperation("delete")
	if m.filePrompt != nil {
		t.Fatal("parent can be deleted")
	}
}

func TestMoveUpdatesQueuePaths(t *testing.T) {
	m := imageModel()
	m.queue = []string{"/folder/one.mp3", "/folder/two.mp3", "/elsewhere/three.mp3"}
	m.active.Path = "/folder/one.mp3"
	m.state = "Stopped"
	m.index = 0
	m.applyFileOperation(fileOperationMsg{from: "/folder", to: "/renamed", operation: "move"})
	if m.active.Path != "/renamed/one.mp3" || m.queue[1] != "/renamed/two.mp3" || m.queue[2] != "/elsewhere/three.mp3" {
		t.Fatal(m.queue, m.active.Path)
	}
	m.applyFileOperation(fileOperationMsg{from: "/renamed/one.mp3", operation: "delete"})
	if len(m.queue) != 2 || m.active.Path != "" || m.state != "Empty" || m.index != -1 {
		t.Fatal(m.queue, m.state, m.index)
	}
}
