BINARY := bml
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X bml/internal/cli.version=$(VERSION)"

# Install location. Override with `make install PREFIX=~/.local`.
PREFIX ?= /usr/local
BINDIR := $(PREFIX)/bin

.PHONY: build test vet install uninstall clean

build:
	go build $(LDFLAGS) -o $(BINARY) .

test:
	go test ./...

vet:
	go vet ./...

# Install globally to $(BINDIR) (default /usr/local/bin).
# Use `sudo make install` if that directory isn't writable.
install: build
	install -d $(DESTDIR)$(BINDIR)
	install -m 0755 $(BINARY) $(DESTDIR)$(BINDIR)/$(BINARY)
	@echo "installed $(BINARY) to $(DESTDIR)$(BINDIR)/$(BINARY)"

uninstall:
	rm -f $(DESTDIR)$(BINDIR)/$(BINARY)
	@echo "removed $(DESTDIR)$(BINDIR)/$(BINARY)"

clean:
	rm -f $(BINARY)
