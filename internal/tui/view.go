package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/lancekrogers/peep/internal/render"
)

func stamp(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	s := int(seconds)
	return fmt.Sprintf("%02d:%02d", s/60, s%60)
}
func (m *Model) View() string {
	w, h := m.width, m.height
	if w < 35 || h < 10 {
		return fmt.Sprintf("peep · %s\n%s\nResize to at least 35x10\nSpace play/pause · s stop · q quit", m.state, render.Safe(filepath.Base(m.active.Path)))
	}
	lines := []string{}
	add := func(s string) { lines = append(lines, render.Fit(s, w)) }
	add("\x1b[1;36m peep\x1b[0m  " + render.Safe(m.dir))
	if m.help {
		for _, line := range []string{"", "  BROWSE                          PLAYBACK", "  j/k or arrows  select           Space / p   play / pause", "  h/l            parent / folder  s           stop and reset", "  Enter          play selection   [ / ]       seek -/+5 seconds", "  /              filter names     { / }       seek -/+30 seconds", "  H              hidden files     0           seek to start", "  f              focus preview    n / b       next / previous", "  v              now playing      z / x / c   speed down/up/reset", "  a              append to queue  - / = / m   volume down/up/mute", "  Q              queue pane       r           repeat off/one/all", "  Queue: j/k select, Enter play   D           image hold 2/5/10/30s", "", "  Selection previews silently; Enter replaces the queue.", "  Timelines always control the named NOW PLAYING item.", "  q / Ctrl+C quit     ? / Esc close help"} {
			add(line)
		}
	} else {
		browse := !m.focus && w >= 90
		queue := m.showQueue && w >= 110
		left := 0
		right := 0
		if browse {
			left = 30
		}
		if queue {
			right = 28
		}
		content := w - left - right - 4
		body := h - 8
		title := "Preview"
		if m.displayingActive {
			title = "Now Playing"
		}
		label := title + ": " + render.Safe(filepath.Base(m.preview.Path))
		if m.preview.Path == "" {
			label = "Preview · select a media file"
		}
		if m.showQueue && !queue {
			label = "QUEUE · Enter play · d remove · Q close"
		}
		header := ""
		if browse {
			header = render.Fit(" FILES", left) + "│ "
		}
		header += render.Fit(label, content+2)
		if queue {
			header += "│ QUEUE"
		}
		add(header)
		offset := max(0, m.cursor-body+1)
		for y := 0; y < body; y++ {
			line := ""
			if browse {
				cell := ""
				i := offset + y
				if i < len(m.entries) {
					e := m.entries[i]
					prefix := "  "
					if i == m.cursor {
						prefix = "> "
					}
					name := render.Safe(e.Name)
					if e.Dir {
						name += "/"
					}
					cell = prefix + name
				}
				line = render.Fit(cell, left) + "│ "
			}
			cell := ""
			if y < len(m.rows) {
				cell = m.rows[y]
			} else if len(m.rows) == 0 {
				switch y {
				case 2:
					cell = "  " + string(m.preview.Kind)
				case 4:
					cell = "  " + render.Safe(m.preview.Title)
				case 5:
					cell = "  " + render.Safe(m.preview.Artist)
				case 6:
					cell = "  " + render.Safe(m.preview.Album)
				case 8:
					if m.preview.HasVisual {
						cell = "  Decoding preview..."
					} else {
						cell = "  Select a file · Enter to play"
					}
				}
			}
			if !browse && !m.focus && !m.showQueue && y < min(4, body) {
				i := max(0, m.cursor-3) + y
				cell = ""
				if i < len(m.entries) {
					prefix := "  "
					if i == m.cursor {
						prefix = "> "
					}
					cell = prefix + render.Safe(m.entries[i].Name)
				}
			}
			if m.showQueue && !queue {
				i := max(0, m.queueCursor-body+1) + y
				cell = ""
				if i < len(m.queue) {
					prefix := "  "
					if i == m.queueCursor {
						prefix = "> "
					}
					if i == m.index {
						prefix += "*"
					}
					cell = prefix + render.Safe(filepath.Base(m.queue[i]))
				}
			}
			line += render.Fit(cell, content+2)
			if queue {
				qoff := max(0, m.queueCursor-body+1)
				i := qoff + y
				q := ""
				if i < len(m.queue) {
					prefix := "  "
					if i == m.queueCursor {
						prefix = "> "
					}
					if i == m.index {
						prefix += "*"
					}
					q = prefix + render.Safe(filepath.Base(m.queue[i]))
				}
				line += "│ " + render.Fit(q, right-2)
			}
			add(line)
		}
	}
	for len(lines) < h-6 {
		add("")
	}
	if len(lines) > h-6 {
		lines = lines[:h-6]
	}
	add(strings.Repeat("─", w))
	name := "Nothing playing"
	if m.active.Path != "" {
		name = render.Safe(filepath.Base(m.active.Path))
	}
	add(fmt.Sprintf(" NOW PLAYING  %s   %d/%d", name, max(0, m.index+1), len(m.queue)))
	barWidth := max(5, min(35, w-55))
	filled := 0
	if m.duration > 0 {
		filled = min(barWidth, max(0, int(float64(barWidth)*m.position/m.duration)))
	}
	bar := strings.Repeat("━", filled) + strings.Repeat("─", barWidth-filled)
	total := "--:--"
	if m.duration > 0 {
		total = stamp(m.duration)
	}
	audio := fmt.Sprintf("Vol %.0f%%", m.volume)
	if m.muted {
		audio = "Muted"
	}
	add(fmt.Sprintf(" [%s] %s / %s  %s  %.2gx  %s", m.state, stamp(m.position), total, bar, m.speed, audio))
	status := fmt.Sprintf(" cells · Repeat %s · Image hold %.0fs", []string{"off", "one", "all"}[m.repeat], m.hold)
	if m.filter != "" || m.filtering {
		status = " Filter: " + render.Safe(m.filter)
	}
	if m.notice != "" {
		status = " " + render.Safe(m.notice)
	}
	add(status)
	add(" Space play/pause  s stop  [/] seek  Enter play  Q queue  ? help")
	add(" j/k browse  h/l folders  / filter  f focus  v now playing  q quit")
	return strings.Join(lines, "\n")
}
