# Switchboard — macOS only
#
#   make app             Build production Switchboard.app in ./bin
#   make run             Build a .app and launch it
#   make demo            Launch with fake tunnels for README screenshots
#   make dmg             Build Switchboard.app and wrap it in a .dmg
#   make dmg-universal   Universal (arm64+amd64) .app + .dmg
#   make icons           Regenerate .icns from build/appicon.png
#   make clean           Remove build outputs

APP_NAME                 ?= Switchboard
BIN_DIR                  ?= bin
APP_BUNDLE               := $(BIN_DIR)/$(APP_NAME).app
BINARY                   := $(BIN_DIR)/$(APP_NAME)
GOARCH                   ?= $(shell go env GOARCH)
MACOSX_DEPLOYMENT_TARGET ?= 12.0
PACKAGE_MANAGER          ?= npm
DEMO_DIR                 ?= $(CURDIR)/dev/runtime
DEMO_CONFIG              := $(DEMO_DIR)/tunnels.json

export GOOS := darwin
export CGO_ENABLED := 1
export MACOSX_DEPLOYMENT_TARGET
export CGO_CFLAGS := -mmacosx-version-min=$(MACOSX_DEPLOYMENT_TARGET)
export CGO_LDFLAGS := -mmacosx-version-min=$(MACOSX_DEPLOYMENT_TARGET)

.PHONY: all app package build frontend icons dmg dmg-universal universal run demo clean help open

all: app

help:
	@echo "Switchboard (macOS)"
	@echo ""
	@echo "  make app             Build ./$(APP_BUNDLE)"
	@echo "  make run             Build the .app and open it"
	@echo "  make demo            Screenshot fixtures (fake tunnels + statuses)"
	@echo "  make open            Open an already-built .app"
	@echo "  make dmg             Build a styled .dmg installer"
	@echo "  make dmg-universal   Universal (arm64+amd64) .app + .dmg"
	@echo "  make icons           Rebuild icons from build/appicon.png"
	@echo "  make clean           Remove ./$(BIN_DIR) outputs"
	@echo ""
	@echo "Optional: GOARCH=arm64|amd64  PACKAGE_MANAGER=npm|pnpm|yarn|bun"

# ---------------------------------------------------------------------------
# Primary targets
# ---------------------------------------------------------------------------

app package: $(APP_BUNDLE)
	@echo "→ $(APP_BUNDLE)"

build: $(BINARY)

frontend:
	@command -v wails3 >/dev/null || { echo "wails3 not found in PATH"; exit 1; }
	wails3 task common:build:frontend PACKAGE_MANAGER=$(PACKAGE_MANAGER)

icons: build/darwin/icons.icns

dmg: app
	wails3 task darwin:create:dmg
	@echo "→ $(BIN_DIR)/$(APP_NAME).dmg"

# Universal binary via lipo, then the same styled DMG packaging.
universal:
	@command -v wails3 >/dev/null || { echo "wails3 not found in PATH"; exit 1; }
	wails3 task darwin:package:universal
	@echo "→ $(APP_BUNDLE) (universal)"

dmg-universal: universal
	wails3 task darwin:create:dmg
	@echo "→ $(BIN_DIR)/$(APP_NAME).dmg"

run: app
	open "$(APP_BUNDLE)"

# Fake tunnels + painted statuses for README screenshots. Uses an isolated
# config under ./dev/runtime so your real tunnels.json is untouched.
# Launch the Mach-O directly so SWITCHBOARD_* env vars reach the process
# (plain `open Foo.app` does not forward them).
demo: app
	@mkdir -p "$(DEMO_DIR)"
	@cp dev/demo-tunnels.json "$(DEMO_CONFIG)"
	@echo "→ demo config $(DEMO_CONFIG)"
	SWITCHBOARD_CONFIG="$(DEMO_CONFIG)" SWITCHBOARD_DEMO=1 \
		"$(APP_BUNDLE)/Contents/MacOS/$(APP_NAME)"

open:
	@test -d "$(APP_BUNDLE)" || { echo "Missing $(APP_BUNDLE) — run make app first"; exit 1; }
	open "$(APP_BUNDLE)"

clean:
	rm -rf "$(BIN_DIR)/$(APP_NAME)" "$(APP_BUNDLE)" "$(BIN_DIR)/$(APP_NAME).dmg" \
		"$(BIN_DIR)/$(APP_NAME).dev.app" "$(BIN_DIR)/$(APP_NAME)-amd64" "$(BIN_DIR)/$(APP_NAME)-arm64"

# ---------------------------------------------------------------------------
# Build steps
# ---------------------------------------------------------------------------

$(BINARY): frontend build/darwin/icons.icns
	@mkdir -p "$(BIN_DIR)"
	GOARCH=$(GOARCH) go build -tags production -trimpath -buildvcs=false -ldflags="-w -s" \
		-o "$(BINARY)" .
	@echo "→ $(BINARY) ($(GOARCH))"

build/darwin/icons.icns: build/appicon.png
	@command -v wails3 >/dev/null || { echo "wails3 not found in PATH"; exit 1; }
	cd build && wails3 generate icons \
		-input appicon.png \
		-macfilename darwin/icons.icns \
		-windowsfilename /tmp/switchboard-unused.ico
	rm -f /tmp/switchboard-unused.ico build/darwin/Assets.car
	cp build/darwin/icons.icns build/darwin/dmg-file-icon.icns
	cp build/appicon.png build/darwin/dmg-file-icon.png
	cp build/appicon.png assets/appicon.png

$(APP_BUNDLE): $(BINARY) build/darwin/icons.icns build/darwin/Info.plist
	@rm -rf "$(APP_BUNDLE)"
	@mkdir -p "$(APP_BUNDLE)/Contents/MacOS"
	@mkdir -p "$(APP_BUNDLE)/Contents/Resources"
	cp build/darwin/Info.plist "$(APP_BUNDLE)/Contents/Info.plist"
	cp build/darwin/icons.icns "$(APP_BUNDLE)/Contents/Resources/icons.icns"
	cp "$(BINARY)" "$(APP_BUNDLE)/Contents/MacOS/$(APP_NAME)"
	codesign --force --deep --sign - "$(APP_BUNDLE)"
	@echo "→ ad-hoc signed $(APP_BUNDLE)"
	@# Drop Finder's icon cache for this bundle path so the new .icns shows up.
	@touch "$(APP_BUNDLE)"
	@/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister -f "$(APP_BUNDLE)" >/dev/null 2>&1 || true
