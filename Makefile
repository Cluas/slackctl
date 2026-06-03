BINARY := slackctl
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X github.com/cluas/slackctl/internal/cli.version=$(VERSION)

.PHONY: build install clean test build-windows-amd64 build-windows-arm64 build-windows

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/slackctl/

build-windows-amd64:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)_windows_amd64.exe ./cmd/slackctl/

build-windows-arm64:
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)_windows_arm64.exe ./cmd/slackctl/

build-windows: build-windows-amd64 build-windows-arm64

install: build
	cp $(BINARY) $(GOPATH)/bin/ 2>/dev/null || cp $(BINARY) /usr/local/bin/

clean:
	rm -f $(BINARY) $(BINARY)_windows_amd64.exe $(BINARY)_windows_arm64.exe

test:
	go test ./...
