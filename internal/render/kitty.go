package render

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Renderer selects native still-image graphics only on known direct terminals.
// Multiplexers retain the cell fallback unless the user explicitly overrides it.
func Renderer(request string, getenv func(string) string) (string, error) {
	if request == "cells" || request == "kitty" {
		return request, nil
	}
	if request != "auto" {
		return "", fmt.Errorf("unknown renderer %q; use auto, kitty, or cells", request)
	}
	if getenv("TMUX") != "" || getenv("STY") != "" {
		return "cells", nil
	}
	program := strings.ToLower(getenv("TERM_PROGRAM"))
	if program == "ghostty" || getenv("KITTY_WINDOW_ID") != "" || getenv("TERM") == "xterm-kitty" {
		return "kitty", nil
	}
	return "cells", nil
}

type Placement struct{ X, Y, Columns, Rows int }

// KittyWriter is the only terminal writer. It serializes Bubble Tea text and
// image commands, places images after text, and removes owned images on exit.
// Fd preserves Bubble Tea's TTY-size detection when using this output wrapper.
type KittyWriter struct {
	timer                     *time.Timer
	drawn                     Placement
	out                       io.Writer
	fd                        uintptr
	mu                        sync.Mutex
	png                       []byte
	key, sent                 int
	rect                      Placement
	visible, uploaded, closed bool
	imageID                   int
}

func NewKittyWriter(out *os.File) *KittyWriter {
	return &KittyWriter{out: out, fd: out.Fd(), imageID: os.Getpid()}
}
func (w *KittyWriter) Fd() uintptr { return w.fd }
func (w *KittyWriter) Set(png []byte, key int, rect Placement, visible bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.png = png
	w.key = key
	w.rect = rect
	w.visible = visible && len(png) > 0 && rect.Columns > 0 && rect.Rows > 0
	if !w.closed && w.timer == nil && w.needsDraw() {
		w.timer = time.AfterFunc(25*time.Millisecond, func() {
			w.mu.Lock()
			defer w.mu.Unlock()
			w.timer = nil
			if !w.closed && w.needsDraw() {
				_, _ = w.writeLocked(nil)
			}
		})
	}
}
func (w *KittyWriter) remove() string { return fmt.Sprintf("\x1b_Ga=d,d=I,i=%d,q=2\x1b\\", w.imageID) }
func (w *KittyWriter) needsDraw() bool {
	return (w.uploaded && (!w.visible || w.sent != w.key || w.drawn != w.rect)) || (w.visible && (!w.uploaded || w.sent != w.key || w.drawn != w.rect))
}
func (w *KittyWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writeLocked(p)
}
func (w *KittyWriter) writeLocked(p []byte) (int, error) {
	if bytes.Contains(p, []byte("\x1b[?1049h")) {
		w.closed = false
	}
	exiting := bytes.Contains(p, []byte("\x1b[?1049l"))
	if exiting {
		w.visible = false
		w.closed = true
	}
	var buf bytes.Buffer
	buf.WriteString("\x1b[?2026h")
	if w.uploaded && (!w.visible || w.sent != w.key) {
		buf.WriteString(w.remove())
		w.uploaded = false
	}
	buf.Write(p)
	if w.visible && !w.closed {
		if !w.uploaded {
			buf.WriteString(kittyUpload(w.png, w.imageID))
			w.uploaded = true
			w.sent = w.key
		}
		r := w.rect
		w.drawn = r
		// Save/restore cursor; C=1 prevents placement from advancing or scrolling it.
		fmt.Fprintf(&buf, "\x1b7\x1b[%d;%dH\x1b_Ga=p,i=%d,p=1,c=%d,r=%d,C=1,z=1,q=2\x1b\\\x1b8", r.Y+1, r.X+1, w.imageID, r.Columns, r.Rows)
	}
	buf.WriteString("\x1b[?2026l")
	if _, err := w.out.Write(buf.Bytes()); err != nil {
		return 0, err
	}
	return len(p), nil
}
func kittyUpload(png []byte, id int) string {
	data := base64.StdEncoding.EncodeToString(png)
	var b strings.Builder
	for offset := 0; offset < len(data); offset += 4096 {
		end := min(offset+4096, len(data))
		more := 0
		if end < len(data) {
			more = 1
		}
		if offset == 0 {
			fmt.Fprintf(&b, "\x1b_Ga=t,f=100,t=d,i=%d,q=2,m=%d;", id, more)
		} else {
			fmt.Fprintf(&b, "\x1b_Gm=%d;", more)
		}
		b.WriteString(data[offset:end])
		b.WriteString("\x1b\\")
	}
	return b.String()
}

func (w *KittyWriter) Read(p []byte) (int, error) {
	if r, ok := w.out.(io.Reader); ok {
		return r.Read(p)
	}
	return 0, io.EOF
}
func (w *KittyWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	if w.timer != nil {
		w.timer.Stop()
		w.timer = nil
	}
	if w.uploaded {
		w.uploaded = false
		_, err := io.WriteString(w.out, w.remove())
		return err
	}
	return nil
}
