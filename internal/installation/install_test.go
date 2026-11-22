package installation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestInstallUpgradeAndFailurePreservation(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "peep")
	prefix := filepath.Join(dir, "prefix with spaces")
	if err := os.WriteFile(source, []byte("first version"), 0755); err != nil {
		t.Fatal(err)
	}
	public, err := Install(context.Background(), source, prefix)
	if err != nil {
		t.Fatal(err)
	}
	first, err := filepath.EvalSymlinks(public)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(source, []byte("second version"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err = Install(context.Background(), source, prefix); err != nil {
		t.Fatal(err)
	}
	second, _ := filepath.EvalSymlinks(public)
	if second == first {
		t.Fatal("upgrade did not switch version")
	}
	data, _ := os.ReadFile(first)
	if string(data) != "first version" {
		t.Fatal("running version removed")
	}
	data, _ = os.ReadFile(public)
	if string(data) != "second version" {
		t.Fatal("wrong installed contents")
	}
	if _, err = Install(context.Background(), source, prefix); err != nil {
		t.Fatal("repeat install", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = Install(ctx, source, prefix); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation", err)
	}
	after, _ := filepath.EvalSymlinks(public)
	if after != second {
		t.Fatal("failed install changed active version")
	}
	if _, err = Install(context.Background(), filepath.Join(dir, "missing"), prefix); err == nil {
		t.Fatal("missing source accepted")
	}
	if err = os.WriteFile(second, []byte("corrupt existing copy"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err = Install(context.Background(), source, prefix); err == nil {
		t.Fatal("damaged version accepted")
	}
	entries, _ := filepath.Glob(filepath.Join(prefix, "lib/peep/versions/.stage-*"))
	if len(entries) > 0 {
		t.Fatal("staging leaked", entries)
	}
}
func TestFailedActivationPreservesDestination(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	os.WriteFile(source, []byte("binary"), 0755)
	public := filepath.Join(dir, "install/bin/peep")
	os.MkdirAll(public, 0755)
	os.WriteFile(filepath.Join(public, "keep"), []byte("safe"), 0600)
	if _, err := Install(context.Background(), source, filepath.Join(dir, "install")); err == nil {
		t.Fatal("replaced directory")
	}
	if _, err := os.Stat(filepath.Join(public, "keep")); err != nil {
		t.Fatal(err)
	}
}
func TestDependencyPlans(t *testing.T) {
	for _, tc := range []struct {
		platform string
		root     bool
		tools    []string
		want     []string
	}{
		{"darwin", false, []string{"brew"}, []string{"/tools/brew", "install", "ffmpeg", "mpv"}},
		{"linux", false, []string{"apt-get", "sudo"}, []string{"/tools/sudo", "/tools/apt-get", "install", "-y", "ffmpeg", "mpv"}},
		{"linux", true, []string{"pacman"}, []string{"/tools/pacman", "-S", "--needed", "--noconfirm", "ffmpeg", "mpv"}},
	} {
		lookup := func(name string) (string, error) {
			for _, tool := range tc.tools {
				if name == tool {
					return "/tools/" + name, nil
				}
			}
			return "", os.ErrNotExist
		}
		plan, err := dependencyPlan(tc.platform, tc.root, lookup)
		if err != nil || len(plan) != 1 || !reflect.DeepEqual(plan[0], tc.want) {
			t.Fatal(plan, err)
		}
	}
	if _, err := dependencyPlan("linux", false, func(string) (string, error) { return "", os.ErrNotExist }); err == nil {
		t.Fatal("missing manager accepted")
	}
}
