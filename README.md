# peep

A terminal file explorer for pictures, animated GIFs, videos, and music.

Browse silently, press Enter to play, and keep browsing while the current item continues. Still previews use native Kitty graphics automatically when a direct terminal confirms Kitty graphics support, preserving images up to 2048 pixels per dimension. Other terminals use ANSI color cells. Video and animated GIF playback also use native graphics on that path, preserving up to 1920 × 1080 pixels at a 12 fps target. The color-cell renderer remains the compatibility fallback.

## Install and run

Requires Go 1.24+, FFmpeg (`ffmpeg` and `ffprobe`), and mpv. `just` runs development recipes. The initial targets are macOS and Linux with a UTF-8, true-color terminal.

```sh
# macOS media dependencies
brew install ffmpeg mpv just

# From this repository
just build
./bin/peep ~/Pictures
./bin/peep --play movie.mp4
./bin/peep --play photo.png animation.gif movie.mp4 song.flac
./bin/peep --doctor
./bin/peep --seek-step=1s --play movie.mp4
./bin/peep --renderer=kitty ~/Pictures  # explicit native protocol override
./bin/peep --renderer=cells ~/Pictures # compatibility fallback

# Install the executable into your Go bin directory
just install
```

On Linux, install FFmpeg and mpv through your distribution's package manager. Audio plays on the machine running peep. Remote audio forwarding is not provided.

Without `--play`, startup is silent. Opening a directory lists files and folders. Text and Markdown files have scrollable previews; binary and special files show an unavailable-preview message. Enter on a file snapshots the visible media into a queue and starts at that file. Filtering or navigating afterward does not alter playback. Files are never modified or deleted.

## Controls

| Key | Action |
|---|---|
| `j/k`, up/down | Select file; select queue entry when queue is open |
| `h/l`, left/right, `Backspace` | Parent / enter directory (`Backspace` always goes up) |
| `Enter` | Enter directory, focus a document, or start selected media |
| `PgUp/PgDn` | Scroll document preview |
| `j/k`, arrows, Home/End in document focus | Scroll document / beginning / end |
| `R` on Markdown | Toggle styled Markdown and source |
| `/` or `Ctrl+F`, `Esc` | Search files and folders / go back (close overlay, return from focus, clear filter, then parent) |
| `H` | Toggle hidden files |
| `Space`, `p` | Play/pause |
| `s` | Stop and reset to zero |
| `[`, `]` | Seek backward/forward by the selected step (default 5 seconds); also clickable |
| `t`, `Tab`, then left/right | Focus timeline and seek backward/forward by the selected step |
| Click timeline | Seek to the clicked position |
| `,` | Cycle seek step: 1, 5, 10, 30, 60 seconds |
| `{`, `}` | Seek backward/forward 30 seconds |
| `0` | Seek to start |
| `n`, `b` | Next/previous queue entry |
| `z`, `x`, `c` | Halve/double speed (0.25–4x), reset to 1x |
| `-`, `=`, `m` | Volume down/up, mute |
| `D` | Cycle image display duration: 2, 5, 10, 30 seconds |
| `r` | Repeat off / one / all |
| `a`, `Q` | Append selected file / toggle queue |
| `d` in queue | Remove an inactive queue entry; never its file |
| `f`, `v` | Focus preview / show now playing |
| `B` | Toggle full-width browser |
| `?`, `q`, `Ctrl+C` | Help / quit / quit |

Search is immediate and local to the current directory. It matches case-insensitive substrings and fuzzy subsequences; space-separated terms must all match. Results are filtered from the cached listing. Enter keeps the filter; Esc clears it. Directory selection uses the full-width browser rather than an empty preview pane.

The interface uses a full-color theme, highlighted selection, bordered file/preview/queue panes, and a visible `../ Parent directory` entry. In narrow terminals the browser occupies the main pane; `f` switches to the preview. Click a file to select it, then press Enter to open or play.

During music playback, the now-playing pane shows a 24-band spectrum calculated from the actual audio. It follows playback time and seeking, freezes on pause, and clears on stop. Mute and volume affect displayed intensity. This is a visualizer, not an audio equalization effect. Use `v` to return to it while browsing.

Use `--seek-step=500ms`, `--seek-step=1s`, or another positive duration to choose a startup jump size. `,` changes it during the session, and transport labels update immediately. The fixed `{` / `}` and Shift+arrow jumps remain 30 seconds.

Timelines always control the file named **NOW PLAYING**, even when the preview shows something else. Images use a slideshow clock, initially five seconds. GIFs play one duration before advancing. Rewind is backward seeking; continuous reverse playback is not implemented. Unknown-duration files have no percentage seek or end jump.

## Pixelated previews over SSH

The preview title shows `native` or `cells`. Cells draw the image with colored text blocks, so resolution is limited by the terminal grid. Native graphics transmit image pixels through the Kitty protocol. Over SSH, the terminal app on the viewing device must support that protocol; the server's terminal software does not supply that capability. A successful SSH connection and true-color text do not imply native image support.

Auto mode tests support at startup. If the client does not answer or rejects the query, peep keeps the cell fallback. A smaller terminal font can provide more cells, within the decode cap, but cannot match native photo quality. Peep does not yet implement Sixel or iTerm2 image adapters. `--renderer=kitty` forces the protocol only when you know the client supports it; it cannot add support to the client.

## Development

The root `justfile` imports focused recipes from `just/build.just`, `just/dev.just`, and `just/test.just`.

```sh
just                   # list recipes
just check             # format check, vet, race tests, build
just test-integration  # generated fixtures + real ffmpeg/mpv; null audio output
just test-tui          # real PTY interaction, resize, controls, process cleanup
just build-all         # macOS/Linux, arm64/amd64; compile only
```

Integration fixtures and terminal transcripts are generated under ignored `tmp/`. Tests use synthetic tones and a null audio device, so they verify playback timing and command behavior without playing sound through your speakers. Actual device output and perceptual audio/video synchronization require a manual listening check.

## Bootstrap boundaries

- Implemented: directory navigation, hidden-file toggle, filename filtering, still and animated previews, in-terminal video, music playback, tags, and an audio-reactive spectrum, embedded cover art in static previews when FFmpeg exposes it, mixed queues, transport, speed, volume, repeat, and responsive focus/queue views.
- The cell fallback uses a 120-column by 100-pixel decode cap. Native still previews preserve up to 2048 pixels per dimension and are placed by the terminal. Auto mode sends a bounded capability query during startup. Confirmed Kitty-compatible terminals use native graphics; a rejection or no reply within 350 ms keeps cells. Multiplexers skip probing and default to cells. `--renderer=kitty` and `--renderer=cells` bypass detection. No iTerm2 inline-image adapter is implemented. 12 fps is a playback target, not a universal performance guarantee.
- The queue stops with an error on a bad item; use next or select another file. Automatic skip-on-error is deferred. Stopping during load cancels it; Play retries the retained entry.
- Preferences, queue state, and position are session-local. No automatic resume or persistent configuration yet.
- Shuffle, image zoom/pan, custom bindings, timestamp entry, drag scrubbing, queue reordering, subtitles, and network streams are deferred from the broader design.
- Support is based on the installed FFmpeg/mpv build. Filename recognition is not a codec guarantee. Tested fixture details live in [verification](docs/verification.md).

See [architecture](docs/architecture.md) for the rendering and playback boundary. The owning campaign design is `workflow/design/peep` (WI-aa8358).

## Document previews

Text and Markdown previews are implemented on this branch. Enter or `f` focuses a document; Escape returns to the browser. File filtering and playback controls remain available while reading. Reads are bounded to 2 MiB and layout to 12,000 rows, with explicit truncation markers. Non-UTF-8, binary, and special files are not decoded as text. Documents never enter media queues. Remote document resources are not fetched.

Markdown also displays local inline and reference-style images after their containing block. Native graphics preserve pixels and clip images when scrolling; other terminals show color cells. Missing images show an error, and remote images are not fetched. GIFs embedded in documents are still posters. `R` switches back to source.

Mermaid fences and `.mmd`/`.mermaid` files render through the optional [mmdg](https://github.com/bvolpato/mermaid-go-renderer) helper when native graphics are available. `R` switches between diagram and source. Missing helpers, invalid diagrams and cell-only terminals keep readable source with an explanation. Top-level fences are replaced by diagrams; diagrams in lists/quotes appear after their containing block, preserving the surrounding text.

```sh
just install-mermaid  # optional; installs pinned mmdg v0.13.0, requires Go 1.27+
peep --doctor        # reports whether the optional helper is on PATH
```

Peep itself still builds with Go 1.24+ and CGO disabled. The helper needs neither Node nor a browser; it does include an embedded Goja JavaScript engine. It is an independent installation, not a dependency of peep's Go module. Flowcharts and sequence diagrams are verified; other Mermaid types depend on the helper's support. Mermaid renders use a dark background, embedded fonts, bounded input/output and a cancellable deadline. No links are opened and no remote image assets are fetched.

Development uses feature branches and reviewable PRs. The [file preview proposal](docs/file-previews.md) records the scope and decisions.
