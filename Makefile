# iRUN — cross-platform developer Makefile
#
# Default targets work on any OS with `go` and `make` installed.
# Windows users can keep using `build.bat` for parity.

# Windows builds append .exe automatically; POSIX leaves it bare.
ifeq ($(OS),Windows_NT)
EXE := .exe
endif

GO        ?= go
BIN_DIR   ?= bin
SERVER    := $(BIN_DIR)/iRUN$(EXE)
SCANNER   := $(BIN_DIR)/iRUN-find$(EXE)
CLIENT    := $(BIN_DIR)/sshr$(EXE)
IGO       := $(BIN_DIR)/igo$(EXE)

# Use the lowercased OS name for the build matrix label.
ifeq ($(OS),Windows_NT)
GOOS := windows
else
GOOS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
endif

.PHONY: all build server scanner client igo clean test vet fmt lint run-scan help

all: build ## Build all binaries into bin/

build: server scanner client igo ## Build all binaries into bin/

server: ## Build iRUN (SSH server)
	@mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -ldflags="-s -w" -o $(SERVER) .

scanner: ## Build iRUN-find (LAN scanner)
	@mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -ldflags="-s -w" -o $(SCANNER) ./find

client: ## Build sshr (SSH client)
	@mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -ldflags="-s -w" -o $(CLIENT) ./sshr

igo: ## Build igo (human iRUN connector with agent side-channel)
	@mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -ldflags="-s -w" -o $(IGO) ./igo

# ---- Quality ----

test: ## Run unit tests
	$(GO) test ./...

vet: ## Run go vet
	$(GO) vet ./...

fmt: ## Run gofmt -w on the source tree
	$(GO) fmt ./...

lint: fmt vet ## Format and vet (gofmt + go vet)
	@echo "[+] fmt + vet clean"

# ---- Operations ----

run-scan: scanner ## Build and run iRUN-find against the local /24(s)
	$(SCANNER)

clean: ## Remove the bin/ directory
	rm -rf $(BIN_DIR)

help: ## List available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'
