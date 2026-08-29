# syntax=docker/dockerfile:1.7

# Build stage: pure-Go (CGO disabled), multi-arch friendly.
FROM --platform=$BUILDPLATFORM golang:1.27.0-alpine3.24 AS build
ARG TARGETOS
ARG TARGETARCH
# 32-bit ARM ships as linux/arm/v7 (Raspberry Pi 2 and up on a 32-bit OS);
# buildx passes the variant here and Go wants it as GOARM.
ARG TARGETVARIANT
# Stamped into the binary and reported by /healthz. The repository-root
# Makefile passes the git tag; an unstamped build reports "dev" (ADR-0013).
ARG VERSION=dev

ENV CGO_ENABLED=0 \
    GOOS=$TARGETOS \
    GOARCH=$TARGETARCH

WORKDIR /src

# Cache the module download layer.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN case "$TARGETVARIANT" in \
        v5) export GOARM=5 ;; \
        v6) export GOARM=6 ;; \
        v7) export GOARM=7 ;; \
    esac; \
    go build -trimpath \
      -ldflags="-s -w -X github.com/enable-it/nextchapter/backend/internal/config.defaultVersion=$VERSION" \
      -o /out/nextchapter ./cmd/nextchapter

# Final stage: distroless static, runs as nonroot per the ADR-mandated
# security posture for self-hosted deploys.
#
# This is the one base with no version tag to pin to — distroless publishes
# only `nonroot`/`latest`/`debug` plus tags named after its own source
# commits, which are no more readable than a digest. Floating `nonroot` is
# the right call anyway: this stage is CA certs, /etc/passwd and tzdata with
# no shell and no package manager, so tracking it means picking up cert and
# CVE updates rather than freezing them. Everything that determines the
# binary — the Go toolchain image, go.mod and go.sum — is pinned above.
FROM gcr.io/distroless/static-debian12:nonroot
ARG VERSION=dev
ARG REVISION=unknown
ARG CREATED=unknown

# OCI metadata. `version` is what `docker inspect` reports, and `source` is
# what makes GHCR attach the package to the repository — without it the
# package is an unlinked orphan that does not inherit the repo's settings.
LABEL org.opencontainers.image.title="NextChapter" \
      org.opencontainers.image.description="Self-hosted reading progress tracker for web novels, manhwa and manhua." \
      org.opencontainers.image.source="https://github.com/Rishikesh01/NextChapter" \
      org.opencontainers.image.url="https://github.com/Rishikesh01/NextChapter" \
      org.opencontainers.image.documentation="https://github.com/Rishikesh01/NextChapter#readme" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.version="$VERSION" \
      org.opencontainers.image.revision="$REVISION" \
      org.opencontainers.image.created="$CREATED"

WORKDIR /app
COPY --from=build /out/nextchapter /app/nextchapter
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/nextchapter"]
