# peep

Your files. In focus. Browse files, preview images and video, listen to music and audiobooks, and read Markdown, source code, and Mermaid diagrams in your terminal.

```sh
npm install -g @obedience-corp/peep
peep setup
peep ~/Pictures
peep --doctor
```

macOS and Linux on x64 and arm64 are supported. The matching native executable is installed through npm's optional platform dependencies. No Go compiler, postinstall download, Node-based diagram renderer, or separate Mermaid helper is needed. Node is used only to launch the Go executable.

`peep setup` detects missing FFmpeg/mpv tools, shows the package-manager command,
and asks before installing. It supports Homebrew on macOS and apt-get, dnf, or
pacman on Linux. It leaves your npm installation of peep in place. For unattended
setup, use `peep setup --yes`. You can also install the tools yourself:

- macOS: `brew install ffmpeg mpv`
- Arch Linux: `sudo pacman -S ffmpeg mpv`
- Debian/Ubuntu: `sudo apt install ffmpeg mpv`

`peep --doctor` reports missing tools. Documents and Mermaid work without them. Native image quality requires a Kitty-graphics-compatible terminal; other terminals use color-cell previews.

In peep, `y` copies the selected path, `Y` copies image pixels, and `Ctrl+G`
opens a window to drag the file into another app. macOS uses built-in desktop
tools. Linux copying needs `wl-clipboard` on Wayland or `xclip` on X11;
dragging needs `ripdrag` or `dragon`/`dragon-drop`. These desktop helpers are
optional and are not installed by media setup.

[Source and full guide](https://github.com/lancekrogers/peep)
