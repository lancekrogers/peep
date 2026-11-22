package tui

import (
	"context"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lancekrogers/peep/internal/media"
	"github.com/lancekrogers/peep/internal/player"
)

func (m *Model) key(k tea.KeyMsg) tea.Cmd {
	key := k.String()
	if key == "ctrl+c" || (!m.filtering && key == "q") {
		m.clearSpectrum()
		if m.previewCancel != nil {
			m.previewCancel()
		}
		if m.loadCancel != nil {
			m.loadCancel()
		}
		return tea.Quit
	}
	if m.filtering {
		switch key {
		case "esc":
			m.filtering = false
			m.filter = ""
			return m.refilter()
		case "enter":
			m.filtering = false
			return m.resizeFrame()
		case "backspace":
			r := []rune(m.filter)
			if len(r) > 0 {
				m.filter = string(r[:len(r)-1])
			}
		default:
			if len(k.Runes) > 0 {
				m.filter += string(k.Runes)
			}
		}
		return m.refilter()
	}
	if m.help {
		if key == "?" || key == "esc" {
			m.help = false
		}
		return nil
	}
	if m.timeline {
		switch key {
		case "left", "h":
			return m.seek(m.position - 5)
		case "right", "l":
			return m.seek(m.position + 5)
		case "shift+left":
			return m.seek(m.position - 30)
		case "shift+right":
			return m.seek(m.position + 30)
		case "home":
			return m.seek(0)
		case "end":
			return m.seek(m.duration)
		}
	}
	switch key {
	case "tab", "shift+tab", "t":
		m.timeline = !m.timeline
		return nil
	case "?":
		m.help = true
	case "esc":
		if m.timeline {
			m.timeline = false
			return nil
		}
		if m.showQueue {
			m.showQueue = false
		} else if m.focus {
			m.focus = false
			return m.previewSelection()
		} else if m.filter != "" {
			m.filter = ""
			return m.refilter()
		} else {
			return m.navigate(filepath.Dir(m.dir))
		}
		return m.resizeFrame()
	case "f":
		m.timeline = false
		m.focus = !m.focus
		return m.resizeFrame()
	case "Q":
		m.timeline = false
		m.showQueue = !m.showQueue
		return m.resizeFrame()
	case "v":
		return m.showActive()
	case "/", "ctrl+f":
		m.focus = false
		m.showQueue = false
		m.timeline = false
		m.filtering = true
	case "B":
		m.browserOnly = !m.browserOnly
		if m.browserOnly {
			m.focus = false
		}
		m.timeline = false
		return m.resizeFrame()
	case "H":
		m.hidden = !m.hidden
		return m.readDir()
	case "j", "down", "k", "up":
		delta := 1
		if key == "k" || key == "up" {
			delta = -1
		}
		if m.showQueue {
			m.queueCursor = max(0, min(len(m.queue)-1, m.queueCursor+delta))
			return nil
		}
		m.cursor = max(0, min(len(m.entries)-1, m.cursor+delta))
		return m.previewSelection()
	case "home", "end":
		if key == "home" {
			m.cursor = 0
		} else {
			m.cursor = max(0, len(m.entries)-1)
		}
		return m.previewSelection()
	case "h", "left", "backspace":
		return m.navigate(filepath.Dir(m.dir))
	case "l", "right":
		if e := m.selected(); e != nil && e.Dir {
			return m.navigate(e.Path)
		}
	case "enter":
		if m.busy {
			return nil
		}
		if m.showQueue {
			return m.load(m.queueCursor)
		}
		if e := m.selected(); e != nil {
			if e.Dir {
				return m.navigate(e.Path)
			}
			m.browserOnly = false
			return m.queueFromSelection()
		}
	case "a":
		if e := m.selected(); e != nil && !e.Dir {
			m.queue = append(m.queue, e.Path)
			m.notice = "Added to queue"
		}
	case "d":
		if m.showQueue && !m.busy && len(m.queue) > 0 {
			i := m.queueCursor
			if i == m.index {
				m.notice = "Stop and choose another item before removing the active entry"
				return nil
			}
			m.queue = append(m.queue[:i], m.queue[i+1:]...)
			if i < m.index {
				m.index--
			}
			m.queueCursor = max(0, min(i, len(m.queue)-1))
		}
	case " ", "p":
		return m.toggle()
	case "s":
		return m.stop()
	case "n":
		if m.index+1 < len(m.queue) {
			return m.loadState(m.index+1, m.navigationState())
		} else if m.repeat == 2 {
			return m.loadState(0, m.navigationState())
		}
	case "b":
		if m.index > 0 {
			return m.loadState(m.index-1, m.navigationState())
		} else if m.repeat == 2 {
			return m.loadState(len(m.queue)-1, m.navigationState())
		}
	case "[":
		return m.seek(m.position - 5)
	case "]":
		return m.seek(m.position + 5)
	case "{":
		return m.seek(m.position - 30)
	case "}":
		return m.seek(m.position + 30)
	case "0":
		return m.seek(0)
	case "r":
		m.repeat = (m.repeat + 1) % 3
	case "z", "x", "c":
		if m.busy {
			return nil
		}
		speed := m.speed
		if key == "c" {
			speed = 1
		} else if key == "z" {
			speed = max(0.25, speed/2)
		} else {
			speed = min(4, speed*2)
		}
		m.speed = speed
		if m.backend != nil {
			return m.control(func(ctx context.Context, p *player.Player) error {
				return p.Command(ctx, "set_property", "speed", speed)
			}, nil)
		}
	case "-", "=", "m":
		if m.busy {
			return nil
		}
		if key == "m" {
			m.muted = !m.muted
		} else if key == "-" {
			m.volume = max(0, m.volume-5)
		} else {
			m.volume = min(100, m.volume+5)
		}
		vol, muted := m.volume, m.muted
		return m.control(func(ctx context.Context, p *player.Player) error {
			if err := p.Command(ctx, "set_property", "volume", vol); err != nil {
				return err
			}
			return p.Command(ctx, "set_property", "mute", muted)
		}, nil)
	case "D":
		values := []float64{2, 5, 10, 30}
		for i, d := range values {
			if m.hold == d {
				m.hold = values[(i+1)%len(values)]
				break
			}
		}
		if m.active.Kind == media.Image {
			m.duration = m.hold
			m.position = min(m.position, m.duration)
		}
	}
	return nil
}
func (m *Model) navigate(dir string) tea.Cmd {
	m.focus = false
	m.showQueue = false
	m.timeline = false
	if e := m.selected(); e != nil {
		m.history[m.dir] = e.Path
	}
	if filepath.Dir(m.dir) == dir {
		m.history[dir] = m.dir
	}
	m.dir = dir
	m.cursor = 0
	m.filter = ""
	return m.readDir()
}
func (m *Model) resizeFrame() tea.Cmd {
	if m.displayingActive {
		return m.showActive()
	}
	return m.previewSelection()
}
func (m *Model) toggle() tea.Cmd {
	if m.busy {
		return nil
	}
	if m.state == "Empty" {
		return m.queueFromSelection()
	}
	if m.state == "Error" || m.state == "Ended" || (m.state == "Stopped" && m.active.Kind == "") {
		return m.load(m.index)
	}
	wasStopped := m.state == "Stopped"
	paused := m.state == "Playing"
	if paused {
		m.state = "Paused"
	} else {
		m.state = "Playing"
		m.last = time.Now()
	}
	if m.active.Kind == media.Image {
		return nil
	}
	cmd := m.control(func(ctx context.Context, p *player.Player) error { return p.Pause(ctx, paused) }, nil)
	if wasStopped && m.active.Kind == media.Audio {
		return tea.Batch(cmd, m.startSpectrum())
	}
	return cmd
}
func (m *Model) stop() tea.Cmd {
	if m.busy && m.state != "Loading" {
		m.stopPending = true
		return nil
	}
	m.clearSpectrum()
	if m.state == "Empty" {
		return nil
	}
	if m.state == "Loading" {
		if m.loadCancel != nil {
			m.loadCancel()
		}
		m.loadID++
		m.busy = false
		m.state = "Stopped"
		m.position = 0
		if m.backend != nil {
			return m.control(func(ctx context.Context, p *player.Player) error { return p.Command(ctx, "stop") }, nil)
		}
		return nil
	}
	if m.busy {
		return nil
	}
	m.state = "Stopped"
	return m.seek(0)
}
func (m *Model) seek(pos float64) tea.Cmd {
	if m.busy || m.active.Path == "" || m.state == "Error" {
		return nil
	}
	if m.duration <= 0 {
		m.notice = "Duration unknown; seeking unavailable"
		return nil
	}
	pos = max(0, min(pos, m.duration))
	if m.state == "Ended" || (m.state == "Stopped" && pos > 0) {
		m.state = "Paused"
	}
	if m.active.Kind == media.Image {
		m.position = pos
		return nil
	}
	stopped := m.state == "Stopped"
	return m.control(func(ctx context.Context, p *player.Player) error {
		if stopped {
			if err := p.Pause(ctx, true); err != nil {
				return err
			}
		}
		return p.Seek(ctx, pos)
	}, &pos)
}

func (m *Model) navigationState() string {
	if m.state == "Paused" || m.state == "Stopped" {
		return m.state
	}
	return "Playing"
}
