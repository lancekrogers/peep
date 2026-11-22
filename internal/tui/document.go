package tui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lancekrogers/peep/internal/document"
)

type documentMsg struct {
	id     int
	doc    *document.Document
	layout document.Layout
	anchor int
	err    error
}

func (m *Model) clearDocument() {
	m.doc = nil
	m.docLayout = document.Layout{}
	m.docOffset = 0
	m.docSource = false
}
func (m *Model) documentWidth() int {
	w := m.layout().preview - 2
	if w < 4 {
		w = m.width - 4
	}
	return max(4, w)
}
func (m *Model) documentAnchor() int {
	if len(m.docLayout.Lines) == 0 {
		return 0
	}
	return m.docLayout.Lines[min(m.docOffset, len(m.docLayout.Lines)-1)].SourceLine
}
func (m *Model) readDocument(path string) tea.Cmd {
	id, width := m.previewID, m.documentWidth()
	ctx, cancel := context.WithTimeout(m.ctx, 10*time.Second)
	m.previewCancel = cancel
	return func() tea.Msg {
		defer cancel()
		d, err := document.Read(ctx, path)
		var layout document.Layout
		if err == nil {
			layout, err = document.Render(ctx, d, width, false)
		}
		return documentMsg{id, d, layout, 0, err}
	}
}
func (m *Model) reflowDocument() tea.Cmd {
	if m.doc == nil {
		return nil
	}
	if m.previewCancel != nil {
		m.previewCancel()
	}
	m.previewID++
	id, d, width, raw, anchor := m.previewID, m.doc, m.documentWidth(), m.docSource, m.documentAnchor()
	ctx, cancel := context.WithTimeout(m.ctx, 10*time.Second)
	m.previewCancel = cancel
	return func() tea.Msg {
		defer cancel()
		l, e := document.Render(ctx, d, width, raw)
		return documentMsg{id, d, l, anchor, e}
	}
}
func (m *Model) applyDocument(v documentMsg) {
	if v.err != nil {
		m.previewError = v.err.Error()
		return
	}
	m.doc = v.doc
	m.docLayout = v.layout
	m.docOffset = 0
	for i, line := range v.layout.Lines {
		if line.SourceLine > v.anchor {
			break
		}
		if i == 0 || v.layout.Lines[i-1].SourceLine != line.SourceLine {
			m.docOffset = i
		}
	}
	m.scrollDocument(0)
}
func (m *Model) scrollDocument(delta int) {
	m.docOffset = max(0, min(max(0, len(m.docLayout.Lines)-m.layout().body), m.docOffset+delta))
}
func (m *Model) documentRows(w, h int) []string {
	rows := make([]string, h)
	for i := range rows {
		at := m.docOffset + i
		if at < len(m.docLayout.Lines) {
			rows[i] = m.docLayout.Lines[at].Text
		}
	}
	return rows
}
func (m *Model) documentTitle() string {
	mode := "Text"
	if m.doc.Kind == "MD" {
		mode = "Markdown"
		if m.docSource {
			mode = "Source"
		}
	}
	return fmt.Sprintf("%s · %d/%d", mode, min(m.docOffset+1, len(m.docLayout.Lines)), len(m.docLayout.Lines))
}
