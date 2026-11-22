package render

import (
	"bytes"
	"encoding/base64"
	"regexp"
	"strings"
	"testing"
)

func TestRendererSelection(t *testing.T) {
	for _, c := range []struct {
		env  map[string]string
		want string
	}{{map[string]string{"TERM_PROGRAM": "ghostty"}, "kitty"}, {map[string]string{"KITTY_WINDOW_ID": "1"}, "kitty"}, {map[string]string{"TERM_PROGRAM": "ghostty", "TMUX": "session"}, "cells"}, {map[string]string{}, "cells"}} {
		got, err := Renderer("auto", func(k string) string { return c.env[k] })
		if err != nil || got != c.want {
			t.Fatalf("%v: %s %v", c.env, got, err)
		}
	}
	if _, err := Renderer("invalid", func(string) string { return "" }); err == nil {
		t.Fatal("invalid renderer accepted")
	}
}
func TestKittyLifecycle(t *testing.T) {
	var out bytes.Buffer
	w := &KittyWriter{out: &out, imageID: 123}
	png := bytes.Repeat([]byte{1, 2, 3}, 5000)
	w.Set(png, 1, Placement{X: 5, Y: 3, Columns: 60, Rows: 20}, true)
	if _, err := w.Write([]byte("text")); err != nil {
		t.Fatal(err)
	}
	first := out.String()
	if !strings.HasPrefix(first, "text") || !strings.Contains(first, "\x1b[4;6H") || !strings.Contains(first, "c=60,r=20,C=1") {
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
	out.Reset()
	w.Set(nil, 2, Placement{}, false)
	w.Write([]byte("help"))
	if !strings.HasPrefix(out.String(), "\x1b_Ga=d,d=I,i=123") {
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
