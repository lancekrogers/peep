// Package desktop bridges explicit file-sharing actions to the local desktop.
package desktop

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/lancekrogers/peep/internal/media"
)

//go:embed drag_macos.js
var macDrag string

// Read binary PNG data from stdin, never interpolate a filename into script code.
const macImage = `ObjC.import('AppKit');
var data = $.NSFileHandle.fileHandleWithStandardInput.readDataToEndOfFile;
if (!data.length) throw Error('No image data');
var board = $.NSPasteboard.generalPasteboard;
board.clearContents;
if (!board.setDataForType(data, $.NSPasteboardTypePNG)) throw Error('Clipboard rejected image');`

type environment struct {
	platform string
	getenv   func(string) string
	lookup   func(string) (string, error)
}

func local() environment { return environment{runtime.GOOS, os.Getenv, exec.LookPath} }

func (e environment) clipboard(image bool) ([]string, error) {
	if e.getenv("SSH_CONNECTION") != "" || e.getenv("SSH_TTY") != "" {
		return nil, fmt.Errorf("desktop clipboard is local to this machine; use peep in a local desktop terminal")
	}
	if e.platform == "darwin" {
		if image {
			return []string{"osascript", "-l", "JavaScript", "-e", macImage}, nil
		}
		return []string{"pbcopy"}, nil
	}
	if e.platform == "linux" {
		mime := "text/plain;charset=utf-8"
		if image {
			mime = "image/png"
		}
		if e.getenv("WAYLAND_DISPLAY") != "" {
			if path, err := e.lookup("wl-copy"); err == nil {
				return []string{path, "--type", mime}, nil
			}
			return nil, fmt.Errorf("install wl-clipboard for copying on Wayland")
		}
		if e.getenv("DISPLAY") != "" {
			if path, err := e.lookup("xclip"); err == nil {
				target := "UTF8_STRING"
				if image {
					target = "image/png"
				}
				return []string{path, "-selection", "clipboard", "-in", "-target", target}, nil
			}
			if !image {
				if path, err := e.lookup("xsel"); err == nil {
					return []string{path, "--clipboard", "--input"}, nil
				}
			}
			return nil, fmt.Errorf("install xclip for copying on X11")
		}
		return nil, fmt.Errorf("no desktop display; copying requires a local Wayland or X11 session")
	}
	return nil, fmt.Errorf("desktop clipboard is unavailable on %s", e.platform)
}

func absolute(path string) (string, os.FileInfo, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", nil, fmt.Errorf("share file: %w", err)
	}
	return path, info, nil
}

func CopyPath(ctx context.Context, path string) error {
	path, _, err := absolute(path)
	if err != nil {
		return err
	}
	args, err := local().clipboard(false)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return command(ctx, args, strings.NewReader(path), nil)
}

// CopyImage copies full-resolution pixels, not the downscaled terminal preview.
// Animated images contribute their first frame. The original is never changed.
func CopyImage(ctx context.Context, path string) error {
	path, info, err := absolute(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || (media.Classify(path) != media.Image && media.Classify(path) != media.GIF) {
		return fmt.Errorf("select an image file to copy its pixels")
	}
	args, err := local().clipboard(true)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var png limitedBuffer
	png.limit = 64 << 20
	if err := command(ctx, []string{"ffmpeg", "-v", "error", "-nostdin", "-threads", "1", "-i", path,
		"-an", "-sn", "-frames:v", "1", "-threads", "1", "-f", "image2pipe", "-c:v", "png", "pipe:1"}, nil, &png); err != nil {
		return fmt.Errorf("copy image (run peep setup if FFmpeg is missing): %w", err)
	}
	if png.Len() == 0 {
		return fmt.Errorf("image decoder produced no pixels")
	}
	return command(ctx, args, bytes.NewReader(png.Bytes()), nil)
}

func (e environment) drag(path string) ([]string, error) {
	if e.getenv("SSH_CONNECTION") != "" || e.getenv("SSH_TTY") != "" {
		return nil, fmt.Errorf("dragging requires a local desktop session; remote files are not transferred")
	}
	if e.platform == "darwin" {
		return []string{"osascript", "-l", "JavaScript", "-e", macDrag, path}, nil
	}
	if e.platform == "linux" {
		if e.getenv("WAYLAND_DISPLAY") == "" && e.getenv("DISPLAY") == "" {
			return nil, fmt.Errorf("dragging requires a local Wayland or X11 session")
		}
		for _, name := range []string{"ripdrag", "dragon-drop", "dragon"} {
			if helper, err := e.lookup(name); err == nil {
				return []string{helper, "--and-exit", path}, nil
			}
		}
		return nil, fmt.Errorf("install ripdrag or dragon-drop (dragon on Arch) to open a drag window")
	}
	return nil, fmt.Errorf("desktop dragging is unavailable on %s", e.platform)
}

// Drag waits until the user drops the file or closes the window. Cancellation
// closes the helper; stdout/stderr are never allowed to corrupt the TUI.
func Drag(ctx context.Context, path string) error {
	path, info, err := absolute(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() && !info.IsDir() {
		return fmt.Errorf("only files and folders can be dragged")
	}
	args, err := local().drag(path)
	if err != nil {
		return err
	}
	return command(ctx, args, nil, nil)
}

type limitedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		return 0, fmt.Errorf("output exceeds %d MiB limit", b.limit>>20)
	}
	return b.Buffer.Write(p)
}

func command(ctx context.Context, args []string, in io.Reader, out io.Writer) error {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdin, cmd.Stdout = in, out
	// Clipboard helpers fork persistent selection owners. Give their diagnostics
	// a real file descriptor rather than an os/exec pipe that the child can keep
	// open after the launcher exits (which would hang Cmd.Wait).
	cmd.WaitDelay = time.Second
	stderr, err := os.CreateTemp("", "peep-sharing-errors-*")
	if err != nil {
		return err
	}
	defer os.Remove(stderr.Name())
	defer stderr.Close()
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("%s: %w", filepath.Base(args[0]), ctx.Err())
		}
		var detail [300]byte
		n, _ := stderr.ReadAt(detail[:], 0)
		return fmt.Errorf("%s: %w: %s", filepath.Base(args[0]), err, strings.TrimSpace(string(detail[:n])))
	}
	return nil
}
