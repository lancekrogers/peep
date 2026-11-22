# Unit tests, including race detection.
test:
    go test -race ./...

# Formatting, static analysis, unit tests, and build.
check:
    test -z "$(gofmt -l cmd internal)"
    go vet ./...
    go test -race ./...
    mkdir -p bin
    go build -o bin/peep ./cmd/peep

# Generate small synthetic media fixtures (no personal files).
fixtures:
    bash scripts/fixtures.sh

# Exercise actual ffmpeg and mpv using a null audio device.
test-integration: fixtures
    PEEP_INTEGRATION=1 go test -race -count=1 ./internal/media ./internal/player

# Exercise the real TUI in a PTY with synthetic fixtures.
test-tui: build fixtures
    python3 scripts/tui-smoke.py
