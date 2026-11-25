package installation

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupConfirmationAndVerification(t *testing.T) {
	for _, answer := range []string{"\n", "no\n", "yes", "", "y\n", "YES\n"} {
		t.Run(strings.ReplaceAll(answer, "\n", "ENTER"), func(t *testing.T) {
			dir := t.TempDir()
			marker := filepath.Join(dir, "installed")
			manager := filepath.Join(dir, "brew")
			if err := os.WriteFile(manager, []byte("#!/bin/sh\nprintf installed > '"+marker+"'\n"), 0700); err != nil {
				t.Fatal(err)
			}
			lookup := func(name string) (string, error) {
				if name == "brew" {
					return manager, nil
				}
				if _, err := os.Stat(marker); err == nil {
					return "/tools/" + name, nil
				}
				return "", os.ErrNotExist
			}
			var out bytes.Buffer
			err := ensureMedia(context.Background(), strings.NewReader(answer), &out, &out, true, "darwin", false, lookup)
			if err != nil {
				t.Fatal(err)
			}
			_, installed := os.Stat(marker)
			want := answer == "y\n" || answer == "YES\n"
			if (installed == nil) != want {
				t.Fatalf("unexpected install for %q: %s", answer, out.String())
			}
			if !strings.Contains(out.String(), "Missing preview tools") || !strings.Contains(out.String(), "[y/N]") {
				t.Fatal(out.String())
			}
		})
	}
}

func TestSetupNoopFailureAndPostCheck(t *testing.T) {
	var out bytes.Buffer
	lookup := func(name string) (string, error) { return "/tools/" + name, nil }
	if err := ensureMedia(context.Background(), strings.NewReader(""), &out, &out, true, "linux", false, lookup); err != nil || strings.Contains(out.String(), "[y/N]") {
		t.Fatal(out.String(), err)
	}
	for _, exit := range []string{"0", "7"} {
		dir := t.TempDir()
		manager := filepath.Join(dir, "brew")
		os.WriteFile(manager, []byte("#!/bin/sh\nexit "+exit+"\n"), 0700)
		lookup := func(name string) (string, error) {
			if name == "brew" {
				return manager, nil
			}
			return "", os.ErrNotExist
		}
		if err := ensureMedia(context.Background(), strings.NewReader(""), &out, &out, false, "darwin", false, lookup); err == nil {
			t.Fatal("setup falsely reported success", exit)
		}
	}
}
