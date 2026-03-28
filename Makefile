BINARY   := ncore-cli
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -ldflags "-s -w -X main.version=$(VERSION)"

PLATFORMS := \
	darwin/amd64 \
	darwin/arm64 \
	linux/amd64 \
	linux/arm64 \
	windows/amd64

.PHONY: build dist clean

## build: build for the current platform
build:
	go build $(LDFLAGS) -o $(BINARY) .

## dist: cross-compile for all platforms into dist/
dist:
	@mkdir -p dist
	@$(foreach platform,$(PLATFORMS), \
		$(eval OS   := $(word 1,$(subst /, ,$(platform)))) \
		$(eval ARCH := $(word 2,$(subst /, ,$(platform)))) \
		$(eval OUT  := dist/$(BINARY)_$(OS)_$(ARCH)$(if $(filter windows,$(OS)),.exe,)) \
		echo "Building $(OUT) …"; \
		GOOS=$(OS) GOARCH=$(ARCH) go build $(LDFLAGS) -o $(OUT) . || exit 1; \
	)
	@echo "Done. Artifacts in dist/:"
	@ls -lh dist/

## clean: remove build artifacts
clean:
	rm -f $(BINARY)
	rm -rf dist/
