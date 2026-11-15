GO ?= go
GOOS ?= windows
GOARCH ?= amd64
CGO_ENABLED ?= 0

DATETIME := $(shell date +'%Y%m%d-%H%M%S')
COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
VERSION ?= $(DATETIME)-$(COMMIT)
BUILDID ?= local

LDFLAGS := -X main.version=$(VERSION) -X main.buildid=$(BUILDID)
GO_ENV := CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH)

.PHONY: all runexe runner runexe.exe runner.exe clean

all: runexe.exe runner.exe

runexe: runexe.exe

runner: runner.exe

runexe.exe:
	$(GO_ENV) $(GO) build -o $@ -ldflags "$(LDFLAGS)" ./runexe

runner.exe:
	$(GO_ENV) $(GO) build -o $@ -ldflags "$(LDFLAGS)" ./runner

clean:
	rm -f runexe.exe runner.exe
