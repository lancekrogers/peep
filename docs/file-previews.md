# File and document previews

Status: proposal, not implemented. Owning campaign work item: WI-aa8358.

Peep should let someone browse a folder, read its README with illustrations and diagrams, and keep listening to the current track. This extends its existing browse-and-preview behavior. Keep the first implementation read-only and local, with optional diagram rendering.

## Scope and delivery

| PR | User outcome | Main work |
|---|---|---|
| 1. Text and Markdown | Select a text, source, or Markdown file and read a scrollable preview; Enter focuses the document | File classification, bounded reads, text rendering, styled Markdown, viewport and focus controls, media-only queue filtering |
| 2. Embedded images | Markdown displays local illustrations between text blocks | Resolve relative paths, document layout, multiple graphics placements, clipping, cell fallback and alt text |
| 3. Mermaid | Mermaid fences and `.mmd` files display diagrams when the optional renderer is installed | Cancellable diagram adapter, bounded cache, PNG composition, source fallback and diagnostics |

Each PR should be independently usable. Basic document previews must not require a browser runtime. Editing, HTML page execution, remote asset fetching, PDF/Office parsing, and a general plugin system are outside this slice. Plain source previews precede syntax highlighting; interactive document links and in-document search can follow after scrolling and layout work reliably.

## Current code and the necessary extension

`internal/browser.Read` currently excludes every unrecognized media extension. Its entries carry `media.Kind`, and `Model.previewSelection` routes all files through FFprobe. Queue creation and append treat any non-directory entry as playable. Simply exposing Markdown files in the browser would therefore send documents to the media backend.

Introduce preview classification independent of playback classification. List ordinary files; identify Markdown, known text/source files, existing media, and unsupported/binary files. Use a bounded sample to recognize otherwise unknown text. Unsupported files show basic file metadata and an explicit unavailable-preview message. Queue creation, append, CLI startup arguments, and next/previous must continue to accept only playable media.

A focused `internal/document` package should read and parse documents. The existing TUI owns preview selection, cancellation, focus, viewport, and generation checks. Do not build a second browser or player. Reading a document must not start media playback or replace the playing item.

## Interaction

```text
 peep   BROWSE    / search files & folders
 < Backspace parent   ~/projects/peep
 ┌ Files ──────────────────┐ ┌ README.md · Markdown · 12/96 ──────────┐
 │ ../                     │ │ peep                                    │
 │ > README.md        MD   │ │ Terminal media and document previews    │
 │   architecture.mmd MMD │ │                                         │
 │   screenshot.png IMAGE │ │ [local illustration, fitted to width]   │
 │   song.flac      AUDIO │ │                                         │
 │                         │ │ Architecture                            │
 │                         │ │ [rendered Mermaid diagram]              │
 └─────────────────────────┘ └───────────────────────────────────────────┘
 NOW PLAYING  song.flac       [Playing] 01:23 / 03:04
 [ rewind ]   [ pause ]   [ stop ]   [ forward ]
 Enter/f read    PgUp/PgDn scroll    Esc back    / file filter
```

The sketch shows the completed three-PR feature, not PR 1 alone. Existing narrow-terminal focus behavior applies.

- Selection previews silently. Enter or `f` focuses a document. Enter on media retains its play behavior.
- Browser focus: arrows and `j/k` select files; Home/End select first/last.
- Document focus: arrows and `j/k` scroll; Home/End go to document start/end. PgUp/PgDn scroll by a viewport. Mouse wheel over the document scrolls it.
- Escape exits document focus before existing back/filter/parent actions. Backspace remains parent navigation.
- `/` and Ctrl+F retain file filtering. Tab retains timeline focus. Space and seek shortcuts retain control of NOW PLAYING, even while reading.
- Display a document position and rendered/source mode. A contextual `R` toggle switches Markdown between rendered text and source without re-reading the file.
- Preserve scroll position on resize as closely as practical using a source-block anchor; reset on selecting a different document. No persistent reading history in this slice.

## Rendering approach

Use terminal text for Markdown headings, paragraphs, lists, quotes, tables, and fenced code. Text should stay crisp and searchable as terminal text instead of rasterizing the entire page. [Glamour](https://github.com/charmbracelet/glamour) is a candidate for ANSI Markdown styling; it supports width-aware wrapping. Select a version compatible with peep's pinned Go/Charm dependencies during PR 1 rather than bundling a Bubble Tea migration into this feature.

Represent the document as ordered text and image/diagram blocks, retaining source locations and asset references. Parse Markdown structurally, including reference-style images, rather than extracting image syntax with regular expressions. This intermediate layout lets image blocks reserve rows and lets later diagram output reuse their placement path. In the initial layout, image elements occupy their own blocks even when the Markdown syntax places them within a paragraph.

### Local images

Resolve image paths relative to the Markdown file, not the process working directory. Reuse existing image decoding and renderer capability selection. Fit images to document width with bounded height; preserve aspect ratio. Display alt text and the referenced path when an image is missing, unreadable, unsupported, or cannot be decoded. Remote URLs remain labeled references without automatic downloads.

The current Kitty compositor owns one image and placement. PR 2 must extend that same synchronized writer to manage the visible document placements. Assign stable IDs, delete placements as blocks leave the viewport, crop partially visible blocks, and clear all document images on navigation, resize, help, and exit. Bound decoded assets and uploads; do not decode every image in a long document at selection time. The existing single-image media path must retain its behavior.

Cell-only terminals retain styled Markdown text, with cell image previews and alt text. Terminal graphics capability still determines image fidelity; this feature cannot make text blocks reproduce native image pixels.

### Mermaid

Prefer a native renderer with no Node/browser dependency. Evaluate [mmdg (Go)](https://github.com/bvolpato/mermaid-go-renderer) first: its CLI provides PNG output without CGO, and can run as a cancellable helper without changing peep's Go module. [mmdr (Rust)](https://github.com/1jehuang/mermaid-rs-renderer) also offers SVG/PNG output without Node and remains an alternative. [pozeiden (Zig)](https://github.com/sc2in/pozeiden) offers SVG output, but would require a rasterization step for the current terminal compositor and documents syntax gaps. Select and pin a helper only after fixture rendering and output inspection; upstream coverage/performance claims are not peep verification results.

No Node-based renderer is required. The adapter stays optional, with readable diagram source when unavailable.
Extract each Mermaid fence or standalone `.mmd` document and render it to a bounded PNG, then use the same image-block compositor. Cache by source content, renderer version, width, and theme. Render visible diagrams first; cancel obsolete work and apply a deadline/output limit. Invoke the executable directly with fixed arguments and private temporary files. Diagram content supplies source only; do not accept executable commands or arbitrary renderer configuration from the document.

With no renderer installed, a failed render, or a cell-only terminal where diagram labels would be illegible, show the Mermaid source with a concise explanation. Ordinary Markdown must remain readable. Pin and document the supported adapter version when implementing PR 3; the core executable must continue to work without the optional helper.

## Bounds and failure behavior

Start with a 2 MiB document read limit and show a truncation marker; tune from fixtures rather than silently hanging on large files. Read regular files only, detect NUL/invalid text, and show metadata for binary files and special filesystem entries. Sanitize source control characters before applying trusted renderer styling so file contents cannot emit terminal commands.

Keep reads, asset decoding, and diagram processes outside the UI event loop. Bound asset bytes, pixel dimensions, worker concurrency, and cache memory. A removed file, permission failure, malformed document, or stale result must leave navigation and playback responsive. Markdown is content: raw HTML remains escaped/source text, and embedded instructions are never executed.

## Acceptance evidence

- A folder containing media, README.md, source files, unknown text, binary data, and a FIFO stays navigable. No document or special file reaches FFprobe/mpv or enters the playback queue.
- Markdown fixtures cover headings, tables, Unicode, code, long lines, empty content, malformed markup, truncation, and terminal escape bytes. Render/source toggle and resizing preserve useful reading position.
- A real PTY demonstrates document selection, focus, scrolling, return to browser, file search, and uninterrupted playback/transport while reading.
- Image fixtures cover relative/reference paths, spaces, missing images, multiple visible images, partial clipping, fast scrolling, help overlays, resize, and replacement. Verify native protocol placement lifecycle and cell output separately; inspect native pixels in a compatible graphical terminal before claiming visual fidelity.
- Mermaid tests cover a flowchart and sequence diagram, invalid syntax, missing executable, timeout/cancellation, output limits, cache invalidation, and source fallback. Verify no child processes or graphics survive exit.
- Each implementation PR runs `just check`; changes touching media composition or transport also run integration and PTY suites. Document platform and terminal limits honestly.
