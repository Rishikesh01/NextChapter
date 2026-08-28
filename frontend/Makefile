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

.PHONY: clean
clean: ## Remove build and test output
	rm -rf .output .wxt playwright-report test-results blob-report

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

.PHONY: check-playwright-pin
check-playwright-pin: ## Assert @playwright/test == PLAYWRIGHT_VERSION == Dockerfile.test default
	@dep=$$(node -p "require('./package.json').devDependencies['@playwright/test']"); \
	if [ "$$dep" != "$(PLAYWRIGHT_VERSION)" ]; then \
		echo "playwright pin mismatch: package.json has $$dep, Makefile PLAYWRIGHT_VERSION is $(PLAYWRIGHT_VERSION)"; \
		exit 1; \
	fi; \
	dockerfile_default=$$(sed -n 's/^ARG PLAYWRIGHT_VERSION=//p' Dockerfile.test); \
	if [ "$$dockerfile_default" != "$(PLAYWRIGHT_VERSION)" ]; then \
		echo "playwright pin mismatch: Dockerfile.test ARG default is $$dockerfile_default, Makefile PLAYWRIGHT_VERSION is $(PLAYWRIGHT_VERSION)"; \
		exit 1; \
	fi
