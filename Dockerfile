# syntax=docker/dockerfile:1
# Multi-stage build for the Engram Sovereign FSM node
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Build deps. apk's cache is mounted so the package index isn't re-fetched
# every rebuild even when go.mod churn invalidates lower layers.
RUN --mount=type=cache,target=/var/cache/apk \
    apk add --no-cache git make

# Copy go mod files first so dependency downloads are cached independently
# of source changes.
COPY go.mod go.sum ./

# go.mod's `replace github.com/cometbft/cometbft => ../engram-consensus-core`
# is a relative path to a sibling repo (the forked CometBFT core, see
# CLAUDE.md) that `COPY . .` can never see. Pulled in via a second BuildKit
# context (`cometbft-fork`, wired in each docker/engram-validator-node0N.yml's
# build.additional_contexts) to /engram-consensus-core, exactly where
# `../engram-consensus-core` resolves from /app -- so the same replace works
# for both local `go build` and this Docker build.
COPY --from=cometbft-fork . /engram-consensus-core

# GOMODCACHE mounted as a persistent BuildKit cache: modules are fetched
# once and reused across rebuilds (and across all 4 node images) instead of
# re-downloading every time go.mod/go.sum or COPY . . invalidates the layer.
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source (.dockerignore excludes circuit/spec/scripts/tests/docs --
# everything Go-build-relevant).
COPY . .

# Module and build caches both mounted: without the build-cache mount, every
# source-changing rebuild recompiles the whole dependency graph from scratch.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -o engramd ./cmd/engramd

# Runtime stage
#
# glibc-based (debian:bookworm-slim), NOT alpine -- reanchor.go's
# VerifyZKProof shells out to a real `bb` (Barretenberg) binary during
# DeliverTx, and every validator running the SAME bb build is consensus-
# safety-load-bearing (see that file's doc). The official bb release is
# glibc-linked; Alpine's musl + gcompat shim is missing symbols it actually
# needs (e.g. mallinfo2, __res_init), so `bb --version` fails under
# alpine+gcompat.
FROM debian:bookworm-slim

WORKDIR /root

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates bash curl jq && \
    rm -rf /var/lib/apt/lists/*

# Pin the same bb version this repo's off-chain proving tooling uses
# (scripts/reanchoring_prover/, docs/EXPERIMENT.md's E6) -- proofs from one
# bb version aren't guaranteed wire-compatible with another's verifier.
# uname -m reports aarch64/x86_64, not Barretenberg's arm64/amd64 naming,
# hence the translation.
ARG BB_VERSION=5.0.0-nightly.20260522
RUN ARCH=$(uname -m) && \
    case "$ARCH" in \
      aarch64) BB_ARCH=arm64 ;; \
      x86_64) BB_ARCH=amd64 ;; \
      *) echo "unsupported arch: $ARCH" >&2; exit 1 ;; \
    esac && \
    curl -sL --fail "https://github.com/AztecProtocol/barretenberg/releases/download/v${BB_VERSION}/barretenberg-${BB_ARCH}-linux.tar.gz" -o /tmp/bb.tar.gz && \
    tar xzf /tmp/bb.tar.gz -C /usr/local/bin && \
    rm /tmp/bb.tar.gz && \
    /usr/local/bin/bb --version

# Copy binary from builder
COPY --from=builder /app/engramd /usr/local/bin/engramd

# Create node home directory (matches cmd/engramd's defaultHome())
RUN mkdir -p .engramd

# Expose ports
EXPOSE 26656 26657 26660 1317

ENTRYPOINT ["engramd"]
