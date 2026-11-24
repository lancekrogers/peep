package tui

import (
	"context"
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lancekrogers/peep/internal/desktop"
)

type sharingMsg struct {
	action, path string
	err          error
}

func (m *Model) share(action string) tea.Cmd {
	if action == "drag" && m.dragCancel != nil {
		m.dragCancel()
		m.notice = "Closing drag window"
		return nil
	}
	if action != "drag" && m.copying {
		m.notice = "Clipboard operation in progress"
		return nil
	}
	var path string
	switch {
	case m.showQueue && m.queueCursor >= 0 && m.queueCursor < len(m.queue):
		path = m.queue[m.queueCursor]
	case m.focus && m.displayingActive:
		path = m.active.Path
	case m.focus && m.doc != nil:
		path = m.doc.Path
	default:
		if e := m.selected(); e != nil && e.Name != ".." {
			path = e.Path
		}
	}
	if path == "" {
		m.notice = "Select a file or folder to share"
		return nil
	}
	ctx := m.ctx
	if action == "drag" {
		ctx, m.dragCancel = context.WithCancel(ctx)
		m.notice = "Opening drag window · drag its icon into another app · Ctrl+G closes it"
	} else {
		m.copying = true
		m.notice = "Copying " + action + "…"
	}
	return func() tea.Msg {
		var err error
		switch action {
		case "path":
			err = desktop.CopyPath(ctx, path)
		case "image":
			err = desktop.CopyImage(ctx, path)
		case "drag":
			err = desktop.Drag(ctx, path)
		}
		return sharingMsg{action, path, err}
	}
}

func (m *Model) shared(v sharingMsg) {
	if v.action == "drag" {
		if m.dragCancel != nil {
			m.dragCancel()
		}
		m.dragCancel = nil
		m.notice = "Drag window closed"
	} else {
		m.copying = false
		m.notice = fmt.Sprintf("Copied %s · %s", v.action, displayPath(v.path))
	}
	if v.err != nil && !errors.Is(v.err, context.Canceled) {
		m.notice = v.err.Error()
	}
}
