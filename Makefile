.DEFAULT_GOAL := help

GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

BASE := $(shell cat VERSION 2>/dev/null || echo 0.0.0)
GIT_TAG := $(shell git describe --tags --exact-match 2>/dev/null)
EPOCH := $(shell date -u +%s)
BUILT_FROM := $(shell git status --porcelain 2>/dev/null | grep -q . && echo dirty || echo $(GIT_COMMIT))
VERSION ?= $(if $(GIT_TAG),$(GIT_TAG),$(BASE)-dev.$(EPOCH)_$(BUILT_FROM))

REPO ?= ygelfand/LANovo
RELEASES ?= https://github.com/$(REPO)/releases

BUILDVARS := github.com/ygelfand/LANovo/internal/layout
LDFLAGS := -X '$(BUILDVARS).Version=$(VERSION)' \
	-X '$(BUILDVARS).GitCommit=$(GIT_COMMIT)' \
	-X '$(BUILDVARS).BuildDate=$(BUILD_DATE)'

comma := ,
TAGS ?=

BUILD_DIR := bin
ASSET_DIR := internal/host/assets/payload

# lanovod targets the Lenovo Smart Display: APQ8053, Android Things 1.1 on Android 8.1 (API 27).
DEVICE_ARCH ?= arm
DEVICE_ENV := GOOS=linux GOARCH=$(DEVICE_ARCH) GOARM=7 CGO_ENABLED=0
DEVICE_LDFLAGS := -s -w $(LDFLAGS)
DEVICE_BIN := $(BUILD_DIR)/lanovod

ADB ?= adb
DEVICE_TMP := /data/local/tmp

DEVICE ?=
EXPECT_PLATFORM ?= msm8x53 mt8167

ifneq ($(DEVICE),)
ADB := $(ADB) -s $(DEVICE)
endif

# init takes a service's SELinux domain from the label of the file it execs; /system/bin is system_file.
LANOVOD := /system/bin/lanovod
STATE_DIR := /data/misc/lanovo

# / is the system image on this system-as-root device; its block device carries a read-only flag a plain remount does not clear.
SETRW = blockdev --setrw $$(ls -d /dev/block/bootdevice/by-name /dev/block/platform/bootdevice/by-name 2>/dev/null | head -1)/system$$(getprop ro.boot.slot_suffix)

##@ Development

NDK ?= $(firstword $(ANDROID_NDK_HOME) $(ANDROID_NDK_LATEST_HOME) $(wildcard /opt/homebrew/Caskroom/android-ndk/*/AndroidNDK*.app/Contents/NDK))
NDK_HOST := $(if $(filter Darwin,$(shell uname -s)),darwin-x86_64,linux-x86_64)
BOARD ?=
NATIVE_API = $(shell go run ./cmd/native-target $(BOARD))
LIBCOUNTERTOP_MODULE := github.com/ygelfand/libcountertop
LIBCOUNTERTOP_DIR = $(shell go list -m -f '{{.Dir}}' $(LIBCOUNTERTOP_MODULE))
SURFACE_SRC = $(LIBCOUNTERTOP_DIR)/native/surface
SURFACE_BIN = $(BUILD_DIR)/lanovo-surface-api$(NATIVE_API)
SURFACE := /system/bin/lanovo-surface
CAMSHIM_BIN = $(BUILD_DIR)/liblanovo-camshim-api$(NATIVE_API).so
CAMERA_BIN = $(BUILD_DIR)/lanovo-camera-api$(NATIVE_API)
COUNTERTOP_NATIVE = $(MAKE) -f "$(LIBCOUNTERTOP_DIR)/native/Makefile" API="$(NATIVE_API)" PREFIX=lanovo OUT="$(abspath $(BUILD_DIR))" NDK="$(NDK)" SURFACE_BIN="$(abspath $(SURFACE_BIN))" CAMERA_BIN="$(abspath $(CAMERA_BIN))" CAMSHIM_BIN="$(abspath $(CAMSHIM_BIN))"
PARTS_DIR := internal/parts/payload

.PHONY: build
build: build-lanovoctl build-lanovod ## Build everything

.PHONY: build-lanovoctl
build-lanovoctl: ## Build the host CLI into ./bin
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/lanovoctl ./cmd/lanovoctl

.PHONY: build-lanovod
build-lanovod: build-surface build-camshim build-camera ## Cross-compile lanovod for the display, carrying its native parts
	@mkdir -p $(BUILD_DIR) $(PARTS_DIR)
	cp $(SURFACE_BIN) $(PARTS_DIR)/lanovo-surface
	cp $(CAMSHIM_BIN) $(PARTS_DIR)/liblanovo-camshim.so
	cp $(CAMERA_BIN) $(PARTS_DIR)/lanovo-camera
	$(DEVICE_ENV) go build -tags "payload$(if $(TAGS),$(comma)$(TAGS))" -ldflags "$(DEVICE_LDFLAGS)" -o $(DEVICE_BIN) ./cmd/lanovod

# go list may know the version before its source archive has been downloaded.
.PHONY: countertop-native-source
countertop-native-source:
	@test -n "$(LIBCOUNTERTOP_DIR)" || go mod download $(LIBCOUNTERTOP_MODULE)

.PHONY: build-surface
build-surface: countertop-native-source ## Build the SurfaceFlinger helper with the NDK
	+$(COUNTERTOP_NATIVE) surface

.PHONY: build-camshim
build-camshim: countertop-native-source ## Build the camera service preload with the NDK
	+$(COUNTERTOP_NATIVE) camshim

.PHONY: build-camera
build-camera: countertop-native-source ## Build the camera helper with the NDK
	+$(COUNTERTOP_NATIVE) camera

.PHONY: run-lanovoctl
run-lanovoctl: ## Run lanovoctl on the host (make run-lanovoctl ARGS="check")
	go run ./cmd/lanovoctl $(ARGS)

.PHONY: push-lanovod
push-lanovod: build-lanovod ## Push lanovod to /data/local/tmp for iteration
	@$(ADB) push $(DEVICE_BIN) $(DEVICE_TMP)/lanovod >/dev/null
	@$(ADB) shell chmod 755 $(DEVICE_TMP)/lanovod

TIMEOUT ?= 90s

PKG ?= ./...

.PHONY: test
test: ## Run tests (make test PKG=./internal/... TIMEOUT=30s)
	go test -timeout $(TIMEOUT) $(PKG)

HOST_CC ?= cc

.PHONY: test-native
test-native: countertop-native-source ## Run the shared native wire tests with the host compiler
	+$(COUNTERTOP_NATIVE) test HOST_CC="$(or $(HOST_CC),cc)"

.PHONY: test-race
test-race: ## Run tests with the race detector
	go test -race -timeout $(TIMEOUT) $(PKG)

.PHONY: fmt
fmt: ## Format Go source
	golangci-lint fmt

.PHONY: fmt-check
fmt-check: ## Fail when any Go source is unformatted
	golangci-lint fmt --diff

.PHONY: vet
vet: ## Run go vet
	go vet ./...
	$(DEVICE_ENV) go vet ./...

.PHONY: lint
lint: ## Run golangci-lint
	@if command -v golangci-lint >/dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping"; \
	fi

.PHONY: tidy
tidy: ## Tidy go.mod / go.sum
	go mod tidy

.PHONY: check
check: fmt vet lint test ## Format, vet, lint and test

##@ Generate

.PHONY: thumbs
thumbs: device ## Render the visual picker's thumbnails on the device into what the binary embeds
	$(ADB) shell rm -rf /data/local/tmp/lanovo-thumbs
	$(ADB) shell lanovod ctl display thumbs /data/local/tmp/lanovo-thumbs
	rm -f $(CURDIR)/internal/ui/visual/thumbs/*.jpg
	$(ADB) pull /data/local/tmp/lanovo-thumbs/. $(CURDIR)/internal/ui/visual/thumbs/

MARK_SIZE ?= 1000x666!

.PHONY: logo
logo: ## Rescale the light and dark marks from assets/ into what the binary embeds
	magick assets/logo_light.png -resize $(MARK_SIZE) -colors 256 internal/ui/logo_light.png
	magick assets/logo_dark.png -resize $(MARK_SIZE) -colors 256 internal/ui/logo_dark.png


##@ Device (lanovod)

.PHONY: device
device: ## Say which device the device targets would write to, and refuse the wrong one
	@n=$$($(ADB) devices | grep -c "device$$"); \
	if [ "$$n" -eq 0 ]; then \
		echo "no device attached"; exit 1; \
	fi; \
	if [ "$$n" -gt 1 ] && [ -z "$(DEVICE)" ]; then \
		echo "$$n devices attached, so name one:"; \
		$(ADB) devices | grep "device$$" | sed 's/^/  make DEVICE=/;s/\tdevice$$//'; \
		exit 1; \
	fi; \
	platform=$$($(ADB) shell getprop ro.board.platform | tr -d '\r'); \
	model=$$($(ADB) shell getprop ro.product.model | tr -d '\r'); \
	if [ -n "$(EXPECT_PLATFORM)" ] && ! echo " $(EXPECT_PLATFORM) " | grep -q " $$platform "; then \
		echo "refusing: $$model is a $$platform, wanted one of: $(EXPECT_PLATFORM)"; \
		echo "these targets remount / and write to /system, so this is not a mistake to make twice"; \
		echo "override with EXPECT_PLATFORM= if that is wrong"; \
		exit 1; \
	fi; \
	echo "device: $$model ($$platform)"

.PHONY: payload
payload: ## Stage lanovod for embedding into lanovoctl
	@$(MAKE) --no-print-directory build-lanovod
	@mkdir -p $(ASSET_DIR)
	cp $(DEVICE_BIN) $(ASSET_DIR)/lanovod
	@shasum -a 256 $(ASSET_DIR)/lanovod | awk '{print $$1}' > $(ASSET_DIR)/lanovod.sha256

.PHONY: dist
dist: payload ## Full build: lanovod, then lanovoctl carrying it
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -tags payload -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/lanovoctl ./cmd/lanovoctl

.PHONY: install-lanovod
install-lanovod: build-lanovod device ## Install lanovod, which puts its other parts in place as it starts
	@$(ADB) shell 'setprop ctl.stop lanovod; sleep 1'
	@$(ADB) shell '$(SETRW) && mount -o remount,rw /'
	@$(ADB) push $(DEVICE_BIN) $(LANOVOD) >/dev/null
	@$(ADB) shell 'chmod 755 $(LANOVOD); restorecon $(LANOVOD); sync; mount -o remount,ro /; ls -lZ $(LANOVOD)'
	@$(ADB) shell 'setprop ctl.start lanovod; sleep 1; echo "init.svc: lanovod $$(getprop init.svc.lanovod)"'

.PHONY: restart
restart: ## Restart lanovod through init
	@$(ADB) shell 'setprop ctl.stop lanovod; sleep 1; setprop ctl.start lanovod; \
		sleep 1; echo "init.svc: $$(getprop init.svc.lanovod)"'

.PHONY: state
state: ## Show what init says about lanovod
	@$(ADB) shell 'echo "init.svc: $$(getprop init.svc.lanovod)"; \
		echo "started:  $$(getprop ro.boottime.lanovod)"; \
		echo "selinux:  $$(getenforce)"; \
		echo "backlight: $$(cat /sys/class/leds/wled/brightness)"'

.PHONY: logs
logs: ## Tail lanovod logs from a connected display
	$(ADB) logcat -s lanovod:*

.PHONY: shell
shell: ## Open a root shell on a connected display
	$(ADB) shell

##@ Build & Release

AT ?= $(VERSION)
FROM ?= $(RELEASES)/download/$(AT)
PAGE ?= $(RELEASES)/tag/$(AT)

.PHONY: manifest
manifest: build-lanovod ## Write the manifest a device fetches to find this build
	go run ./cmd/mkmanifest \
		-version "$(AT)" \
		-from "$(FROM)" \
		-arm $(DEVICE_BIN) \
		-title "LANovo $(AT)" \
		-release-url "$(PAGE)" \
		-out $(BUILD_DIR)/manifest.json
	@cat $(BUILD_DIR)/manifest.json

.PHONY: release-dev
release-dev: ## Publish this working tree to the dev channel, without pushing anything
	@command -v gh >/dev/null || { echo "needs the gh CLI: brew install gh"; exit 1; }
	@command -v goreleaser >/dev/null || { echo "needs goreleaser: brew install goreleaser"; exit 1; }
	VERSION=$(VERSION) goreleaser release --snapshot --clean
	@for f in dist/lanovoctl_*/lanovoctl dist/lanovoctl_*/lanovoctl.exe; do \
		[ -f "$$f" ] || continue; \
		d=$$(basename $$(dirname $$f)); \
		os=$$(echo $$d | cut -d_ -f2); \
		arch=$$(echo $$d | cut -d_ -f3); \
		if [ "$$arch" = amd64 ]; then arch=x86_64; fi; \
		ext=$${f##*.}; [ "$$ext" = exe ] && ext=.exe || ext=; \
		cp "$$f" "dist/lanovo_$${os}_$${arch}$${ext}"; \
	done
	@gh release view dev >/dev/null 2>&1 || \
		gh release create dev --prerelease --title dev --notes "Rolling build from the dev channel."
	@$(MAKE) --no-print-directory manifest VERSION=$(VERSION) FROM=$(RELEASES)/download/dev PAGE=$(RELEASES)/tag/dev
	gh release upload dev dist/lanovo_* $(DEVICE_BIN) $(BUILD_DIR)/manifest.json --clobber
	@echo "dev channel now serves $(VERSION)"

.PHONY: release
release: ## Release the version in VERSION
	@$(MAKE) --no-print-directory tag TAG=$(BASE)

.PHONY: tag
tag:
	@test -n "$(TAG)" || { echo "usage: make release, make release-dev, or make tag TAG=0.0.2"; exit 1; }
	@test -z "$$(git status --porcelain)" || { echo "the working tree is dirty"; exit 1; }
	@git rev-parse -q --verify "refs/tags/$(TAG)" >/dev/null && { echo "$(TAG) already exists"; exit 1; } || true
	git tag -a "$(TAG)" -m "LANovo $(TAG)"
	git push origin "$(TAG)"
	@echo "pushed $(TAG) — the release workflow builds it from here"

.PHONY: snapshot
snapshot: ## Build a local goreleaser snapshot
	goreleaser release --snapshot --clean

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf $(BUILD_DIR) dist/ coverage.out
	rm -rf $(ASSET_DIR)
	go clean

##@ Help

.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
