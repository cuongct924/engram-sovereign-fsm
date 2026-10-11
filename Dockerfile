# syntax=docker/dockerfile:1
# Multi-stage build for the Engram Sovereign FSM node
FROM golang:1.26-alpine@sha256:3082400e369fa24d5fc60bca20edab3f6d604e0c5a690ec66b295eff4dd87ade AS builder

WORKDIR /app

# Build deps. apk's cache is mounted so the package index isn't re-fetched
# every rebuild even when go.mod churn invalidates lower layers.
RUN --mount=type=cache,target=/var/cache/apk \
    apk add --no-cache git make

# Copy go mod files first so dependency downloads are cached independently
# of source changes.
COPY go.mod go.sum ./

# go.mod replaces github.com/cometbft/cometbft with the public engram-consensus-core fork,
# pinned to a commit, so `go mod download` below fetches it like any other module.

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
FROM debian:bookworm-slim@sha256:7c7b2c966bc9ee8cedfeef67e0e279108992c77681fa595db4a9d65c06ccc587

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
# The tarball is checked against a pinned SHA-256, so a changed or re-published release fails the build.
# `bb --version` is not run here: the amd64 binary needs CPU instructions that emulation lacks (SIGILL), and
# the image is meant to be built on a native machine of its target architecture anyway.
ARG BB_VERSION=5.0.0-nightly.20260522
ARG BB_SHA256_AMD64=d207ec90fbfa2fba24d7a47b7a75892ee052b7984252b866a4a0c1b5296e1571
ARG BB_SHA256_ARM64=b6842ea75c171fd84b5e8fa0af75f61121d0c3dee1c52e17b7f8d7619a4189fe
RUN ARCH=$(uname -m) && \
    case "$ARCH" in \
      aarch64) BB_ARCH=arm64; BB_SHA256="$BB_SHA256_ARM64" ;; \
      x86_64) BB_ARCH=amd64; BB_SHA256="$BB_SHA256_AMD64" ;; \
      *) echo "unsupported arch: $ARCH" >&2; exit 1 ;; \
    esac && \
    curl -sL --fail "https://github.com/AztecProtocol/barretenberg/releases/download/v${BB_VERSION}/barretenberg-${BB_ARCH}-linux.tar.gz" -o /tmp/bb.tar.gz && \
    echo "${BB_SHA256}  /tmp/bb.tar.gz" | sha256sum -c - && \
    tar xzf /tmp/bb.tar.gz -C /usr/local/bin && \
    rm /tmp/bb.tar.gz && \
    test -x /usr/local/bin/bb

# Copy binary from builder
COPY --from=builder /app/engramd /usr/local/bin/engramd

# Create node home directory (matches cmd/engramd's defaultHome())
RUN mkdir -p .engramd

# Expose ports
EXPOSE 26656 26657 26660 1317

ENTRYPOINT ["engramd"]
