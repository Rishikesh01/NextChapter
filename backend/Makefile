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
GOOSE           ?= go run github.com/pressly/goose/v3/cmd/goose@v3.27.1
SWAG            ?= go run github.com/swaggo/swag/cmd/swag@v1.16.2
GOLANGCI_LINT   ?= go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4
SQLC            ?= sqlc
DB_FILE         ?= ./nextchapter.db
BIN_DIR         ?= ./bin
BIN_NAME        ?= nextchapter
DOCKER_IMAGE    ?= nextchapter:dev
HOST_PORT       ?= 8080

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
	$(GOLANGCI_LINT) run ./...

.PHONY: security-audit
security-audit:  ## Run govulncheck (reports deferred stdlib CVEs from docs/adr/0007)
	govulncheck ./...

.PHONY: tidy
tidy:  ## go mod tidy
	$(GO) mod tidy

# ---- Migration helpers ----
# Uses goose against the on-disk SQLite DB ($(DB_FILE)) by default; the
# `pg-` variants point at a Postgres URL ($(PG_URL)). The two trees
# live side-by-side under migrations/sqlite/ and migrations/postgres/.

PG_URL ?= postgres://nextchapter:nextchapter@localhost:5432/nextchapter?sslmode=disable

.PHONY: migrate-up
migrate-up:  ## Apply all pending SQLite migrations to $(DB_FILE)
	$(GOOSE) -dir ./migrations/sqlite sqlite3 $(DB_FILE) up

.PHONY: migrate-down
migrate-down:  ## Roll back the most recent SQLite migration on $(DB_FILE)
	$(GOOSE) -dir ./migrations/sqlite sqlite3 $(DB_FILE) down

.PHONY: migrate-status
migrate-status:  ## Show SQLite migration status against $(DB_FILE)
	$(GOOSE) -dir ./migrations/sqlite sqlite3 $(DB_FILE) status

.PHONY: migrate-create
migrate-create:  ## Create a new SQLite SQL migration. Usage: make migrate-create name=<short_name>
	@test -n "$(name)" || (echo "Usage: make migrate-create name=<short_name>" && exit 1)
	$(GOOSE) -dir ./migrations/sqlite create $(name) sql

.PHONY: pg-migrate-up
pg-migrate-up:  ## Apply all pending Postgres migrations to $(PG_URL)
	$(GOOSE) -dir ./migrations/postgres postgres "$(PG_URL)" up

.PHONY: pg-migrate-down
pg-migrate-down:  ## Roll back the most recent Postgres migration on $(PG_URL)
	$(GOOSE) -dir ./migrations/postgres postgres "$(PG_URL)" down

.PHONY: pg-migrate-status
pg-migrate-status:  ## Show Postgres migration status against $(PG_URL)
	$(GOOSE) -dir ./migrations/postgres postgres "$(PG_URL)" status

.PHONY: pg-migrate-create
pg-migrate-create:  ## Create a new Postgres SQL migration. Usage: make pg-migrate-create name=<short_name>
	@test -n "$(name)" || (echo "Usage: make pg-migrate-create name=<short_name>" && exit 1)
	$(GOOSE) -dir ./migrations/postgres create $(name) sql

# ---- Local Postgres (docker compose) ----
# These wrap the repository-root docker-compose.yaml so an operator
# can stand up a Postgres for the Postgres-flavoured make-run /
# pg-migrate-* targets without keeping the compose verbs in muscle
# memory.

.PHONY: pg-up
pg-up:  ## Start the local Postgres container (docker compose)
	docker compose -f ../docker-compose.yaml up -d postgres

.PHONY: pg-down
pg-down:  ## Stop the local Postgres container (docker compose)
	docker compose -f ../docker-compose.yaml down

.PHONY: test-postgres
test-postgres:  ## Run integration tests with the Postgres testcontainer enabled (requires Docker)
	NEXTCHAPTER_TEST_POSTGRES=1 $(GO) test -race -count=1 ./tests/integration/...

# ---- Codegen ----

.PHONY: sqlc
sqlc:  ## Regenerate sqlc bindings into internal/store/generated/
	$(SQLC) generate

.PHONY: swagger
swagger:  ## Regenerate the OpenAPI spec from handler annotations.
	$(SWAG) init \
	    -g cmd/nextchapter/main.go \
	    -o internal/swaggerdocs \
	    --parseInternal --parseDependency=false

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
