# NextChapter backend Makefile.
# All targets run from the backend/ directory.
#
# Quick start:
#   make help          # list every target with its description
#   make run           # start the server with defaults
#   make test-race     # the standard pre-PR check
#   make lint          # gofmt + vet + golangci-lint
#   make security-audit # govulncheck (see docs/adr/0007 for known deferred CVEs)

GO              ?= go
GOOSE           ?= go run github.com/pressly/goose/v3/cmd/goose@latest
SQLC            ?= sqlc
DB_FILE         ?= ./nextchapter.db
BIN_DIR         ?= ./bin
BIN_NAME        ?= nextchapter
DOCKER_IMAGE    ?= nextchapter:dev
HOST_PORT       ?= 8080
OPENAPI_SPEC    ?= ../docs/api/openapi.yaml

.DEFAULT_GOAL := help

.PHONY: help
help:  ## List available targets
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# ---- Dev workflow ----

.PHONY: run
run:  ## Run the server with default config (sqlite://./nextchapter.db on :8080)
	$(GO) run ./cmd/nextchapter

.PHONY: test
test:  ## Run all tests
	$(GO) test ./...

.PHONY: test-race
test-race:  ## Run all tests with the race detector
	$(GO) test -race ./...

.PHONY: fmt
fmt:  ## Format Go source (gofmt -w)
	gofmt -w .

.PHONY: fmt-check
fmt-check:  ## Fail if any file would be modified by gofmt
	@diff=$$(gofmt -l .); \
	if [ -n "$$diff" ]; then \
		echo "Unformatted files:"; echo "$$diff"; exit 1; \
	fi

.PHONY: vet
vet:  ## Run go vet
	$(GO) vet ./...

.PHONY: lint
lint: fmt-check vet  ## gofmt + go vet + golangci-lint (govulncheck is separate; see security-audit)
	golangci-lint run ./...

.PHONY: security-audit
security-audit:  ## Run govulncheck (reports deferred stdlib CVEs from docs/adr/0007)
	govulncheck ./...

.PHONY: tidy
tidy:  ## go mod tidy
	$(GO) mod tidy

# ---- Migration helpers ----
# Uses goose against an on-disk SQLite DB ($(DB_FILE)). Override DB_FILE for other targets.

.PHONY: migrate-up
migrate-up:  ## Apply all pending migrations to $(DB_FILE)
	$(GOOSE) -dir ./migrations sqlite3 $(DB_FILE) up

.PHONY: migrate-down
migrate-down:  ## Roll back the most recent migration on $(DB_FILE)
	$(GOOSE) -dir ./migrations sqlite3 $(DB_FILE) down

.PHONY: migrate-status
migrate-status:  ## Show migration status against $(DB_FILE)
	$(GOOSE) -dir ./migrations sqlite3 $(DB_FILE) status

.PHONY: migrate-create
migrate-create:  ## Create a new SQL migration. Usage: make migrate-create name=<short_name>
	@test -n "$(name)" || (echo "Usage: make migrate-create name=<short_name>" && exit 1)
	$(GOOSE) -dir ./migrations create $(name) sql

# ---- Codegen ----

.PHONY: sqlc
sqlc:  ## Regenerate sqlc bindings into internal/store/generated/
	$(SQLC) generate

.PHONY: openapi-validate
openapi-validate:  ## Lint $(OPENAPI_SPEC) using redocly via npx (no local install needed)
	npx --yes @redocly/cli@latest lint $(OPENAPI_SPEC)

# ---- Build / Docker ----

.PHONY: build
build:  ## Build the host binary into $(BIN_DIR)/$(BIN_NAME) (CGO_ENABLED=0)
	mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -o $(BIN_DIR)/$(BIN_NAME) ./cmd/nextchapter

.PHONY: clean
clean:  ## Remove build artefacts and the local DB
	rm -rf $(BIN_DIR) $(DB_FILE) $(DB_FILE)-shm $(DB_FILE)-wal

.PHONY: docker-build
docker-build:  ## Build the distroless Docker image as $(DOCKER_IMAGE)
	docker build -t $(DOCKER_IMAGE) .

.PHONY: docker-run
docker-run:  ## Run the Docker image with a persistent ./nextchapter.db on :$(HOST_PORT)
	docker run --rm -p $(HOST_PORT):8080 \
		-v $(CURDIR)/$(DB_FILE):/data/nextchapter.db \
		-e NEXTCHAPTER_DATABASE_URL=sqlite:///data/nextchapter.db \
		$(DOCKER_IMAGE)
