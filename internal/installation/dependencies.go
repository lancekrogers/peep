package installation

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func EnsureMedia(ctx context.Context, in io.Reader, out, stderr io.Writer) error {
	return ensureMedia(ctx, in, out, stderr, false, runtime.GOOS, os.Geteuid() == 0, exec.LookPath)
}

// SetupMedia installs dependencies only, leaving package-manager-owned peep
// executables alone. Interactive setup defaults to cancel; --yes is explicit.
func SetupMedia(ctx context.Context, in io.Reader, out, stderr io.Writer, yes bool) error {
	return ensureMedia(ctx, in, out, stderr, !yes, runtime.GOOS, os.Geteuid() == 0, exec.LookPath)
}

// Tools are the external programs behind media playback and PDF preview.
var Tools = []string{"ffmpeg", "ffprobe", "mpv", "pdfinfo", "pdftoppm", "pdftotext"}

func ensureMedia(ctx context.Context, in io.Reader, out, stderr io.Writer, confirm bool, platform string, root bool, lookup func(string) (string, error)) error {
	var missing []string
	for _, name := range Tools {
		if _, err := lookup(name); err != nil {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		fmt.Fprintln(out, "Preview tools are ready: "+strings.Join(Tools, ", "))
		return nil
	}
	plan, err := dependencyPlan(platform, root, lookup)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Missing preview tools: %s\n", strings.Join(missing, ", "))
	for _, args := range plan {
		fmt.Fprintln(out, "  "+strings.Join(args, " "))
	}
	if confirm {
		fmt.Fprint(out, "Install these dependencies? [y/N] ")
		answer, err := bufio.NewReader(in).ReadString('\n')
		if err != nil && err != io.EOF {
			return fmt.Errorf("read setup confirmation: %w", err)
		}
		answer = strings.ToLower(strings.TrimSpace(answer))
		if err != nil || (answer != "y" && answer != "yes") {
			fmt.Fprintln(out, "Setup cancelled; nothing installed")
			return nil
		}
	}
	for _, args := range plan {
		fmt.Fprintf(out, "Installing dependencies: %v\n", args)
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Stdin = in
		cmd.Stdout = out
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("install dependencies: %w", err)
		}
	}
	for _, name := range Tools {
		if _, err := lookup(name); err != nil {
			return fmt.Errorf("%s is still missing after dependency installation", name)
		}
	}
	fmt.Fprintln(out, "Preview tools are ready: "+strings.Join(Tools, ", "))
	return nil
}
func dependencyPlan(platform string, root bool, lookup func(string) (string, error)) ([][]string, error) {
	if platform == "darwin" {
		if path, err := lookup("brew"); err == nil {
			return [][]string{{path, "install", "ffmpeg", "mpv", "poppler"}}, nil
		}
		return nil, fmt.Errorf("install Homebrew, then run peep setup; or install ffmpeg, mpv and poppler manually")
	}
	if platform != "linux" {
		return nil, fmt.Errorf("automatic setup is unavailable on %s; install ffmpeg, mpv and poppler manually", platform)
	}
	for _, candidate := range []struct {
		name string
		args []string
	}{{"apt-get", []string{"install", "-y", "ffmpeg", "mpv", "poppler-utils"}}, {"dnf", []string{"install", "-y", "ffmpeg", "mpv", "poppler-utils"}}, {"pacman", []string{"-S", "--needed", "--noconfirm", "ffmpeg", "mpv", "poppler"}}} {
		path, err := lookup(candidate.name)
		if err != nil {
			continue
		}
		command := append([]string{path}, candidate.args...)
		if !root {
			sudo, err := lookup("sudo")
			if err != nil {
				return nil, fmt.Errorf("sudo is required to install dependencies; install ffmpeg, mpv and poppler manually")
			}
			command = append([]string{sudo}, command...)
		}
		return [][]string{command}, nil
	}
	return nil, fmt.Errorf("no supported package manager found; install ffmpeg, mpv and poppler manually")
}
