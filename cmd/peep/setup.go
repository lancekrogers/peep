package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/lancekrogers/peep/internal/installation"
)

func setup(args []string) error {
	flags := flag.NewFlagSet("peep setup", flag.ContinueOnError)
	yes := flags.Bool("yes", false, "install missing media tools without asking")
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "Usage: peep setup [--yes]\n\nInstall missing FFmpeg and mpv using Homebrew, apt-get, dnf, or pacman.\nPeep itself is not reinstalled. Use peep ./setup to open a path named setup.")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("usage: peep setup [--yes]")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()
	// Unblock a pending confirmation read on termination, too.
	stopRead := context.AfterFunc(ctx, func() { _ = os.Stdin.Close() })
	defer stopRead()
	return installation.SetupMedia(ctx, os.Stdin, os.Stdout, os.Stderr, *yes)
}
