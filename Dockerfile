# syntax=docker/dockerfile:1
# Multi-stage build for the Engram Sovereign FSM node
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install build dependencies -- apk's own cache mounted too, so this layer
# doesn't re-fetch the package index on every build even after go.mod churn
# invalidates layers below it.
RUN --mount=type=cache,target=/var/cache/apk \
    apk add --no-cache git make

# Copy go mod files first so dependency downloads are cached independently
# of source changes (unchanged from before -- already correct).
COPY go.mod go.sum ./

# go.mod's `replace github.com/cometbft/cometbft => ../engram-consensus-core`
# is a relative filesystem-path replace pointing at a sibling repo (the
# forked CometBFT core, see CLAUDE.md's M0a-M0d) -- it's not a real module
# version `go mod download` can fetch, and by definition lives outside this
# repo's own directory tree, so plain `COPY . .` can never see it. Pulled in
# via a second BuildKit build context (`cometbft-fork`, wired in each
# docker/engram-validator-node0N.yml's `build.additional_contexts`) and
# copied to `/engram-consensus-core` -- exactly where `../engram-consensus-core`
# resolves to from this stage's `/app` WORKDIR, matching the host layout
# (both repos as siblings under the same parent) so the SAME relative
# replace directive works unmodified for both local `go build` and this
# Docker build. Found by actually running `docker compose build`: it failed
# with "reading .../engram-consensus-core/go.mod: no such file or
# directory" using the old absolute-host-path replace, which could never
# have worked in any container regardless of caching.
COPY --from=cometbft-fork . /engram-consensus-core

# GOMODCACHE mounted as a persistent BuildKit cache: modules are fetched
# once and reused across rebuilds (and across all 4 engram-nodeNN image
# builds from this same Dockerfile) instead of re-downloading from GOPROXY
# every time go.mod/go.sum or COPY . . invalidates the layer below.
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source code (.dockerignore already excludes circuit/spec/scripts/
# tests/docs -- everything here is Go-build-relevant).
COPY . .

# Both the module cache and Go's build cache (/root/.cache/go-build) are
# mounted -- without the build-cache mount, every source-changing rebuild
# recompiles the entire dependency graph from scratch even though the
# module cache mount above avoids re-downloading it.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -o engramd ./cmd/engramd

# Runtime stage
FROM alpine:latest

WORKDIR /root

# Install runtime dependencies
RUN --mount=type=cache,target=/var/cache/apk \
    apk add --no-cache ca-certificates bash curl jq

# Copy binary from builder
COPY --from=builder /app/engramd /usr/local/bin/engramd

# Create node home directory (matches cmd/engramd's defaultHome())
RUN mkdir -p .engramd

# Expose ports
EXPOSE 26656 26657 26660 1317

ENTRYPOINT ["engramd"]
