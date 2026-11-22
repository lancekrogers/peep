# peep

A terminal file explorer for pictures, animated GIFs, videos, and music.

Browse silently, press Enter to play, and keep browsing while the current item continues. The first version renders media as ANSI true-color half-block cells inside the interface. Video and GIF output targets 12 frames per second; this is a working low-resolution terminal player, not a full-resolution Kitty graphics renderer.

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

# Install the executable into your Go bin directory
just install
```

On Linux, install FFmpeg and mpv through your distribution's package manager. Audio plays on the machine running peep. Remote audio forwarding is not provided.

Without `--play`, startup is silent. Opening a directory lists its media and folders. Enter on a file snapshots the visible media into a queue and starts at that file. Filtering or navigating afterward does not alter playback. Files are never modified or deleted.

## Controls

| Key | Action |
|---|---|
| `j/k`, up/down | Select file; select queue entry when queue is open |
| `h/l`, left/right | Parent / enter directory |
| `Enter` | Enter directory or start selected media |
| `/`, `Esc` | Filter filenames / clear filter or close overlay |
| `H` | Toggle hidden files |
| `Space`, `p` | Play/pause |
| `s` | Stop and reset to zero |
| `[`, `]` | Seek backward/forward 5 seconds |
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
| `?`, `q`, `Ctrl+C` | Help / quit / quit |

Timelines always control the file named **NOW PLAYING**, even when the preview shows something else. Images use a slideshow clock, initially five seconds. GIFs play one duration before advancing. Rewind is backward seeking; continuous reverse playback is not implemented. Unknown-duration files have no percentage seek or end jump.

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

- Implemented: directory navigation, hidden-file toggle, filename filtering, still and animated previews, in-terminal video, music playback and tags, embedded cover art when FFmpeg exposes it, mixed queues, transport, speed, volume, repeat, and responsive focus/queue views.
- The cell renderer uses a 120-column by 100-pixel decode cap. 12 fps is the target, not a universal performance guarantee. Slow terminals may render fewer frames. Native-resolution Kitty/iTerm2 graphics remain future work.
- The queue stops with an error on a bad item; use next or select another file. Automatic skip-on-error is deferred. Stopping during load cancels it; Play retries the retained entry.
- Preferences, queue state, and position are session-local. No automatic resume or persistent configuration yet.
- Shuffle, image zoom/pan, custom bindings, timestamp entry, mouse scrubbing, queue reordering, subtitles, and network streams are deferred from the broader design.
- Support is based on the installed FFmpeg/mpv build. Filename recognition is not a codec guarantee. Tested fixture details live in [verification](docs/verification.md).

See [architecture](docs/architecture.md) for the rendering and playback boundary. The owning campaign design is `workflow/design/peep` (WI-aa8358).
