# Build the local binary.
build:
    mkdir -p bin
    go build -trimpath -o bin/peep ./cmd/peep

# Install to Go's configured bin directory.
install:
    go install ./cmd/peep

# Compile the supported OS/architecture combinations.
build-all:
    mkdir -p dist
    for os in darwin linux; do for arch in arm64 amd64; do GOOS=$os GOARCH=$arch CGO_ENABLED=0 go build -trimpath -o dist/peep-$os-$arch ./cmd/peep; done; done

# Optional Mermaid helper; separate from peep's Go 1.24 build (requires Go 1.27+).
install-mermaid:
    CGO_ENABLED=0 go install github.com/bvolpato/mermaid-go-renderer/cmd/mmdg@v0.13.0
