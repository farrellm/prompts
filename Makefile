BINARY := prompts
PKG    := ./cmd/prompts

# Where `go install` puts things: $GOBIN if set, else $GOPATH/bin.
BINDIR := $(shell go env GOBIN)
ifeq ($(BINDIR),)
BINDIR := $(shell go env GOPATH)/bin
endif

.PHONY: all build install uninstall run test vet fmt fmt-check check clean

all: build

build:
	go build -o $(BINARY) $(PKG)

install:
	go install $(PKG)
	@echo "installed $(BINARY) to $(BINDIR)"

uninstall:
	rm -f $(BINDIR)/$(BINARY)

run:
	go run $(PKG)

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

# gofmt -l lists files needing formatting and exits 0 either way, so the
# non-empty case has to be turned into a failure by hand.
fmt-check:
	@files=$$(gofmt -l .); \
	if [ -n "$$files" ]; then \
		echo "not gofmt'd:"; echo "$$files"; exit 1; \
	fi

# The whole gate. There is no linter or CI beyond this.
check: build vet fmt-check test

clean:
	rm -f $(BINARY)
