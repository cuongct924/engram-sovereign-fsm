# syntax=docker/dockerfile:1
# ZK re-anchoring prover -- containerized scripts/reanchoring_prover/watch_and_prove.sh.
# Separate from the root Dockerfile (which excludes circuit/scripts/) --
# this image needs both, plus nargo, and uses its own sibling
# Dockerfile.dockerignore instead of the root one.
FROM golang:1.26-alpine@sha256:3082400e369fa24d5fc60bca20edab3f6d604e0c5a690ec66b295eff4dd87ade AS builder

WORKDIR /app

RUN --mount=type=cache,target=/var/cache/apk \
    apk add --no-cache git make

COPY go.mod go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -o engramd ./cmd/engramd && \
    CGO_ENABLED=0 GOOS=linux go build -o engramprover ./cmd/engramprover

# Runtime stage -- glibc (debian), same reasoning as the root Dockerfile:
# bb is a real glibc binary, Alpine's musl+gcompat is missing symbols it needs.
FROM debian:bookworm-slim@sha256:7c7b2c966bc9ee8cedfeef67e0e279108992c77681fa595db4a9d65c06ccc587

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates bash curl git xz-utils python3 openssh-client && \
    rm -rf /var/lib/apt/lists/*

# Pin the SAME bb version the root Dockerfile and this repo's off-chain
# proving tooling use -- proofs must be wire-compatible with what every
# validator's VerifyZKProof checks them against.
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

# Pin the SAME nargo version this repo's circuits are written against (docs/DEVELOPMENT.md, circuit/*/Nargo.toml).
# Downloaded straight from the Noir release and checked against a pinned SHA-256; the noirup installer would fetch an
# unpinned "latest" helper.
ARG NARGO_VERSION=1.0.0-beta.22
ARG NARGO_SHA256_AMD64=384c4fc800905b213e26aabd738a96a4a85b1a76ffc27fb19aeb6d33494a787b
ARG NARGO_SHA256_ARM64=9e07b436148bb1e89d12a4d2043e613ea5fd81f7d5518e8104e27298b39c314a
RUN ARCH=$(uname -m) && \
    case "$ARCH" in \
      aarch64) NARGO_TARGET=aarch64-unknown-linux-gnu; NARGO_SHA256="$NARGO_SHA256_ARM64" ;; \
      x86_64) NARGO_TARGET=x86_64-unknown-linux-gnu; NARGO_SHA256="$NARGO_SHA256_AMD64" ;; \
      *) echo "unsupported arch: $ARCH" >&2; exit 1 ;; \
    esac && \
    curl -sL --fail "https://github.com/noir-lang/noir/releases/download/v${NARGO_VERSION}/nargo-${NARGO_TARGET}.tar.gz" -o /tmp/nargo.tar.gz && \
    echo "${NARGO_SHA256}  /tmp/nargo.tar.gz" | sha256sum -c - && \
    tar xzf /tmp/nargo.tar.gz -C /usr/local/bin && \
    rm /tmp/nargo.tar.gz && \
    test -x /usr/local/bin/nargo

COPY --from=builder /app/engramd /usr/local/bin/engramd
COPY --from=builder /app/engramprover /usr/local/bin/engramprover
COPY circuit ./circuit

# Compile at BUILD time (not per-proof-cycle at runtime -- confirmed live
# to OOM-kill even a 4096M container, since runtime deploy.resources.limits
# don't constrain `docker build`). Before scripts/zk_assets so a change to
# either doesn't invalidate this layer's cache.
RUN cd circuit/reanchoring_witness && nargo compile
RUN cd circuit/reanchoring && nargo compile
# Chunk-aggregation mode (ENGRAM_AGGREGATE=1, prove_aggregate_and_submit.sh).
RUN cd circuit/reanchoring_aggregator && nargo compile

COPY scripts/reanchoring_prover ./scripts/reanchoring_prover
COPY x/sovereignty/keeper/zk_assets ./x/sovereignty/keeper/zk_assets

ENV ENGRAMD_BIN=/usr/local/bin/engramd
ENV MAX_CHECKS=999999999

ENTRYPOINT ["/bin/bash", "/app/scripts/reanchoring_prover/watch_and_prove.sh"]
