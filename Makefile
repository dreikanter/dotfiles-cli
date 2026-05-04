.PHONY: build test lint clean install update

BINARY := dotfiles
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/dreikanter/dotfiles-cli/internal/cli.Version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/dotfiles

test:
	go test -coverprofile=coverage.out ./...

lint:
	go tool golangci-lint run

clean:
	rm -f $(BINARY) coverage.out

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/dotfiles

update:
	git checkout main
	git pull --tags
	$(MAKE) install
	@echo "Installed: $$(dotfiles --version)"
