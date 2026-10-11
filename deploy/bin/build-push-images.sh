#!/usr/bin/env bash
# Builds the two images the cluster runs, for linux/amd64 (the VMs), and pushes them to Docker Hub. No image is built on a VM.
# The tag is the commit, so a deployment names exactly the code it ran. The images need nothing outside this repository: the
# CometBFT fork, `bb` and `nargo` are fetched at pinned versions and checked against pinned hashes (go.sum, the Dockerfiles).
#
#   export DOCKERHUB_USER=<your Docker Hub user>      # and `docker login` once on this machine
#   deploy/bin/build-push-images.sh
#   export ENGRAM_IMAGE_TAG=<the tag it prints>        # then run the playbooks
#
# Build on a native amd64 machine (a CI runner, a VM, a Linux workstation). Under emulation (an Apple Silicon Mac) the amd64 `bb`
# and `nargo` crash with SIGILL. PLATFORM=linux/arm64 builds natively on Apple Silicon, for local checks only.
#
# CHECK_ONLY=1   lints both Dockerfiles with BuildKit (no build, no push, no login needed).
# ALLOW_DIRTY=1  builds although tracked files outside deploy/ and docs/ differ from the commit (the tag would not describe them).
# BUILDER=<name> uses that buildx builder (e.g. a remote amd64 builder created with `docker buildx create ssh://user@host`).
# PLATFORM=...   overrides the target platform (default linux/amd64).
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"
PLATFORM="${PLATFORM:-linux/amd64}"

if [ "${CHECK_ONLY:-0}" != "1" ]; then
  : "${DOCKERHUB_USER:?set DOCKERHUB_USER to your Docker Hub user}"
else
  DOCKERHUB_USER="${DOCKERHUB_USER:-check-only}"
fi

TAG="$(git -C "$REPO" rev-parse --short=12 HEAD)"
dirty="$(git -C "$REPO" status --porcelain --untracked-files=no -- . ':!deploy' ':!docs')"
if [ -n "$dirty" ] && [ "${ALLOW_DIRTY:-0}" != "1" ]; then
  echo "tracked files differ from commit $TAG, so the tag would not describe the image:" >&2
  echo "$dirty" >&2
  echo "commit them, or set ALLOW_DIRTY=1" >&2
  exit 3
fi
# The fork commit the images contain: the pseudo-version in go.mod's replace line ends with its 12-character commit.
FORK_TAG="$(sed -nE 's|^replace github.com/cometbft/cometbft => github.com/cuongct924/engram-consensus-core v[^ ]*-([0-9a-f]{12})$|\1|p' "$REPO/go.mod")"

NODE_IMAGE="$DOCKERHUB_USER/engram-node:$TAG"
PROVER_IMAGE="$DOCKERHUB_USER/engram-reanchoring-prover:$TAG"

builder=()
[ -n "${BUILDER:-}" ] && builder=(--builder "$BUILDER")
if [ "${CHECK_ONLY:-0}" = "1" ]; then
  action=(--check)
else
  action=(--push)
fi

echo "[images] commit $TAG, engram-consensus-core ${FORK_TAG:-unknown}, platform $PLATFORM"
docker buildx build ${builder[@]+"${builder[@]}"} --platform "$PLATFORM" -f "$REPO/Dockerfile" -t "$NODE_IMAGE" "${action[@]}" "$REPO"
docker buildx build ${builder[@]+"${builder[@]}"} --platform "$PLATFORM" -f "$REPO/docker/reanchoring-prover/Dockerfile" -t "$PROVER_IMAGE" "${action[@]}" "$REPO"

if [ "${CHECK_ONLY:-0}" = "1" ]; then
  echo "[images] Dockerfiles pass the BuildKit check; nothing was built or pushed"
  exit 0
fi
echo "[images] pushed $NODE_IMAGE and $PROVER_IMAGE"
echo "[images] record for the paper: engram $TAG, engram-consensus-core ${FORK_TAG:-unknown}"
echo "export ENGRAM_IMAGE_TAG=$TAG"
