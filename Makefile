BINARY := slackctl
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X github.com/cluas/slackctl/internal/cli.version=$(VERSION)
DIST := dist

# Target platforms for cross-compilation (GOOS/GOARCH)
PLATFORMS := linux/amd64 linux/arm64 windows/amd64 windows/arm64 darwin/amd64 darwin/arm64

.PHONY: build install clean test cross

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/slackctl/

install: build
	cp $(BINARY) $(GOPATH)/bin/ 2>/dev/null || cp $(BINARY) /usr/local/bin/

# Cross-compile a binary for every platform in $(PLATFORMS) into $(DIST)/.
# Windows builds get a .exe suffix; each binary is named slackctl_<os>_<arch>.
cross:
	@rm -rf $(DIST) && mkdir -p $(DIST)
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; \
		ext=""; [ "$$os" = "windows" ] && ext=".exe"; \
		out=$(DIST)/$(BINARY)_$${os}_$${arch}$$ext; \
		echo "building $$out"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
			go build -ldflags "$(LDFLAGS)" -o $$out ./cmd/slackctl/ || exit 1; \
	done
	@echo "done -> $(DIST)/"

clean:
	rm -f $(BINARY)
	rm -rf $(DIST)

test:
	go test ./...
