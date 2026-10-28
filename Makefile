BINARY  := session
VERSION ?= $(shell git describe --tags --always --dirty)
TAG     := ^v[0-9]+\.[0-9]+\.[0-9]+$$

.PHONY: build test vet install tag clean

build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY) ./cmd/session

test:
	go test ./...

vet:
	go vet ./...

install:
	go install ./cmd/session

# Tag a release: tests first, then an annotated tag pushed to origin.
#   make tag VERSION=v0.1.0
tag: test vet
	@test -n "$(VERSION)" || (echo "usage: make tag VERSION=v0.1.0" >&2; exit 1)
	@echo "$(VERSION)" | grep -qE '$(TAG)' || (echo "VERSION must look like v0.1.0" >&2; exit 1)
	@git rev-parse -q --verify "$(VERSION)^{commit}" >/dev/null && (echo "$(VERSION) already exists" >&2; exit 1) ||:
	git tag -a "$(VERSION)" -m "$(VERSION)"
	git push origin "$(VERSION)"

clean:
	rm -f $(BINARY)
