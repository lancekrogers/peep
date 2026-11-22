package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
	"github.com/lancekrogers/peep/internal/player"
	"github.com/lancekrogers/peep/internal/render"
	"github.com/lancekrogers/peep/internal/tui"
)

const version = "0.1.3-dev"

func run() error {
	flags := flag.NewFlagSet("peep", flag.ContinueOnError)
	play := flags.Bool("play", false, "start playback explicitly")
	doctor := flags.Bool("doctor", false, "report media dependencies")
	showVersion := flags.Bool("version", false, "show version")
	renderer := flags.String("renderer", "auto", "image/video renderer: auto, kitty, or cells")
	audio := flags.String("audio-output", "", "mpv audio output override (null for automated tests)")
	flags.Usage = func() {
		fmt.Fprint(flags.Output(), "peep — terminal pictures, GIFs, videos, and music\n\nUsage: peep [flags] [directory | media files...]\n")
		flags.PrintDefaults()
	}
	if err := flags.Parse(os.Args[1:]); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if *showVersion {
		fmt.Println("peep", version)
		return nil
	}
	mode, err := render.Renderer(*renderer, os.Getenv)
	if err != nil {
		return err
	}
	if *doctor {
		missing := false
		fmt.Println("peep", version, "· image/video renderer:", mode)
		for _, name := range []string{"ffmpeg", "ffprobe", "mpv"} {
			path, err := exec.LookPath(name)
			if err != nil {
				fmt.Println(name + ": MISSING")
				missing = true
			} else {
				fmt.Printf("%s: %s\n", name, path)
			}
		}
		if missing {
			return fmt.Errorf("install missing media tools; see README.md")
		}
		return nil
	}
	if !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stdout.Fd()) {
		return fmt.Errorf("peep needs an interactive terminal; use --doctor for diagnostics")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	backend, err := player.New(ctx, *audio)
	notice := ""
	if err != nil {
		notice = err.Error()
	} else {
		defer backend.Close()
	}
	model, err := tui.New(ctx, backend, flags.Args(), *play, notice)
	if err != nil {
		return err
	}
	options := []tea.ProgramOption{tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithContext(ctx)}
	if mode == "kitty" {
		writer := render.NewKittyWriter(os.Stdout)
		defer writer.Close()
		model.SetGraphics(writer)
		options = append(options, tea.WithOutput(writer))
	}
	_, err = tea.NewProgram(model, options...).Run()
	return err
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "peep:", err)
		os.Exit(1)
	}
}
