# NextChapter frontend track — extension (frontend/) + shared packages (packages/).
# The pnpm workspace root is the repository root; this Makefile is the single
# entry point for every frontend workflow. CI calls these targets and nothing else.

.DEFAULT_GOAL := help

# ---------------------------------------------------------------------------
# Tools & config (override like: make PNPM=corepack-pnpm test)
# ---------------------------------------------------------------------------

PNPM ?= pnpm

# Must equal the devDependency version of @playwright/test (check-playwright-pin
# asserts it). Also the tag of mcr.microsoft.com/playwright used by Dockerfile.test.
PLAYWRIGHT_VERSION ?= 1.62.1

DOCKER_IMAGE ?= nextchapter-fe-test
WEB_DOCKER_IMAGE ?= nextchapter-web-test

# ---------------------------------------------------------------------------
# Dev workflow
# ---------------------------------------------------------------------------

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z0-9_-]+:.*##/ {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: install
install: ## Install workspace dependencies (frozen lockfile)
	$(PNPM) install --frozen-lockfile

.PHONY: lint
lint: lint-eslint lint-prettier typecheck depcheck ## Run all static analysis (the linter gate)

.PHONY: lint-eslint
lint-eslint: ## ESLint over the workspace, zero warnings allowed
	$(PNPM) -w exec eslint . --max-warnings 0

.PHONY: lint-prettier
lint-prettier: ## Prettier formatting check
	$(PNPM) -w exec prettier --check .

.PHONY: typecheck
typecheck: ## tsc --noEmit in every package (runs wxt prepare first)
	$(PNPM) --filter @nextchapter/extension exec wxt prepare
	$(PNPM) -r --no-bail exec tsc --noEmit

.PHONY: depcheck
depcheck: ## Unused/missing dependency check per package
	$(PNPM) -w exec depcheck frontend
	$(PNPM) -w exec depcheck packages/api-client
	$(PNPM) -w exec depcheck web

.PHONY: fmt
fmt: ## Apply prettier formatting
	$(PNPM) -w exec prettier --write .

.PHONY: test
test: ## Unit tests (Vitest) in every package
	$(PNPM) -r run test

# ---------------------------------------------------------------------------
# Codegen
# ---------------------------------------------------------------------------

.PHONY: api-types
api-types: ## Regenerate API types from backend/internal/swaggerdocs/swagger.yaml
	$(PNPM) --filter @nextchapter/api-client run generate

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------

.PHONY: build
build: ## Production build for Chromium (.output/chrome-mv3)
	$(PNPM) --filter @nextchapter/extension run build

.PHONY: build-firefox
build-firefox: ## Production build for Firefox (.output/firefox-mv3)
	$(PNPM) --filter @nextchapter/extension run build:firefox

.PHONY: web-build
web-build: ## Production build of the web library SPA (web/dist)
	$(PNPM) --filter @nextchapter/web run build

.PHONY: web-embed
web-embed: web-build ## Copy web/dist into the backend's embed dir (release builds; ADR-0010)
	rm -rf ../backend/internal/webui/dist
	mkdir -p ../backend/internal/webui/dist
	cp -r ../web/dist/. ../backend/internal/webui/dist/

.PHONY: web-unembed
web-unembed: ## Restore the committed placeholder in the backend's embed dir
	rm -rf ../backend/internal/webui/dist
	cd .. && git checkout -- backend/internal/webui/dist

.PHONY: clean
clean: ## Remove build and test output
	rm -rf .output .wxt playwright-report test-results blob-report ../web/dist ../web/playwright-report ../web/test-results

# ---------------------------------------------------------------------------
# e2e
# ---------------------------------------------------------------------------

.PHONY: install-browsers
install-browsers: ## Download the pinned Playwright Chromium for local e2e runs
	$(PNPM) --filter @nextchapter/extension exec playwright install chromium

.PHONY: test-e2e
test-e2e: ## e2e against a freshly built backend + test-mode extension (native, dev loop)
	$(MAKE) -C ../backend build
	$(PNPM) --filter @nextchapter/extension exec wxt build -b chrome --mode test
	$(PNPM) --filter @nextchapter/extension exec playwright test

.PHONY: test-e2e-docker
test-e2e-docker: check-playwright-pin ## e2e in the pinned Playwright image (the qa gate)
	docker build --build-arg PLAYWRIGHT_VERSION=$(PLAYWRIGHT_VERSION) -f Dockerfile.test -t $(DOCKER_IMAGE) ..
	docker run --rm $(DOCKER_IMAGE)

.PHONY: web-test-e2e
web-test-e2e: web-embed ## Web e2e against the embedded binary (native, dev loop)
	$(MAKE) -C ../backend build
	$(PNPM) --filter @nextchapter/web exec playwright test

.PHONY: web-test-e2e-docker
web-test-e2e-docker: check-playwright-pin ## Web e2e in the pinned Playwright image (the qa gate)
	docker build --build-arg PLAYWRIGHT_VERSION=$(PLAYWRIGHT_VERSION) -f ../web/Dockerfile.test -t $(WEB_DOCKER_IMAGE) ..
	docker run --rm $(WEB_DOCKER_IMAGE)

.PHONY: check-playwright-pin
check-playwright-pin: ## Assert every @playwright/test dep and Dockerfile ARG == PLAYWRIGHT_VERSION
	@for pkg in ./package.json ../web/package.json; do \
		dep=$$(node -p "require('$$pkg').devDependencies['@playwright/test']"); \
		if [ "$$dep" != "$(PLAYWRIGHT_VERSION)" ]; then \
			echo "playwright pin mismatch: $$pkg has $$dep, Makefile PLAYWRIGHT_VERSION is $(PLAYWRIGHT_VERSION)"; \
			exit 1; \
		fi; \
	done; \
	for df in Dockerfile.test ../web/Dockerfile.test; do \
		dockerfile_default=$$(sed -n 's/^ARG PLAYWRIGHT_VERSION=//p' $$df); \
		if [ "$$dockerfile_default" != "$(PLAYWRIGHT_VERSION)" ]; then \
			echo "playwright pin mismatch: $$df ARG default is $$dockerfile_default, Makefile PLAYWRIGHT_VERSION is $(PLAYWRIGHT_VERSION)"; \
			exit 1; \
		fi; \
	done
