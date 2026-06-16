BINARY := bml
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X bml/internal/cli.version=$(VERSION)"

.PHONY: build test vet install clean

build:
	go build $(LDFLAGS) -o $(BINARY) .

test:
	go test ./...

vet:
	go vet ./...

install:
	go install $(LDFLAGS) .

clean:
	rm -f $(BINARY)
