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
	"github.com/lancekrogers/peep/internal/tui"
)

const version = "0.1.0-dev"

func run() error {
	flags := flag.NewFlagSet("peep", flag.ContinueOnError)
	play := flags.Bool("play", false, "start playback explicitly")
	doctor := flags.Bool("doctor", false, "report media dependencies")
	showVersion := flags.Bool("version", false, "show version")
	renderer := flags.String("renderer", "cells", "renderer: auto or cells")
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
	if *doctor {
		missing := false
		fmt.Println("peep", version, "· renderer: ANSI true-color cells (12 fps target)")
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
	if *renderer != "auto" && *renderer != "cells" {
		return fmt.Errorf("renderer %q is not implemented; use cells", *renderer)
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
	_, err = tea.NewProgram(model, tea.WithAltScreen(), tea.WithContext(ctx)).Run()
	return err
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "peep:", err)
		os.Exit(1)
	}
}
