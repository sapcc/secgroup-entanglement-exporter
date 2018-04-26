BINARY = secgroup-entanglement-exporter
PKG    = github.com/sapcc/$(BINARY)
PREFIX := /usr

all: build

# force people to use golangvend
GO            := GOPATH=$(CURDIR)/.gopath GOBIN=$(CURDIR)/build go
GO_BUILDFLAGS :=
GO_LDFLAGS    := -s -w

build: FORCE
	$(GO) install $(GO_BUILDFLAGS) -ldflags '$(GO_LDFLAGS)' '$(PKG)'

install: FORCE build
	install -D -m 0755 build/$(BINARY) "$(DESTDIR)$(PREFIX)/bin/$(BINARY)"

vendor: FORCE
	# vendoring by https://github.com/holocm/golangvend
	@golangvend

.PHONY: FORCE
