# NextChapter — repository-root Makefile.
#
# The single entry point for anything spanning more than one track. Per-track
# detail stays in backend/Makefile and frontend/Makefile; this file delegates
# to them and owns the three things neither can:
#
#   * release artefacts — make dist        (backend + extension + SPA)
#   * the dev loop      — make setup / make dev-*
#   * disk hygiene      — make disk-report / make clean-*
#
#   make help    lists every target.

SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c

# Release artefacts are built one at a time: they share web/dist and the
# backend's embed directory, so a parallel build would race itself.
.NOTPARALLEL:

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

# The version is the git tag, always. An untagged tree gets `<sha>[-dirty]`,
# and a tree with no git at all gets "dev" — both are valid, neither is a
# release. ADR-0013.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

GO       ?= go
PNPM     ?= pnpm
DIST     ?= $(CURDIR)/dist
IMAGE    ?= ghcr.io/rishikesh01/nextchapter

GO_PKG   := github.com/enable-it/nextchapter/backend
LDFLAGS  := -s -w -X $(GO_PKG)/internal/config.defaultVersion=$(VERSION)

# Binary release matrix. linux/arm builds GOARM=7 — Raspberry Pi 2 and newer.
PLATFORMS        ?= linux/amd64 linux/arm64 linux/arm darwin/amd64 darwin/arm64 windows/amd64

# The container image now matches the Linux binaries arch-for-arch, including
# 32-bit ARM for a Pi on a 32-bit OS. The distroless base publishes all three.
DOCKER_PLATFORMS ?= linux/amd64,linux/arm64,linux/arm/v7
DOCKER_TAGS      ?= $(VERSION)

# OCI metadata baked into the image. REVISION/CREATED are resolved here rather
# than in the Dockerfile so a build from a tarball without git still works.
REVISION ?= $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
CREATED  ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
IMAGE_BUILD_ARGS = --build-arg VERSION='$(VERSION)' \
                   --build-arg REVISION='$(REVISION)' \
                   --build-arg CREATED='$(CREATED)'

.DEFAULT_GOAL := help

.PHONY: help
help:  ## List available targets
	@printf '\033[1mNextChapter\033[0m — VERSION=\033[36m%s\033[0m\n' '$(VERSION)'
	@awk 'BEGIN {FS = ":.*?## "} \
	     /^##@ / {printf "\n\033[1m%s\033[0m\n", substr($$0, 5)} \
	     /^[a-zA-Z0-9_-]+:.*?## / {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@printf '\nPer-track targets: \033[36mmake -C backend help\033[0m, \033[36mmake -C frontend help\033[0m\n'

##@ Dev setup

.PHONY: setup
setup: setup-go setup-node  ## Install everything needed to build, lint and unit-test

.PHONY: setup-go
setup-go:  ## Download the backend's Go modules
	cd backend && $(GO) mod download

.PHONY: setup-node
setup-node:  ## Install the pnpm workspace (frozen lockfile)
	$(MAKE) -C frontend install

.PHONY: setup-browsers
setup-browsers:  ## Download the pinned Playwright Chromium (~650 MB, only for native e2e)
	$(MAKE) -C frontend install-browsers

##@ Dev loop

.PHONY: dev-backend
dev-backend:  ## Run the API on :8080 against backend/nextchapter.db
	$(MAKE) -C backend run

.PHONY: dev-web
dev-web:  ## Run the SPA dev server (Vite proxies API paths to :8080)
	$(PNPM) --filter @nextchapter/web run dev

.PHONY: dev-extension
dev-extension:  ## Run the extension in an auto-reloading Chromium
	$(PNPM) --filter @nextchapter/extension run dev

.PHONY: lint
lint:  ## Lint both tracks
	$(MAKE) -C backend lint
	$(MAKE) -C frontend lint

.PHONY: test
test:  ## Unit + integration tests for both tracks (no Docker)
	$(MAKE) -C backend test-race
	$(MAKE) -C frontend test

.PHONY: test-e2e
test-e2e:  ## Both Playwright e2e gates in the pinned Docker image (needs Docker)
	$(MAKE) -C frontend test-e2e-docker
	$(MAKE) -C frontend web-test-e2e-docker

##@ Release artefacts

$(DIST):
	mkdir -p $(DIST)

.PHONY: dist
dist: dist-web dist-extension dist-backend checksums  ## Build every release artefact into dist/
	@printf '\n\033[1mdist/ (%s)\033[0m\n' '$(VERSION)'
	@ls -lh $(DIST)

.PHONY: dist-web
dist-web: | $(DIST)  ## SPA tarball for standalone hosting (nginx, a CDN, Caddy)
	$(MAKE) -C frontend web-build
	@stage='$(DIST)/nextchapter-web-$(VERSION)'; \
	rm -rf "$$stage" && mkdir -p "$$stage"; \
	cp -r web/dist/. "$$stage/"; \
	tar -czf "$$stage.tar.gz" -C '$(DIST)' "$$(basename "$$stage")"; \
	rm -rf "$$stage"; \
	echo "  → $$stage.tar.gz"

.PHONY: dist-extension
dist-extension: | $(DIST)  ## Chrome + Firefox zips, plus the sources zip AMO requires
	NEXTCHAPTER_VERSION='$(VERSION)' $(PNPM) --filter @nextchapter/extension exec wxt zip -b chrome
	NEXTCHAPTER_VERSION='$(VERSION)' $(PNPM) --filter @nextchapter/extension exec wxt zip -b firefox
	cp frontend/.output/chrome-mv3.zip          '$(DIST)/nextchapter-extension-$(VERSION)-chrome-mv3.zip'
	cp frontend/.output/firefox-mv3.zip         '$(DIST)/nextchapter-extension-$(VERSION)-firefox-mv3.zip'
	cp frontend/.output/firefox-mv3-sources.zip '$(DIST)/nextchapter-extension-$(VERSION)-firefox-sources.zip'

.PHONY: dist-backend
dist-backend: | $(DIST)  ## Cross-compiled server binaries with the SPA embedded
	$(MAKE) -C frontend web-embed
	@trap '$(MAKE) -C frontend web-unembed' EXIT; \
	for platform in $(PLATFORMS); do \
	  os="$${platform%%/*}"; arch="$${platform##*/}"; \
	  ext=''; case "$$os" in windows) ext='.exe' ;; esac; \
	  name="nextchapter_$(VERSION)_$${os}_$${arch}"; \
	  stage="$(DIST)/$$name"; \
	  rm -rf "$$stage" && mkdir -p "$$stage"; \
	  echo "  → $$os/$$arch"; \
	  ( cd backend && CGO_ENABLED=0 GOOS="$$os" GOARCH="$$arch" GOARM=7 \
	      $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o "$$stage/nextchapter$$ext" ./cmd/nextchapter ); \
	  cp LICENSE README.md "$$stage/"; \
	  if [ "$$os" = windows ]; then \
	    ( cd '$(DIST)' && zip -qr "$$name.zip" "$$name" ); \
	  else \
	    tar -czf "$$stage.tar.gz" -C '$(DIST)' "$$name"; \
	  fi; \
	  rm -rf "$$stage"; \
	done

.PHONY: checksums
checksums: | $(DIST)  ## sha256 every artefact in dist/ into dist/checksums.txt
	@cd '$(DIST)' && rm -f checksums.txt && \
	files="$$(ls -1 *.tar.gz *.zip 2>/dev/null || true)"; \
	if [ -z "$$files" ]; then echo "dist/ is empty — run 'make dist' first"; exit 1; fi; \
	sha256sum $$files > checksums.txt; \
	cat checksums.txt

.PHONY: docker-build
docker-build:  ## Build the release image for the host arch, SPA embedded
	$(MAKE) -C frontend web-embed
	@trap '$(MAKE) -C frontend web-unembed' EXIT; \
	docker build $(IMAGE_BUILD_ARGS) -t '$(IMAGE):$(VERSION)' backend

.PHONY: docker-push
docker-push:  ## Build and push the multi-arch image (requires a logged-in registry)
	$(MAKE) -C frontend web-embed
	@trap '$(MAKE) -C frontend web-unembed' EXIT; \
	docker buildx build --push \
	  --platform '$(DOCKER_PLATFORMS)' \
	  $(IMAGE_BUILD_ARGS) \
	  $(foreach t,$(DOCKER_TAGS),-t '$(IMAGE):$(t)') \
	  backend

##@ Disk hygiene

.PHONY: disk-report
disk-report:  ## Show what NextChapter currently costs on disk, and what reclaims it
	@scripts/disk-report.sh

.PHONY: clean
clean:  ## Remove build output from the working tree (always safe)
	$(MAKE) -C backend clean
	$(MAKE) -C frontend clean
	$(MAKE) -C frontend web-unembed
	rm -rf $(DIST)
	rm -f nextchapter.db nextchapter.db-shm nextchapter.db-wal

.PHONY: clean-deps
clean-deps: clean  ## …and node_modules (restore with `make setup-node`)
	rm -rf node_modules frontend/node_modules web/node_modules packages/*/node_modules
	rm -rf packages/api-client/.cache

.PHONY: clean-docker
clean-docker:  ## Drop NextChapter's images/containers and the WHOLE shared build cache
	@echo "Removing NextChapter's e2e images and containers…"
	-@docker ps -aq --filter 'ancestor=nextchapter-fe-test' --filter 'ancestor=nextchapter-web-test' \
	  | xargs -r docker rm -f
	-@docker images --format '{{.Repository}}:{{.Tag}}' \
	  | grep -E '^(nextchapter[-:]|$(subst .,\.,$(IMAGE)):)' \
	  | xargs -r docker rmi -f
	@echo "Pruning the Docker build cache (shared with your other projects, but regenerable)…"
	docker builder prune -af
	@echo "Untouched: every image not named nextchapter*, and all volumes."

.PHONY: clean-caches
clean-caches:  ## Clear the Go build cache (DEEP=1 also drops the Go module cache + pnpm store)
	$(GO) clean -cache
ifeq ($(DEEP),1)
	$(GO) clean -modcache
	$(PNPM) store prune
endif

.PHONY: clean-all
clean-all: clean-deps clean-docker clean-caches  ## Every clean target above
