BINARY := slackctl
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X github.com/cluas/slackctl/internal/cli.version=$(VERSION)

.PHONY: build install clean test

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/slackctl/

install: build
	cp $(BINARY) $(GOPATH)/bin/ 2>/dev/null || cp $(BINARY) /usr/local/bin/

clean:
	rm -f $(BINARY)

test:
	go test ./...
