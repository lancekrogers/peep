#!/usr/bin/env bash
# Run a real clipboard round trip on a disposable Weston compositor nested in Xvfb.
set -euo pipefail
runtime_dir=$(mktemp -d)
chmod 700 "$runtime_dir"
export XDG_RUNTIME_DIR="$runtime_dir" WAYLAND_DISPLAY=peep-test-wayland
weston --backend=x11-backend.so --socket="$WAYLAND_DISPLAY" --idle-time=0 --log="$runtime_dir/weston.log" &
compositor=$!
trap 'kill "$compositor" 2>/dev/null || true; wait "$compositor" 2>/dev/null || true; rm -rf "$runtime_dir"' EXIT
for i in {1..100}; do
    test -S "$runtime_dir/$WAYLAND_DISPLAY" && break
    sleep .1
done
test -S "$runtime_dir/$WAYLAND_DISPLAY"
PEEP_DESKTOP_INTEGRATION=1 "$@"
