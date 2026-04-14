GO ?= go
VERSION ?= dev
BIN_DIR ?= $(CURDIR)/bin
LOCAL_BIN ?= $(HOME)/.local/bin
LDFLAGS := -s -w -X github.com/jo-cube/toolbox/internal/buildinfo.version=$(VERSION)

.PHONY: build test run-hello install-hello clean

build:
	@mkdir -p "$(BIN_DIR)"
	$(GO) build -ldflags '$(LDFLAGS)' -o "$(BIN_DIR)/hello" ./cmd/hello

test:
	$(GO) test ./...

run-hello:
	$(GO) run -ldflags '$(LDFLAGS)' ./cmd/hello

install-hello:
	@mkdir -p "$(LOCAL_BIN)"
	GOBIN="$(LOCAL_BIN)" $(GO) install -ldflags '$(LDFLAGS)' ./cmd/hello

clean:
	rm -rf "$(BIN_DIR)"
