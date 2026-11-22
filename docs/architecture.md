# Bootstrap architecture

Go owns the browser, queue, keyboard controls, transport state, and terminal composition. Bubble Tea v1.3.10 is pinned to the version already used by other campaign tools. Media work runs outside the UI thread.

- `internal/browser`: directory listings, type recognition, filter, and deterministic case-insensitive name order with directories first.
- `internal/media`: cancellable FFprobe metadata and FFmpeg RGB frame streams. The queue holds at most two decoded frames; a pending future frame lives in the UI. Processes receive arguments directly, never through a shell.
- `internal/player`: one isolated mpv process, a private temporary Unix socket, structured JSON commands, property observations, and deterministic process/socket cleanup.
- `internal/render`: ANSI true-color foreground/background pairs using the upper-half-block character; terminal-safe filenames/tags and display-width clipping.
- `internal/tui`: event-loop state, asynchronous listing/loading, preview generations, queue, and layout. Only Bubble Tea writes terminal output.

For moving media, mpv supplies audio and authoritative media time with `--vo=null`. A separate FFmpeg process decodes visual frames at 12 fps. The UI releases each frame when mpv time reaches its timestamp. Seeking or resizing cancels that visual decoder and starts another at the current position. Pausing freezes the transport and limits prefetch to the bounded channel. Still images use a monotonic application clock. A selected poster preview never plays audio.

This deliberately trades duplicate video decoding for a simple, isolated bootstrap path that cannot fight the TUI for stdout. It proves terminal composition and transport without native FFI. It is not the final efficiency story: a shared-decoder backend should be evaluated against the existing interfaces after measurement. Frame timestamps currently use the requested seek offset and the resampled 12 fps cadence; sample-accurate synchronization is not claimed.

Resource limits: 10-second metadata probe, 10-second file-load deadline, 3-second IPC commands, two queued frames, 120 x 100 maximum cell decode size; native PNG limits are described below. Selection, resize, seek, and exit cancel obsolete decoder processes. Preview IDs reject late results. Filename extensions select candidate media types; FFprobe validates actual streams before playback.

Runtime dependencies are external installations, not bundled binaries. The Go executable cross-compiles with CGO disabled; mpv/FFmpeg and a suitable terminal are required on the destination. Windows IPC is not implemented.

Future graphics backends must preserve a single terminal writer, cancellation, bounded buffering, and the preview-versus-playback distinction. The broader campaign design remains the product direction; README.md documents what this bootstrap actually implements.

## Native still previews and directory search

`internal/media.PreviewPNG` produces a single PNG with a maximum dimension of 2048 pixels, without upscaling smaller images. A ten-second deadline and 20 MiB output cap bound work. Selection generation IDs reject stale results. This static path covers still images, embedded album art, and video/GIF posters. Native moving playback uses the PNG frame stream described below.

`internal/render.KittyWriter` wraps Bubble Tea's output and implements the terminal file interface, preserving resize detection. A mutex serializes text and Kitty graphics commands. PNG payloads are uploaded in 4096-character base64 chunks, then placed in the preview rectangle after text output. The terminal preserves aspect ratio within that rectangle. Each process owns its image ID; replacement, help overlays, and alternate-screen exit delete its image. Cursor save/restore and `C=1` prevent image placement from moving the TUI cursor. No worker writes to stdout directly.

Auto-selection starts with cells and sends a non-storing Kitty query after Bubble Tea enters raw mode. A positive reply enables native graphics and refreshes the selected/active preview. A negative reply or 350 ms timeout keeps cells. An input wrapper retains the terminal file interface, removes only replies matching the query ID, and forwards unrelated keys. Fragment assembly is bounded in time and bytes. Multiplexers skip probing; explicit renderer overrides bypass it. The startup wrapper keeps capability negotiation outside the media model. The protocol implementation follows [Kitty graphics documentation](https://sw.kovidgoyal.net/kitty/graphics-protocol/). Rendering in a particular terminal still needs user validation; PTY tests verify protocol payloads and lifecycle, not an emulator's actual pixels.

Directory data is loaded once per navigation/hidden-file refresh. Search filters the cached entries with case-insensitive substring and fuzzy-subsequence ranking. Folder selection and active search expand the browser to use available width; the preview pane returns when media is selected or focus is requested.

## Audio-reactive spectrum

A separate cancellable FFmpeg process provides mono 16 kHz PCM for the active music file. Goertzel analysis over Hann-windowed 512-sample blocks measures 24 logarithmically spaced frequencies from 60 Hz to 7 kHz. Levels are mapped from decibels to bar heights; the UI applies decay between observed samples. This is actual file audio, not a simulated animation or microphone capture.

Each block is timestamped against the same media timeline as mpv. The UI consumes it at the corresponding playback position. A two-block channel bounds decode prefetch; seeks cancel and restart the stream at the target. Pause freezes displayed levels; stop/file replacement/exit cancel analysis. Volume and mute scale the display. Analysis failure leaves playback functional and shows a visualizer-specific message. The player does not change audio frequency response.

## Native video and GIF frames

The Kitty path streams PNG video frames at up to 1920 × 1080, sampled at 12 fps. FFmpeg keeps smaller sources at their original dimensions. A bounded PNG parser separates frames without losing bytes across pipe reads; two queued frames and a 20 MiB per-frame limit bound buffering. Frame timestamps continue to follow mpv's playback clock.

The terminal compositor refreshes changed native frames even when text is unchanged. A short deferred flush gives the text renderer an opportunity to paint first; a shared lock serializes all writes. Synchronized-output markers enclose deletion, upload, text, and placement to avoid presenting a half-updated frame. Selection, seek, resize, overlays, and exit retain the same generation/cancellation and image-ownership rules. Actual sustained frame rate and graphical-terminal playback remain host-dependent.
