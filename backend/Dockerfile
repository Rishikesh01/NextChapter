# syntax=docker/dockerfile:1.7

# Build stage: pure-Go (CGO disabled), multi-arch friendly.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine@sha256:4c9fe60190a2a3350ddc51de80d0224b8a6698d12bdfc999fee45ea9d6c46dbc AS build
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
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
WORKDIR /app
COPY --from=build /out/nextchapter /app/nextchapter
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/nextchapter"]
