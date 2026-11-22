# peep

Go terminal media explorer. Read README.md and docs/architecture.md before changing playback or rendering.

- Keep terminal writes in Bubble Tea's renderer. Media helpers must never inherit terminal output.
- Keep filesystem and subprocess I/O cancellable. Bound frame buffers; discard stale preview generations.
- Run `just check` before committing. For playback changes also run `just test-integration` and `just test-tui`.
- Use the modular `just/` recipes; keep the root justfile an index.
- Keep plans, design proposals, and task tracking in campaign work items, not this repository. The owning work item is WI-aa8358 at `workflow/design/peep` in the My_Tools campaign. Repository docs describe usage, implemented architecture, and verification.
- Bootstrap is complete. Use feature branches and ready-for-review pull requests targeting `main`. Dependent slices may target the preceding feature branch for review; retarget them to `main` after that dependency merges. Do not push changes directly to `main` or merge PRs without explicit user authorization.
- Use `camp p commit` in the owning campaign. For unmerged PR branches, use `--no-sync`; update the campaign submodule reference after the change lands on project `main`.
- No AI coauthor trailers. Do not commit generated fixtures or binaries.
