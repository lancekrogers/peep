package desktop

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func testEnvironment(platform string, vars map[string]string, tools ...string) environment {
	return environment{platform, func(k string) string { return vars[k] }, func(name string) (string, error) {
		for _, tool := range tools {
			if name == tool {
				return "/bin/" + name, nil
			}
		}
		return "", os.ErrNotExist
	}}
}

func TestDesktopBackendSelection(t *testing.T) {
	for _, tc := range []struct {
		name, platform string
		vars           map[string]string
		tools          []string
		image          bool
		want           []string
	}{
		{"mac path", "darwin", nil, nil, false, []string{"pbcopy"}},
		{"mac image", "darwin", nil, nil, true, []string{"osascript", "-l", "JavaScript", "-e", macImage}},
		{"Wayland preferred", "linux", map[string]string{"WAYLAND_DISPLAY": "wayland-0", "DISPLAY": ":0"}, []string{"wl-copy", "xclip"}, true, []string{"/bin/wl-copy", "--type", "image/png"}},
		{"Wayland text", "linux", map[string]string{"WAYLAND_DISPLAY": "wayland-0"}, []string{"wl-copy"}, false, []string{"/bin/wl-copy", "--type", "text/plain;charset=utf-8"}},
		{"X11 image", "linux", map[string]string{"DISPLAY": ":0"}, []string{"xclip"}, true, []string{"/bin/xclip", "-selection", "clipboard", "-in", "-target", "image/png"}},
		{"X11 text", "linux", map[string]string{"DISPLAY": ":0"}, []string{"xclip"}, false, []string{"/bin/xclip", "-selection", "clipboard", "-in", "-target", "UTF8_STRING"}},
		{"xsel text", "linux", map[string]string{"DISPLAY": ":0"}, []string{"xsel"}, false, []string{"/bin/xsel", "--clipboard", "--input"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := testEnvironment(tc.platform, tc.vars, tc.tools...).clipboard(tc.image)
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("%q: %v", got, err)
			}
		})
	}
	for _, env := range []environment{
		testEnvironment("linux", nil, "xclip"),
		testEnvironment("linux", map[string]string{"DISPLAY": ":0"}, "xsel"),
		testEnvironment("linux", map[string]string{"WAYLAND_DISPLAY": "wayland-0", "DISPLAY": ":0"}, "xclip"),
		testEnvironment("darwin", map[string]string{"SSH_CONNECTION": "remote"}),
	} {
		if _, err := env.clipboard(true); err == nil {
			t.Fatal("unavailable clipboard accepted")
		}
	}
}

func TestDragPlansPreservePathAsData(t *testing.T) {
	path := "/tmp/-odd ' \" $()\n雪.png"
	for _, platform := range []string{"darwin", "linux"} {
		for _, helper := range []string{"ripdrag", "dragon-drop", "dragon"} {
			env := testEnvironment(platform, map[string]string{"WAYLAND_DISPLAY": "wayland-0"}, helper)
			args, err := env.drag(path)
			if err != nil || args[len(args)-1] != path {
				t.Fatal(args, err)
			}
			if platform == "linux" && !reflect.DeepEqual(args, []string{"/bin/" + helper, "--and-exit", path}) {
				t.Fatal(args)
			}
		}
	}
	for _, vars := range []map[string]string{nil, {"DISPLAY": ":0", "SSH_TTY": "remote"}} {
		if _, err := testEnvironment("linux", vars, "ripdrag").drag(path); err == nil {
			t.Fatal("no local display accepted")
		}
	}
	if _, err := testEnvironment("linux", map[string]string{"DISPLAY": ":0"}).drag(path); err == nil {
		t.Fatal("missing helper accepted")
	}
}

func TestCommandFailureAndCancellation(t *testing.T) {
	if err := command(context.Background(), []string{"sh", "-c", "echo deliberate >&2; exit 4"}, nil, nil); err == nil || !strings.Contains(err.Error(), "deliberate") {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := command(ctx, []string{"sleep", "30"}, nil, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	var b limitedBuffer
	b.limit = 3
	if _, err := b.Write([]byte("four")); err == nil || b.Len() != 0 {
		t.Fatal("output limit not enforced")
	}
}

func TestSharingRejectsInvalidSources(t *testing.T) {
	dir := t.TempDir()
	if err := CopyImage(context.Background(), dir); err == nil {
		t.Fatal("directory accepted as image")
	}
	if err := CopyPath(context.Background(), filepath.Join(dir, "missing")); err == nil {
		t.Fatal("missing path accepted")
	}
	if err := Drag(context.Background(), filepath.Join(dir, "missing")); err == nil {
		t.Fatal("missing drag source accepted")
	}
}

// Exercise Apple's PNG pasteboard API on a private named board. This test is
// safe on developer machines: it never reads or writes the user's clipboard.
func TestMacImagePasteboard(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("AppKit requires macOS")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	name := "peep-test-" + filepath.Base(t.TempDir()) + "-" + time.Now().Format("150405.000000000")
	var original bytes.Buffer
	if err := png.Encode(&original, image.NewRGBA(image.Rect(0, 0, 25, 10))); err != nil {
		t.Fatal(err)
	}
	script := "function run(argv) {" + strings.Replace(macImage, "$.NSPasteboard.generalPasteboard", "$.NSPasteboard.pasteboardWithName($(argv[0]))", 1) + "}"
	if err := command(ctx, []string{"osascript", "-l", "JavaScript", "-e", script, name}, bytes.NewReader(original.Bytes()), nil); err != nil {
		t.Fatal(err)
	}
	read := `ObjC.import('AppKit'); function run(argv) { var p=$.NSPasteboard.pasteboardWithName($(argv[0])); var d=p.dataForType($.NSPasteboardTypePNG); $.NSFileHandle.fileHandleWithStandardOutput.writeData(d); p.releaseGlobally; }`
	var got bytes.Buffer
	if err := command(ctx, []string{"osascript", "-l", "JavaScript", "-e", read, name}, nil, &got); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Bytes(), original.Bytes()) {
		t.Fatal("AppKit changed PNG data")
	}
}

// Runs on disposable CI desktop sessions, not the developer's clipboard.
func TestNativeClipboard(t *testing.T) {
	if os.Getenv("PEEP_DESKTOP_INTEGRATION") != "1" {
		t.Skip("requires an isolated desktop clipboard")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "quote ' dollar $ snow 雪\nimage.png")
	im := image.NewRGBA(image.Rect(0, 0, 2500, 20)) // Wider than peep's preview cap.
	im.Set(12, 7, color.RGBA{25, 110, 220, 255})
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, im); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if err := CopyPath(ctx, path); err != nil {
		t.Fatal(err)
	}
	read := []string{"pbpaste"}
	if runtime.GOOS == "linux" {
		read = []string{"xclip", "-selection", "clipboard", "-out"}
		if os.Getenv("WAYLAND_DISPLAY") != "" {
			read = []string{"wl-paste", "--no-newline"}
		}
	}
	got, err := exec.CommandContext(ctx, read[0], read[1:]...).Output()
	if err != nil || string(got) != path {
		t.Fatalf("clipboard path round trip failed: %v", err)
	}
	if err := CopyImage(ctx, path); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "darwin" {
		read = []string{"osascript", "-l", "JavaScript", "-e", `ObjC.import('AppKit'); var d=$.NSPasteboard.generalPasteboard.dataForType($.NSPasteboardTypePNG); $.NSFileHandle.fileHandleWithStandardOutput.writeData(d);`}
	} else if os.Getenv("WAYLAND_DISPLAY") != "" {
		read = []string{"wl-paste", "--type", "image/png"}
	} else {
		read = append(read, "-target", "image/png")
	}
	got, err = exec.CommandContext(ctx, read[0], read[1:]...).Output()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := png.Decode(bytes.NewReader(got))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds() != im.Bounds() || color.RGBAModel.Convert(decoded.At(12, 7)) != im.At(12, 7) {
		t.Fatal("clipboard changed image dimensions or pixels")
	}
}
