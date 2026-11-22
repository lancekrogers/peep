package tui

import (
	"context"
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/lancekrogers/peep/internal/media"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func imageModel() *Model {
	return &Model{ctx: context.Background(), state: "Playing", active: media.Info{Path: "image.png", Kind: media.Image}, duration: 5, position: 2, speed: 1, last: time.Now(), width: 100, height: 30}
}
func TestImageTransport(t *testing.T) {
	m := imageModel()
	m.toggle()
	if m.state != "Paused" {
		t.Fatal(m.state)
	}
	m.seek(30)
	if m.position != 5 || m.state != "Paused" {
		t.Fatalf("paused seek: %+v", m)
	}
	m.stop()
	if m.state != "Stopped" || m.position != 0 {
		t.Fatal("stop must reset")
	}
	m.toggle()
	if m.state != "Playing" {
		t.Fatal("resume failed")
	}
}
func TestStalePreview(t *testing.T) {
	m := imageModel()
	m.previewID = 3
	m.Update(previewMsg{id: 2, info: media.Info{Path: "wrong.png"}})
	if m.preview.Path != "" {
		t.Fatal("stale preview replaced selection")
	}
}
func TestFilterDoesNotQuit(t *testing.T) {
	m := imageModel()
	m.filtering = true
	m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m.filter != "q" {
		t.Fatal("shortcut intercepted text")
	}
}
func TestEmptyView(t *testing.T) {
	m := imageModel()
	for _, size := range [][2]int{{120, 35}, {60, 20}, {20, 5}} {
		m.width = size[0]
		m.height = size[1]
		if m.View() == "" {
			t.Fatal("empty view")
		}
	}
}

func TestBatchedFilterInput(t *testing.T) {
	m := imageModel()
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/video")})
	if !m.filtering || m.filter != "video" {
		t.Fatalf("batched input lost: %q", m.filter)
	}
}
func TestStoppedSeekPauses(t *testing.T) {
	m := imageModel()
	m.stop()
	m.seek(3)
	if m.state != "Paused" || m.position != 3 {
		t.Fatal("seeking from stopped must produce a paused position")
	}
}
func TestCancelledLoadCanRestart(t *testing.T) {
	m := imageModel()
	m.state = "Stopped"
	m.active = media.Info{Path: "loading.mp4"}
	m.queue = []string{"loading.mp4"}
	m.index = 0
	if cmd := m.toggle(); cmd == nil || m.state != "Loading" {
		t.Fatal("cancelled load did not restart")
	}
	m.loadCancel()
}

func TestPreviewFailureReplacesLoading(t *testing.T) {
	m := imageModel()
	m.previewID = 1
	m.preview = media.Info{Path: "bad.jpg", Kind: media.Image, HasVisual: true}
	m.Update(frameMsg{id: 1, ok: true, frame: media.Frame{Err: fmt.Errorf("no visual frame")}})
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "Preview unavailable") || strings.Contains(view, "Loading preview") {
		t.Fatal("error left loading state visible")
	}
}
func TestTimelineArrowsAndClick(t *testing.T) {
	m := imageModel()
	m.state = "Paused"
	m.duration = 20
	m.position = 10
	m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	m.key(tea.KeyMsg{Type: tea.KeyRight})
	if m.position != 15 {
		t.Fatal("right did not seek")
	}
	m.key(tea.KeyMsg{Type: tea.KeyLeft})
	if m.position != 10 {
		t.Fatal("left did not rewind")
	}
	m.mouse(tea.MouseMsg{X: 26, Y: m.height - 5, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	if m.position != 5 {
		t.Fatalf("timeline click: %v", m.position)
	}
	m.focus = true
	m.timeline = false
	m.key(tea.KeyMsg{Type: tea.KeyEsc})
	if m.focus {
		t.Fatal("Esc did not return to browser")
	}
}
func TestViewBoundsAndColor(t *testing.T) {
	for _, size := range [][2]int{{140, 40}, {110, 30}, {90, 25}, {70, 20}, {60, 20}, {35, 12}} {
		for _, mode := range []string{"browser", "focus", "queue", "help"} {
			m := imageModel()
			m.width = size[0]
			m.height = size[1]
			m.focus = mode == "focus"
			m.showQueue = mode == "queue"
			m.help = mode == "help"
			v := m.View()
			lines := strings.Split(v, "\n")
			if len(lines) != m.height {
				t.Fatalf("%v %s: %d rows", size, mode, len(lines))
			}
			for _, line := range lines {
				if ansi.StringWidth(line) != m.width {
					t.Fatalf("%v %s: width %d", size, mode, ansi.StringWidth(line))
				}
			}
			if !strings.Contains(v, "\x1b[48;2;") {
				t.Fatal("missing full-color panels")
			}
		}
	}
}
func TestParentNavigationRestoresSelection(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	m := imageModel()
	m.dir = child
	m.history = map[string]string{}
	cmd := m.navigate(root)
	m.Update(cmd())
	if m.selected() == nil || m.selected().Path != child {
		t.Fatal("parent navigation lost child selection")
	}
	if m.entries[0].Name != ".." {
		t.Fatal("visible parent row missing")
	}
}
