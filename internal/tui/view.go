package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/lancekrogers/peep/internal/media"
	"github.com/lancekrogers/peep/internal/render"
)

func stamp(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	s := int(seconds)
	return fmt.Sprintf("%02d:%02d", s/60, s%60)
}
func (m *Model) fileRows(w, h int, queue bool) []string {
	out := make([]string, h)
	cursor, count := m.cursor, len(m.entries)
	if queue {
		cursor = m.queueCursor
		count = len(m.queue)
	}
	offset := max(0, cursor-h+1)
	for y := 0; y < h; y++ {
		i := offset + y
		if i >= count {
			continue
		}
		name, kind := "", ""
		if queue {
			name = filepath.Base(m.queue[i])
			kind = string(media.Classify(name))
			if i == m.index {
				name = "▶ " + name
			}
		} else {
			e := m.entries[i]
			name = e.Name
			kind = string(e.Kind)
			if e.Dir {
				kind = "DIR"
				name += "/"
			}
			if e.Name == ".." {
				name = "../  Parent directory"
				kind = "BACK"
			}
		}
		marker := "  "
		if i == cursor {
			marker = "› "
		}
		nameWidth := max(1, w-9)
		row := marker + render.Fit(ansi.Truncate(render.Safe(name), nameWidth, "…"), nameWidth) + " " + color(render.Fit(kind, 5), muted)
		if !queue && m.entries[i].Name == ".." {
			row = marker + color("../ Parent directory", accent)
		}
		if i == cursor {
			row = fill(row, w, selectedBG)
		}
		out[y] = row
	}
	if count == 0 {
		label := "No media found"
		if m.filter != "" {
			label = "No matches · Esc clears search"
		}
		out[h/2] = color(center(label, w), muted)
	}
	return out
}
func (m *Model) previewRows(w, h int) []string {
	out := make([]string, h)
	if m.displayingActive && m.active.Kind == media.Audio {
		return m.spectrumRows(w, h)
	}
	if len(m.nativePNG) > 0 {
		return out
	}
	if len(m.rows) > 0 {
		offset := max(0, (h-len(m.rows))/2)
		for i, row := range m.rows {
			if i+offset < h {
				out[i+offset] = center(row, w)
			}
		}
		return out
	}
	title, detail, hint := "Choose a file", "Images · GIFs · video · music", "Enter opens a folder or plays a file"
	if e := m.selected(); !m.displayingActive && e != nil && e.Dir {
		title = "Browse this folder"
		detail = render.Safe(e.Name) + "/"
		hint = "Enter open  ·  Backspace parent"
	}
	if m.preview.Path != "" {
		title = render.Safe(filepath.Base(m.preview.Path))
		detail = "Loading preview…"
		hint = "Backspace parent  ·  Esc back"
	}
	if m.preview.Kind == media.Audio {
		title = "♪  " + render.Safe(m.preview.Title)
		if m.preview.Title == "" {
			title = "♪  " + render.Safe(filepath.Base(m.preview.Path))
		}
		detail = render.Safe(m.preview.Artist)
		hint = "Enter play  ·  Space pause  ·  [ / ] seek"
	}
	if m.previewError != "" {
		title = "Preview unavailable"
		detail = render.Safe(m.previewError)
		hint = "Select another file  ·  Backspace parent"
	}
	middle := max(0, h/2-2)
	for i, s := range []string{color(title, accent), "", color(detail, muted), "", color(hint, muted)} {
		if middle+i < h {
			out[middle+i] = center(s, w)
		}
	}
	return out
}
func (m *Model) helpRows(w, h int) []string {
	rows := []string{"", "  BROWSE                              PLAYBACK", "  ↑ ↓ / j k  select                  Space / p   play / pause", "  Enter      open folder / play       s           stop and reset", "  Backspace  parent directory         [ / ]       rewind / forward 5s", "  Esc        back / parent            { / }       rewind / forward 30s", "  /          filter                   t / Tab     focus timeline", "  H          hidden files             ← →         seek on timeline", "  f          focus preview            n / b       next / previous", "  v          now playing              z x c       slower / faster / reset", "  a          add to queue             - = m       volume / mute", "  Q          show queue               r           repeat", "", "  Click the timeline to seek. Click a transport button to control playback.", "  Selection previews silently. Transport controls the NOW PLAYING item.", "", "  ? / Esc close help     q / Ctrl+C quit"}
	return rows
}

type transportButton struct{ label, key string }

func (m *Model) buttons() []transportButton {
	play := "▶ Play"
	if m.state == "Playing" {
		play = "Ⅱ Pause"
	}
	return []transportButton{{"[  ◀◀ -5s", "["}, {play + "  Space", " "}, {"■ Stop  s", "s"}, {"+5s ▶▶  ]", "]"}}
}
func (m *Model) View() string {
	m.syncGraphics()
	w, h := m.width, m.height
	if w < 35 || h < 12 {
		return fmt.Sprintf("peep · %s\nResize to at least 35x12\nBackspace parent · Esc back\nSpace play/pause · q quit", m.state)
	}
	l := m.layout()
	lines := []string{}
	add := func(s string) { lines = append(lines, fill(s, w, bg)) }
	mode := "BROWSE"
	if m.focus {
		mode = "PREVIEW"
	}
	if m.showQueue {
		mode = "QUEUE"
	}
	if m.timeline {
		mode = "TIMELINE"
	}
	if m.filtering {
		add(" " + color("/ Search files & folders: ", accent) + render.Safe(m.filter) + color("▏   Enter keep · Esc clear", muted))
	} else {
		add(" " + color("◉ peep", accent) + "  " + badge(mode) + "  " + color("/ Search files & folders · Ctrl+F", fg) + "  " + color("? help · q quit", muted))
	}
	add(" " + color("‹ Backspace parent", accent) + "  " + render.Safe(m.dir))
	if m.help {
		for _, line := range box("Keyboard & mouse", m.helpRows(w-4, l.body), w-2, l.panelHeight, true) {
			add(" " + line)
		}
	} else {
		panes := [][]string{}
		if l.files > 0 {
			panes = append(panes, box(m.fileTitle(), m.fileRows(l.files-2, l.body, false), l.files, l.panelHeight, !m.timeline && !m.showQueue))
		}
		if l.preview > 0 {
			title := "Preview"
			if m.displayingActive {
				title = "Now Playing"
			}
			if m.preview.Path != "" {
				title += " · " + render.Safe(filepath.Base(m.preview.Path))
			}
			panes = append(panes, box(title, m.previewRows(l.preview-2, l.body), l.preview, l.panelHeight, m.focus))
		}
		if l.queue > 0 {
			panes = append(panes, box(fmt.Sprintf("Queue · %d", len(m.queue)), m.fileRows(l.queue-2, l.body, true), l.queue, l.panelHeight, !m.timeline))
		}
		for y := 0; y < l.panelHeight; y++ {
			row := " "
			for i, pane := range panes {
				if i > 0 {
					row += " "
				}
				row += pane[y]
			}
			add(row)
		}
	}
	name := "Select a file and press Enter"
	if m.active.Path != "" {
		name = render.Safe(filepath.Base(m.active.Path))
	}
	add(" " + color("NOW PLAYING", accent) + "  " + name + color(fmt.Sprintf("  %d/%d", max(0, m.index+1), len(m.queue)), muted))
	total := "--:--"
	if m.duration > 0 {
		total = stamp(m.duration)
	}
	audio := fmt.Sprintf("Vol %.0f%%", m.volume)
	if m.muted {
		audio = "Muted"
	}
	add(" " + color(fmt.Sprintf("[%s] %s / %s", m.state, stamp(m.position), total), accent) + color(fmt.Sprintf("    %.2gx · %s · Repeat %s", m.speed, audio, []string{"off", "one", "all"}[m.repeat]), muted))
	n := max(1, w-4)
	done := 0
	if m.duration > 0 {
		done = min(n, max(0, int(float64(n)*m.position/m.duration)))
	}
	bar := color(strings.Repeat("━", done), accent) + color(strings.Repeat("─", n-done), border)
	if m.timeline {
		bar = color("▸", accent) + bar
	} else {
		bar = " " + bar
	}
	add(" " + bar)
	controls := " "
	for _, b := range m.buttons() {
		controls += fill(" "+b.label+" ", len([]rune(b.label))+2, selectedBG) + "  "
	}
	add(controls)
	status := "Ready · / or Ctrl+F search files & folders · Enter open/play"
	if m.filter != "" || m.filtering {
		status = "Filter: " + render.Safe(m.filter)
	} else if m.notice != "" {
		status = render.Safe(m.notice)
	} else if m.timeline {
		status = "TIMELINE · ← rewind 5s · → forward 5s · Shift+arrows 30s · Esc back"
	}
	add(" " + color(status, muted))
	add(" " + color("↑↓ select   Enter open/play   Backspace parent   Esc back   / filter", fg))
	add(" " + color("f focus   B browser only   Q queue   t timeline   { / } seek 30s   ? all controls", muted))
	return strings.Join(lines, "\n")
}

func (m *Model) fileTitle() string {
	if m.filter != "" {
		return fmt.Sprintf("Matches · %d / %d", len(m.entries), len(m.allEntries))
	}
	return fmt.Sprintf("Files · %d", len(m.entries))
}
