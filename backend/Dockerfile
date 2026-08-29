# syntax=docker/dockerfile:1.7

# Build stage: pure-Go (CGO disabled), multi-arch friendly.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG TARGETOS
ARG TARGETARCH
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

RUN go build -trimpath \
    -ldflags="-s -w -X github.com/enable-it/nextchapter/backend/internal/config.defaultVersion=$VERSION" \
    -o /out/nextchapter ./cmd/nextchapter

# Final stage: distroless static, runs as nonroot per the ADR-mandated
# security posture for self-hosted deploys.
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/nextchapter /app/nextchapter
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/nextchapter"]
