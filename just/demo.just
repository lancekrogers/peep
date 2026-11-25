# Open a disposable copy of the curated demo collection.
demo: build
    #!/usr/bin/env bash
    set -euo pipefail
    mkdir -p tmp
    demo_dir=$(mktemp -d "$PWD/tmp/_night-shift-XXXXXX")
    cp -R examples/night-shift/. "$demo_dir/"
    # Distinct example modification dates make the sort demonstration visible.
    touch -t 202601010100 "$demo_dir/01-images/01-moonrise.jpg"
    touch -t 202601020600 "$demo_dir/01-images/02-daybreak.png"
    echo "Demo workspace: $demo_dir"
    ./bin/peep "$demo_dir/00-start-here.md"

# Optional authoring tools: Go, FFmpeg, rsvg-convert; NARRATE=1 also needs macOS say.
demo-assets:
    bash scripts/rebuild-demo.sh

# Verify all curated media and real TUI audiobook/document flows.
test-demo: build
    python3 scripts/demo-smoke.py

# Record docs/assets/pdf-preview.gif (maintainer tool: needs vhs and poppler).
demo-pdf: build
    #!/usr/bin/env bash
    set -euo pipefail
    for tool in vhs pdftoppm; do command -v "$tool" >/dev/null || { echo "demo-pdf needs $tool" >&2; exit 1; }; done
    mkdir -p tmp
    home=$(mktemp -d "$PWD/tmp/pdf-demo-XXXXXX")
    cp -R examples/night-shift "$home/night-shift"
    # The recorded shell sources this while hidden, so the header reads ~/night-shift.
    printf 'export HOME=%q PATH=%q PS1=%q\n' "$home" "$PWD/bin:$PATH" '$ ' > tmp/pdf-demo.env
    PATH="$PWD/bin:$PATH" vhs demos/pdf.tape
