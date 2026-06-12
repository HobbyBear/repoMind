.PHONY: build build-all test clean release

# Build for current platform (static binary, no glibc dependency)
build:
	CGO_ENABLED=0 go build -o repomind .

# Build all release binaries
build-all: build-linux-amd64 build-linux-arm64 \
	build-darwin-amd64 build-darwin-arm64 \
	build-windows-amd64 build-windows-arm64

build-linux-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o repomind-linux-amd64 .

build-linux-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o repomind-linux-arm64 .

build-darwin-amd64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o repomind-darwin-amd64 .

build-darwin-arm64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o repomind-darwin-arm64 .

build-windows-amd64:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o repomind-windows-amd64 .
	cp repomind-windows-amd64 repomind-windows-amd64.exe

build-windows-arm64:
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -o repomind-windows-arm64 .
	cp repomind-windows-arm64 repomind-windows-arm64.exe

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -f repomind
	rm -f repomind-*-*
