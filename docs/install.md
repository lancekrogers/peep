# Installation

Mermaid rendering and fonts are compiled into peep. The native executable needs no Go, Node, browser, or separate `mmdg` installation. The npm launcher requires Node 22+. macOS and Linux on arm64/amd64 are supported.

## Package managers

```sh
# Homebrew (macOS / Linux): includes FFmpeg and mpv
brew install lancekrogers/tap/peep

# npm (macOS / Linux, Node 22+)
npm install -g @obedience-corp/peep

# AUR (Arch Linux): includes FFmpeg and mpv
yay -S peep-tui-bin
# or: paru -S peep-tui-bin
```

For npm installs, install the media tools separately:

```sh
brew install ffmpeg mpv               # macOS / Homebrew Linux
sudo apt install ffmpeg mpv           # Debian / Ubuntu
sudo pacman -S ffmpeg mpv             # Arch Linux
```

Upgrade through the same package manager. Use `command -v peep` to check which
installation your shell runs if you have installed peep through multiple methods.

## Open peep

```sh
peep                   # Browse the current directory
peep ~/Pictures        # Browse a directory
peep README.md         # Preview a file
peep --play movie.mp4   # Start media playback
peep --doctor          # Check media dependencies and terminal support
```

`peep <path>` accepts a file or directory. Quote paths containing spaces, for example
`peep "$HOME/My Pictures"`.

## Release archive

Download the archive for your OS and architecture from
[GitHub Releases](https://github.com/lancekrogers/peep/releases/latest) and verify
its SHA-256 against `checksums.txt`. From the extracted archive:

```sh
./bin/peep --install                  # defaults to ~/.local; installs missing media tools
./bin/peep --install --prefix /your/prefix
./bin/peep --install --no-deps         # offline/managed hosts: skip media dependency setup
~/.local/bin/peep --doctor
```

The installer uses Homebrew on macOS or apt-get/dnf/pacman on Linux for missing FFmpeg/mpv tools. It may request sudo through the package manager. If the manager or packages are unavailable, installation reports the failure before replacing peep. `--no-deps` still supports documents and Mermaid diagrams; media features require their tools.

Installation stages and verifies a versioned executable, then atomically updates `<prefix>/bin/peep`. Old versions remain available to running processes. Use the same command for upgrades. Add `<prefix>/bin` to PATH and check `command -v peep` if an older Go-bin installation takes precedence.

## Build from source

Building from source requires **Go 1.27+** and `just`:

```sh
just build                         # includes Mermaid automatically
just install                       # installs into ~/.local with media setup
just package                       # archive + SHA-256 file for the current host
just package-all                   # macOS/Linux × arm64/amd64 archives
./bin/peep ~/Pictures
./bin/peep --play movie.mp4
./bin/peep --play photo.png animation.gif movie.mp4 song.flac
./bin/peep --doctor
./bin/peep --seek-step=1s --play movie.mp4
./bin/peep --renderer=kitty ~/Pictures
./bin/peep --renderer=cells ~/Pictures
```

Package archives contain one executable, README, documentation, the Night Shift demo collection, the MIT license, and renderer notices. Open the included collection with `./bin/peep examples/night-shift/00-start-here.md`; the `just demo` recipe is available in source checkouts. `--licenses` displays those notices from the executable itself. CI builds host packages and checks installation/rendering without a helper or language toolchain on PATH; publishing a release is a separate action.

On Linux, install FFmpeg and mpv through your distribution's package manager. Audio plays on the machine running peep. Remote audio forwarding is not provided.

Without `--play`, startup is silent. Opening a directory lists files and folders. Text and Markdown files have scrollable previews; binary and special files show an unavailable-preview message. Enter on a file snapshots the visible media into a queue and starts at that file. Filtering or navigating afterward does not alter playback. Browsing and playback never modify files. Explicit file operations are available from the browser.

[Back to peep](../README.md)
