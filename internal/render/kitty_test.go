package render

import (
	"bytes"
	"encoding/base64"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestRendererSelection(t *testing.T) {
	for _, tt := range []struct{ request, want string }{{"auto", "cells"}, {"cells", "cells"}, {"kitty", "kitty"}} {
		got, err := Renderer(tt.request)
		if err != nil || got != tt.want {
			t.Fatalf("%s: %s %v", tt.request, got, err)
		}
	}
	if _, err := Renderer("invalid"); err == nil {
		t.Fatal("invalid renderer accepted")
	}
}
func TestKittyLifecycle(t *testing.T) {
	var out bytes.Buffer
	w := &KittyWriter{out: &out, imageID: 123}
	defer w.Close()
	png := bytes.Repeat([]byte{1, 2, 3}, 5000)
	w.Set(png, 1, Placement{X: 5, Y: 3, Columns: 60, Rows: 20}, true)
	if _, err := w.Write([]byte("text")); err != nil {
		t.Fatal(err)
	}
	if w.FrameRate() != 1 {
		t.Fatal("completed image upload not counted")
	}
	first := out.String()
	if strings.Index(first, "text") > strings.Index(first, "a=t") || !strings.Contains(first, "\x1b[4;6H") || !strings.Contains(first, "c=60,r=20,C=1") {
		t.Fatal("placement or ordering incorrect")
	}
	chunks := regexp.MustCompile("\x1b_G[^;]*;([^\x1b]*)\x1b\\\\").FindAllStringSubmatch(first, -1)
	encoded := ""
	for _, chunk := range chunks {
		if len(chunk[1]) > 4096 {
			t.Fatal("oversized chunk")
		}
		encoded += chunk[1]
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || !bytes.Equal(decoded, png) {
		t.Fatal("PNG transmission changed data")
	}
	out.Reset()
	w.Write([]byte("clock"))
	if strings.Contains(out.String(), "a=t") {
		t.Fatal("unchanged image uploaded again")
	}
	if w.FrameRate() != 1 {
		t.Fatal("text-only redraw counted as a video frame")
	}
	out.Reset()
	w.Set(nil, 2, Placement{}, false)
	w.Write([]byte("help"))
	if !strings.Contains(out.String(), "\x1b_Ga=d,d=I,i=123") {
		t.Fatal("image not cleared before overlay")
	}
	out.Reset()
	w.Set(png, 3, Placement{Columns: 20, Rows: 10}, true)
	w.Write([]byte("image"))
	out.Reset()
	w.Write([]byte("\x1b[?1049l"))
	if !strings.Contains(out.String(), "a=d") || strings.Contains(out.String(), "a=p") {
		t.Fatal("image survived alt-screen exit")
	}
}

func TestKittyRefreshWithoutTextChange(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	w := &KittyWriter{out: writer, imageID: 42}
	defer w.Close()
	done := make(chan string, 1)
	go func() { buf := make([]byte, 8192); n, _ := reader.Read(buf); done <- string(buf[:n]) }()
	w.Set([]byte("frame"), 1, Placement{Columns: 10, Rows: 5}, true)
	select {
	case got := <-done:
		if !strings.Contains(got, "a=t") || !strings.Contains(got, "a=p") {
			t.Fatal("frame was not drawn")
		}
	case <-time.After(time.Second):
		t.Fatal("native frame waited for a text redraw")
	}
	// Close the pipe before cleanup so deletion cannot wait for another reader.
	reader.Close()
}
