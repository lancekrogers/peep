package tui

import (
	"context"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/lancekrogers/peep/internal/media"
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
