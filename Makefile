VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

.PHONY: build install test version

build:
	go build -ldflags "$(LDFLAGS)" -o resection ./cmd/resection

install: build
	go install -ldflags "$(LDFLAGS)" ./cmd/resection
	resection install

test:
	go test ./...

version:
	@echo $(VERSION)
