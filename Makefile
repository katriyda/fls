.PHONY: all clean build build-linux build-darwin build-darwin-arm build-windows

VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo "dev")
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags="-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

all: build-linux build-darwin build-darwin-arm build-windows

build:
	go build $(LDFLAGS) -o fls .

build-linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/fls-linux-amd64 .

build-darwin:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/fls-darwin-amd64 .

build-darwin-arm:
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/fls-darwin-arm64 .

build-windows:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/fls-windows-amd64.exe .

clean:
	rm -rf dist/
	rm -f fls fls.exe
