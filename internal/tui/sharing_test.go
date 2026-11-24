package tui

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lancekrogers/peep/internal/browser"
	"github.com/lancekrogers/peep/internal/document"
)

func TestSharingUsesVisibleSelection(t *testing.T) {
	dir := t.TempDir()
	selected, active := filepath.Join(dir, "selected.txt"), filepath.Join(dir, "active.txt")
	for _, path := range []string{selected, active} {
		if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, mode := range []string{"browser", "queue", "active", "document"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel() // Inspect captured selection without touching any real clipboard.
			m := &Model{ctx: ctx, entries: []browser.Entry{{Name: "selected.txt", Path: selected}}}
			want := active
			switch mode {
			case "browser":
				want = selected
			case "queue":
				m.showQueue = true
				m.queue = []string{active}
			case "active":
				m.focus = true
				m.displayingActive = true
				m.active.Path = active
			case "document":
				m.focus = true
				m.doc = &document.Document{Path: active}
			}
			cmd := m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
			if cmd == nil {
				t.Fatal("no copy action")
			}
			if next := m.share("path"); next != nil {
				t.Fatal("duplicate clipboard job started")
			}
			msg := cmd().(sharingMsg)
			if msg.path != want {
				t.Fatalf("copied %q instead of %q", msg.path, want)
			}
			m.Update(msg)
			if m.copying {
				t.Fatal("clipboard job did not finish")
			}
		})
	}
}

func TestParentCannotBeShared(t *testing.T) {
	m := &Model{entries: []browser.Entry{{Name: "..", Path: t.TempDir()}}}
	if m.share("path") != nil {
		t.Fatal("synthetic parent shared")
	}
}
