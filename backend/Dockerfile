# syntax=docker/dockerfile:1.7

# Build stage: pure-Go (CGO disabled), multi-arch friendly.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine@sha256:4c9fe60190a2a3350ddc51de80d0224b8a6698d12bdfc999fee45ea9d6c46dbc AS build
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
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
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
