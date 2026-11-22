# peep

Go terminal media explorer. Read README.md and docs/architecture.md before changing playback or rendering.

- Keep terminal writes in Bubble Tea's renderer. Media helpers must never inherit terminal output.
- Keep filesystem and subprocess I/O cancellable. Bound frame buffers; discard stale preview generations.
- Run `just check` before committing. For playback changes also run `just test-integration` and `just test-tui`.
- Use the modular `just/` recipes; keep the root justfile an index.
- Use `camp p commit` in the owning campaign. Bootstrap commits go directly to main as authorized; subsequent work follows the user's requested workflow.
- No AI coauthor trailers. Do not commit generated fixtures or binaries.
