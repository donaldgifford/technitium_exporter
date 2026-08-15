# syntax=docker/dockerfile:1.7
# Multi-stage build: small distroless image, cached module + build layers.

FROM golang:1.26.6 AS build
WORKDIR /src
COPY go.* ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
# Defaults mirror the zero-values in cmd/technitium_exporter/main.go, so an
# un-parameterised build reports the same thing the binary would on its own.
# Note the single "$" below: RUN is not on Dockerfile's variable-substitution
# list, so the string reaches /bin/sh intact and the shell expands the ARGs.
# Writing "$${VERSION}" makes the shell read "$$" as its own PID.
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
RUN --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
  CGO_ENABLED=0 go build \
  -trimpath \
  -ldflags="-s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildDate=${DATE}" \
  -o /out/technitium_exporter ./cmd/technitium_exporter

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/technitium_exporter /usr/local/bin/technitium_exporter
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/technitium_exporter"]
