SHELL := /bin/bash

# Pin to the version that the Go bindings track.
WHISPER_VERSION ?= v1.8.4
WHISPER_SRC    := third_party/whisper.cpp
WHISPER_BUILD  := $(WHISPER_SRC)/build

INCLUDE_PATHS := $(abspath $(WHISPER_SRC)/include):$(abspath $(WHISPER_SRC)/ggml/include)
LIBRARY_PATHS := $(abspath $(WHISPER_BUILD)/src):$(abspath $(WHISPER_BUILD)/ggml/src):$(abspath $(WHISPER_BUILD)/ggml/src/ggml-blas):$(abspath $(WHISPER_BUILD)/ggml/src/ggml-metal):$(abspath $(WHISPER_BUILD)/ggml/src/ggml-cpu)

export C_INCLUDE_PATH := $(INCLUDE_PATHS)
export LIBRARY_PATH   := $(LIBRARY_PATHS)
export CGO_LDFLAGS    := -L$(abspath $(WHISPER_BUILD)/src) -L$(abspath $(WHISPER_BUILD)/ggml/src) -L$(abspath $(WHISPER_BUILD)/ggml/src/ggml-blas) -L$(abspath $(WHISPER_BUILD)/ggml/src/ggml-metal) -L$(abspath $(WHISPER_BUILD)/ggml/src/ggml-cpu)

APP_BUNDLE  := build/bin/genie.app
APP_DEST    := /Applications
BIN_DEST    := /usr/local/bin

.PHONY: all whisper-clone whisper whisper-clean dev build run tidy clean icon \
        install install-cli uninstall launch

all: whisper build

# Clone whisper.cpp into third_party/ at the pinned tag.
$(WHISPER_SRC):
	@mkdir -p third_party
	git clone --depth 1 --branch $(WHISPER_VERSION) \
		https://github.com/ggerganov/whisper.cpp $(WHISPER_SRC)

whisper-clone: $(WHISPER_SRC)

# Build whisper.cpp with Metal (and embed the metallib so runtime needs no extras).
# We build through a /tmp symlink so that paths with non-ASCII characters do not
# trip up clang's Objective-C frontend (which has trouble with paths like "genie§").
WORKSPACE   := $(abspath .)
BUILD_LINK  := /tmp/genie-build-$(shell echo $(WORKSPACE) | shasum | cut -c1-8)

# whisper.cpp's ggml-metal CMakeLists doesn't propagate the ggml/include path to
# the C language compile flags, so its Objective-C (.m) files can't find ggml.h.
# We inject it via CMAKE_C_FLAGS so the .m files build cleanly.
GGML_INCLUDE := $(abspath $(WHISPER_SRC)/ggml/include)

whisper: $(WHISPER_SRC)
	@ln -sfn "$(WORKSPACE)" "$(BUILD_LINK)"
	@cd "$(BUILD_LINK)" && cmake -S "$(WHISPER_SRC)" -B "$(WHISPER_BUILD)" \
		-DCMAKE_BUILD_TYPE=Release \
		-DBUILD_SHARED_LIBS=OFF \
		-DGGML_METAL=ON \
		-DGGML_METAL_EMBED_LIBRARY=ON \
		-DWHISPER_BUILD_EXAMPLES=OFF \
		-DWHISPER_BUILD_TESTS=OFF \
		-DCMAKE_C_FLAGS="-I$(BUILD_LINK)/$(WHISPER_SRC)/ggml/include"
	@cd "$(BUILD_LINK)" && cmake --build "$(WHISPER_BUILD)" --target whisper -- -j

whisper-clean:
	rm -rf $(WHISPER_BUILD)

# Run wails dev (frontend hot reload). Whisper must already be built.
dev: whisper
	wails dev

# Production app bundle.
build: whisper
	wails build -clean

# Run the built binary directly (skips the .app wrapper).
run: whisper
	go run .

tidy:
	go mod tidy

# Regenerate build/appicon.png from scripts/make_icon.py.
icon:
	python3 scripts/make_icon.py build/appicon.png

clean: whisper-clean
	rm -rf build/bin frontend/dist

# Helpers so install targets transparently sudo when the target dir isn't
# writable by the current user (typical for /usr/local/bin on macOS). Walks up
# to the nearest existing ancestor before deciding.
SUDO = $(shell d="$(1)"; while [ ! -e "$$d" ]; do d=$$(dirname "$$d"); done; [ -w "$$d" ] || echo sudo)

# Copy the built .app to /Applications. Also makes Spotlight find it.
install: build
	@$(call SUDO,$(APP_DEST)) rm -rf "$(APP_DEST)/Genie.app"
	@$(call SUDO,$(APP_DEST)) cp -R "$(APP_BUNDLE)" "$(APP_DEST)/"
	@echo "installed $(APP_DEST)/Genie.app"

# Drop the `genie` shell wrapper into BIN_DEST so it's on $PATH.
# Override the location with: make install-cli BIN_DEST=$HOME/.local/bin
install-cli:
	@$(call SUDO,$(BIN_DEST)) mkdir -p "$(BIN_DEST)"
	@$(call SUDO,$(BIN_DEST)) install -m 0755 bin/genie "$(BIN_DEST)/genie"
	@echo "installed $(BIN_DEST)/genie"

uninstall:
	@$(call SUDO,$(APP_DEST)) rm -rf "$(APP_DEST)/Genie.app"
	@$(call SUDO,$(BIN_DEST)) rm -f "$(BIN_DEST)/genie"
	@echo "removed $(APP_DEST)/Genie.app and $(BIN_DEST)/genie"

# Open whichever Genie.app exists (installed copy preferred, dev build fallback).
launch:
	@if [ -d "$(APP_DEST)/Genie.app" ]; then \
		open "$(APP_DEST)/Genie.app"; \
	else \
		open "$(APP_BUNDLE)"; \
	fi
