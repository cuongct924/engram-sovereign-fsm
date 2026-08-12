# syntax=docker/dockerfile:1
# ZK re-anchoring prover -- containerized scripts/reanchoring_prover/watch_and_prove.sh.
# Separate from the root Dockerfile (which excludes circuit/scripts/ from
# its context) since this image needs both, plus nargo. Uses its own
# sibling Dockerfile.dockerignore (BuildKit's per-Dockerfile convention),
# not the root one.
FROM golang:1.26-alpine AS builder

WORKDIR /app

RUN --mount=type=cache,target=/var/cache/apk \
    apk add --no-cache git make

COPY go.mod go.sum ./
# Same sibling-repo replace as the root Dockerfile -- see its own comment
# for why this needs a second build context, not a plain COPY.
COPY --from=cometbft-fork . /engram-consensus-core

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -o engramd ./cmd/engramd

# Runtime stage -- glibc (debian), same reasoning as the root Dockerfile:
# bb is a real glibc binary, Alpine's musl+gcompat is missing symbols it needs.
FROM debian:bookworm-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates bash curl git xz-utils && \
    rm -rf /var/lib/apt/lists/*

# Pin the SAME bb version the root Dockerfile and this repo's off-chain
# proving tooling use -- proofs must be wire-compatible with what every
# validator's VerifyZKProof checks them against.
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

# Pin the SAME nargo version this repo's circuits are written against
# (docs/DEVELOPMENT.md, circuit/*/Nargo.toml) via noirup, Noir's own
# version-manager installer.
ARG NARGO_VERSION=1.0.0-beta.22
ENV PATH="/root/.nargo/bin:${PATH}"
RUN curl -sL --fail https://raw.githubusercontent.com/noir-lang/noirup/main/install -o /tmp/noirup-install && \
    bash /tmp/noirup-install && \
    rm /tmp/noirup-install && \
    /root/.nargo/bin/noirup --version "$NARGO_VERSION" && \
    nargo --version

COPY --from=builder /app/engramd /usr/local/bin/engramd
COPY circuit ./circuit
COPY scripts/reanchoring_prover ./scripts/reanchoring_prover
COPY x/sovereignty/keeper/zk_assets ./x/sovereignty/keeper/zk_assets

ENV ENGRAMD_BIN=/usr/local/bin/engramd
ENV MAX_CHECKS=999999999

ENTRYPOINT ["/bin/bash", "/app/scripts/reanchoring_prover/watch_and_prove.sh"]
