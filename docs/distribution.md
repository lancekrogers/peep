# Distribution

peep ships one Go executable, with Mermaid built in, for macOS and Linux on
arm64 and x86-64. Releases include the MIT license and third-party notices.
FFmpeg and mpv provide media decoding and playback.

| Channel | Install command | Media dependencies |
| --- | --- | --- |
| Homebrew tap | `brew install lancekrogers/tap/peep` | Installed by Homebrew |
| npm | `npm install -g @obedience-corp/peep` | Install FFmpeg and mpv separately |
| AUR | `yay -S peep-tui-bin` | Installed by the AUR helper |

The executable is always `peep`. The unscoped npm name `peep` and AUR
names `peep`/`peep-bin` belong to unrelated projects.

## Build and verify

```sh
just check
just distribution 0.2.0
just test-distribution
just test-aur                       # Docker: actual Arch build/install
```

Choose an unused stable version for an actual release. Generation refuses to
replace `dist/distribution`; remove that generated directory explicitly before
rebuilding. Package metadata lives in `packaging/metadata.json`.

The generator verifies all four archive checksums, then creates:

- `dist/distribution/homebrew/peep.rb`
- `dist/distribution/aur/PKGBUILD` and `.SRCINFO`
- `dist/distribution/npm/` with a command package and four native packages
- `dist/distribution/npm-tarballs/` with publishable npm tarballs

The npm launcher uses exact-version optional dependencies selected by npm for
the host OS and CPU. It has no install scripts, network downloads, or compiler
requirements. Node 22 or newer is required for this installation channel.
The npm smoke check installs tarballs offline into a temporary prefix and
verifies both `--version` and Mermaid PNG output. It does not modify the user's
global installation.

## One-time publishing setup

The existing `@obedience-corp` npm organization is used independently of the
personal GitHub repository. The existing AUR maintainer account is `obediencecorp`.

1. Make the repository public when ready to launch. Package publishing refuses
   a private repository because Homebrew/AUR download URLs must be public.
2. Confirm ownership of the npm scope `@obedience-corp`. Publish all four native
   packages before the command package. For the initial publication, use an
   authenticated npm CLI locally or a short-lived `NPM_TOKEN` GitHub environment
   secret with package creation rights. Follow npm's account/2FA requirements.
3. Configure an npm trusted publisher on **each of the five packages** for
   repository `lancekrogers/peep`, workflow `publish.yml`, environment
   `distribution`. Subsequent publication uses GitHub OIDC; remove the initial
   token. The workflow uses Node 24 with its bundled modern npm.
4. In GitHub's `distribution` environment, add `HOMEBREW_TAP_TOKEN` with contents
   and pull-request write access to `lancekrogers/homebrew-tap`.
5. Use the existing AUR account `obediencecorp` and register a dedicated
   publishing SSH key with write access to `peep-tui-bin`.
   Add environment secrets `AUR_SSH_KEY`, `AUR_EMAIL`, and `AUR_KNOWN_HOSTS`.
   Verify the AUR SSH host fingerprint through Arch's published information
   before storing its known-hosts entry. The workflow never accepts a new host
   key automatically.

The existing personal Homebrew tap is used; no Homebrew/core submission is
needed. Its formula update is opened as a pull request for review.

## Release

Merge the packaging PR first. Tag the intended main commit with a new `vX.Y.Z`
tag and push that tag. The release workflow:

1. Builds all four archives and generates every channel from those archives.
2. Tests npm installation and embedded Mermaid on Linux and macOS.
3. Publishes the archives, npm tarballs, formula, AUR metadata, and a combined
   `checksums.txt` to the GitHub release.

Run **publish packages** manually for that release version, selecting one
channel or all three. Every job downloads and verifies the release assets.
Homebrew opens a tap PR, npm publishes native packages before the launcher,
and AUR pushes the recipe and matching `.SRCINFO`.

Merge the tap PR, then test fresh installations through each public channel
and run `peep --version` and `peep --doctor`. npm installations need FFmpeg/mpv
installed separately; see [installation](install.md).

Rerunning a channel for the same version is safe: Homebrew reuses an existing
update branch/PR or recognizes a merged formula; AUR skips an identical recipe;
npm checks published tarball integrity before skipping it.

Published versions are immutable. If a channel fails, resolve its credential
or package issue and rerun that channel. If npm publication stopped halfway,
the publisher verifies already-published packages against the release tarball
integrity before skipping them; do not replace a published version with different bytes.

References: [npm trusted publishers](https://docs.npmjs.com/trusted-publishers/),
[Homebrew taps](https://docs.brew.sh/How-to-Create-and-Maintain-a-Tap),
[AUR submission guidelines](https://wiki.archlinux.org/title/AUR_submission_guidelines).
