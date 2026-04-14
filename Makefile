GO ?= go
VERSION ?= dev
BIN_DIR ?= $(CURDIR)/bin
LOCAL_BIN ?= $(HOME)/.local/bin
LDFLAGS := -s -w -X github.com/jo-cube/toolbox/internal/buildinfo.version=$(VERSION)
ROCKSDB_PREFIX ?= $(shell if command -v pkg-config >/dev/null 2>&1 && pkg-config --exists rocksdb; then pkg-config --variable=prefix rocksdb; elif command -v brew >/dev/null 2>&1 && brew --prefix rocksdb >/dev/null 2>&1; then brew --prefix rocksdb; fi)
ROCKSDB_CGO_CFLAGS ?= $(shell if command -v pkg-config >/dev/null 2>&1 && pkg-config --exists rocksdb; then pkg-config --cflags rocksdb; elif [ -n "$(ROCKSDB_PREFIX)" ]; then printf '%s' '-I$(ROCKSDB_PREFIX)/include'; fi)
ROCKSDB_CGO_LDFLAGS ?= $(shell if command -v pkg-config >/dev/null 2>&1 && pkg-config --exists rocksdb; then pkg-config --libs rocksdb; elif [ -n "$(ROCKSDB_PREFIX)" ]; then printf '%s' '-L$(ROCKSDB_PREFIX)/lib -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy -llz4 -lzstd'; fi)
RDBSH_ENV = CGO_ENABLED=1 CGO_CFLAGS='$(ROCKSDB_CGO_CFLAGS)' CGO_LDFLAGS='$(ROCKSDB_CGO_LDFLAGS)'

.PHONY: build test run-hello run-ksetoff run-rdbsh install-hello install-ksetoff install-rdbsh clean

build:
	@mkdir -p "$(BIN_DIR)"
	$(GO) build -ldflags '$(LDFLAGS)' -o "$(BIN_DIR)/hello" ./cmd/hello
	$(GO) build -ldflags '$(LDFLAGS)' -o "$(BIN_DIR)/ksetoff" ./cmd/ksetoff
	$(RDBSH_ENV) $(GO) build -ldflags '$(LDFLAGS)' -o "$(BIN_DIR)/rdbsh" ./cmd/rdbsh

test:
	$(GO) test ./...

run-hello:
	$(GO) run -ldflags '$(LDFLAGS)' ./cmd/hello $(ARGS)

run-ksetoff:
	$(GO) run -ldflags '$(LDFLAGS)' ./cmd/ksetoff $(ARGS)

run-rdbsh:
	$(RDBSH_ENV) $(GO) run -ldflags '$(LDFLAGS)' ./cmd/rdbsh $(ARGS)

install-hello:
	@mkdir -p "$(LOCAL_BIN)"
	GOBIN="$(LOCAL_BIN)" $(GO) install -ldflags '$(LDFLAGS)' ./cmd/hello

install-ksetoff:
	@mkdir -p "$(LOCAL_BIN)"
	GOBIN="$(LOCAL_BIN)" $(GO) install -ldflags '$(LDFLAGS)' ./cmd/ksetoff

install-rdbsh:
	@mkdir -p "$(LOCAL_BIN)"
	$(RDBSH_ENV) GOBIN="$(LOCAL_BIN)" $(GO) install -ldflags '$(LDFLAGS)' ./cmd/rdbsh

clean:
	rm -rf "$(BIN_DIR)"
