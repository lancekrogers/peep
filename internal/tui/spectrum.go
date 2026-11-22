package tui

import (
	"context"
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/lancekrogers/peep/internal/media"
	"github.com/lancekrogers/peep/internal/render"
	"math"
	"path/filepath"
	"strings"
)

type spectrumMsg struct {
	id    int
	frame media.SpectrumFrame
	ok    bool
}

func (m *Model) clearSpectrum() {
	if m.spectrumCancel != nil {
		m.spectrumCancel()
	}
	m.spectrumID++
	m.spectrum = nil
	m.spectrumPending = nil
	m.bands = [media.SpectrumBands]float64{}
	m.bandTarget = m.bands
	m.spectrumError = ""
}
func (m *Model) startSpectrum() tea.Cmd {
	m.clearSpectrum()
	if m.active.Kind != media.Audio || m.state == "Stopped" || m.state == "Error" {
		return nil
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.spectrumCancel = cancel
	m.spectrum = media.Spectrum(ctx, m.active.Path, m.position)
	return m.nextSpectrum()
}
func (m *Model) nextSpectrum() tea.Cmd {
	ch, id := m.spectrum, m.spectrumID
	if ch == nil {
		return nil
	}
	return func() tea.Msg { f, ok := <-ch; return spectrumMsg{id, f, ok} }
}
func (m *Model) spectrumRows(w, h int) []string {
	out := make([]string, h)
	if h < 5 {
		return out
	}
	title := m.active.Title
	if title == "" {
		title = filepath.Base(m.active.Path)
	}
	barHeight := max(2, min(12, h-7))
	top := max(0, (h-barHeight-5)/2)
	out[top] = center(color("♪  "+render.Safe(title), accent), w)
	out[top+1] = center(color(render.Safe(m.active.Artist), muted), w)
	columns := max(1, min(media.SpectrumBands, w/3))
	barWidth := max(1, min(3, (w-columns)/columns))
	total := columns*(barWidth+1) - 1
	left := max(0, (w-total)/2)
	for row := 0; row < barHeight; row++ {
		var b strings.Builder
		b.WriteString(strings.Repeat(" ", left))
		for col := 0; col < columns; col++ {
			band := col * media.SpectrumBands / columns
			level := m.bands[band]
			if m.muted {
				level = 0
			} else {
				level *= m.volume / 100
			}
			lit := int(math.Ceil(level*float64(barHeight))) > barHeight-row-1
			symbol := "░"
			shade := border
			if lit {
				symbol = "█"
				shade = accent
				if row < barHeight/3 {
					shade = "245;181;98"
				} else if row < 2*barHeight/3 {
					shade = "98;190;242"
				}
			}
			b.WriteString(color(strings.Repeat(symbol, barWidth), shade))
			if col+1 < columns {
				b.WriteByte(' ')
			}
		}
		out[top+3+row] = b.String()
	}
	label := "60 Hz       AUDIO SPECTRUM       7 kHz"
	if m.state == "Paused" {
		label = "PAUSED · spectrum frozen"
	}
	if m.state == "Stopped" || m.state == "Ended" {
		label = fmt.Sprintf("%s · Space to play", m.state)
	}
	if m.spectrumError != "" {
		label = "Spectrum unavailable · audio playback continues"
	}
	if top+barHeight+3 < h {
		out[top+barHeight+3] = center(color(label, muted), w)
	}
	return out
}
