# syntax=docker/dockerfile:1.7

# Build stage: pure-Go (CGO disabled), multi-arch friendly.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG TARGETOS
ARG TARGETARCH

ENV CGO_ENABLED=0 \
    GOOS=$TARGETOS \
    GOARCH=$TARGETARCH

WORKDIR /src

# Cache the module download layer.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -trimpath -ldflags="-s -w" -o /out/nextchapter ./cmd/nextchapter

# Final stage: distroless static, runs as nonroot per the ADR-mandated
# security posture for self-hosted deploys.
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/nextchapter /app/nextchapter
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/nextchapter"]
