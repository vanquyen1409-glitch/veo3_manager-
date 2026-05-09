# VEO3 Manager - cross-platform build automation.
#
# Cross-compiling Wails apps is NOT supported (each OS uses its own native
# webview + CGO toolchain). Each `make <os>` target therefore must run on
# the matching OS, OR you delegate to the GitHub Actions workflow at
# .github/workflows/build.yml which spawns a runner per OS.

VERSION ?= 0.1.0

.PHONY: help all windows windows-arm64 macos macos-intel macos-arm64 linux linux-arm64 ci-trigger clean dev bump-patch bump-minor bump-major

help:
	@echo "VEO3 Manager - build targets"
	@echo "  make windows           Windows amd64 .exe + NSIS installer (run on Windows)"
	@echo "  make windows-arm64     Windows arm64 + amd64 universal NSIS installer"
	@echo "  make macos             macOS universal .app + .dmg              (run on macOS)"
	@echo "  make macos-intel       macOS Intel .app + .dmg                  (run on macOS)"
	@echo "  make macos-arm64       macOS Apple Silicon .app + .dmg          (run on macOS)"
	@echo "  make linux             Linux amd64 binary + AppImage            (run on Linux/WSL2)"
	@echo "  make linux-arm64       Linux arm64 binary + AppImage            (run on Linux/WSL2)"
	@echo "  make ci-trigger        Push a release tag to fire .github/workflows/build.yml (all OSes)"
	@echo "  make all               Best-effort: build whatever the host OS supports"
	@echo "  make dev               Run wails dev (hot reload)"
	@echo "  make clean             Remove build/bin/"
	@echo "  make bump-patch        Bump SemVer patch  (0.1.0 -> 0.1.1)"
	@echo "  make bump-minor        Bump SemVer minor  (0.1.1 -> 0.2.0)"
	@echo "  make bump-major        Bump SemVer major  (0.2.0 -> 1.0.0)"
	@echo ""
	@echo "VERSION=$(VERSION) (override via 'make VERSION=0.2.0 windows')"

dev:
	wails dev

clean:
	rm -rf build/bin

# ---- Windows ----
windows:
	@powershell -NoProfile -ExecutionPolicy Bypass -File build/scripts/build.ps1 -Version $(VERSION)

windows-arm64:
	@powershell -NoProfile -ExecutionPolicy Bypass -File build/scripts/build.ps1 -Version $(VERSION) -Arch both

# ---- macOS ----
macos:
	@bash build/scripts/build-macos.sh $(VERSION) universal

macos-intel:
	@bash build/scripts/build-macos.sh $(VERSION) amd64

macos-arm64:
	@bash build/scripts/build-macos.sh $(VERSION) arm64

# ---- Linux ----
linux:
	@bash build/scripts/build-linux.sh $(VERSION) amd64

linux-arm64:
	@bash build/scripts/build-linux.sh $(VERSION) arm64

# ---- "All" : pragmatic per-host ----
# On Windows -> windows. On macOS -> macos. On Linux -> linux.
all:
	@case "$$(uname -s 2>/dev/null || echo Windows)" in \
	  Linux*)   $(MAKE) linux ;; \
	  Darwin*)  $(MAKE) macos ;; \
	  *)        $(MAKE) windows ;; \
	esac

# ---- SemVer bump (updates wails.json + frontend/package.json) ----
bump-patch:
	@powershell -NoProfile -ExecutionPolicy Bypass -File scripts/bump-version.ps1 patch

bump-minor:
	@powershell -NoProfile -ExecutionPolicy Bypass -File scripts/bump-version.ps1 minor

bump-major:
	@powershell -NoProfile -ExecutionPolicy Bypass -File scripts/bump-version.ps1 major

# ---- Trigger CI for true multi-OS release ----
ci-trigger:
	@echo "Tag and push to fire GitHub Actions multi-OS build:"
	@echo "  git tag v$(VERSION) && git push origin v$(VERSION)"
