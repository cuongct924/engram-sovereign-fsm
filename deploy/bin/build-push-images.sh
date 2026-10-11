#!/usr/bin/env bash
# Builds the two images the cluster runs, for linux/amd64 (the VMs; this machine may be arm64), and pushes them to Docker Hub.
# No image is built on a VM. The tag is the commit, so a deployment names exactly the code it ran.
#
#   export DOCKERHUB_USER=<your Docker Hub user>      # and `docker login` once on this machine
#   deploy/bin/build-push-images.sh
#   export ENGRAM_IMAGE_TAG=<the tag it prints>        # then run the playbooks
#
# CHECK_ONLY=1 lints both Dockerfiles with BuildKit (no build, no push, no login needed).
# ALLOW_DIRTY=1 builds although tracked files outside deploy/ and docs/ differ from the commit (the tag would not describe them).
# FORK_DIR overrides the engram-consensus-core checkout (default: next to this repository, go.mod's replace target).
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"
FORK_DIR="${FORK_DIR:-$REPO/../engram-consensus-core}"
PLATFORM=linux/amd64

[ -d "$FORK_DIR" ] || { echo "engram-consensus-core not found at $FORK_DIR (go.mod's replace target); set FORK_DIR" >&2; exit 2; }
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
FORK_TAG="$(git -C "$FORK_DIR" rev-parse --short=12 HEAD 2>/dev/null || echo unknown)"

NODE_IMAGE="$DOCKERHUB_USER/engram-node:$TAG"
PROVER_IMAGE="$DOCKERHUB_USER/engram-reanchoring-prover:$TAG"

if [ "${CHECK_ONLY:-0}" = "1" ]; then
  action=(--check)
else
  action=(--push)
fi

echo "[images] commit $TAG, engram-consensus-core $FORK_TAG, platform $PLATFORM"
docker buildx build --platform "$PLATFORM" --build-context cometbft-fork="$FORK_DIR" \
  -f "$REPO/Dockerfile" -t "$NODE_IMAGE" "${action[@]}" "$REPO"
docker buildx build --platform "$PLATFORM" --build-context cometbft-fork="$FORK_DIR" \
  -f "$REPO/docker/reanchoring-prover/Dockerfile" -t "$PROVER_IMAGE" "${action[@]}" "$REPO"

if [ "${CHECK_ONLY:-0}" = "1" ]; then
  echo "[images] Dockerfiles pass the BuildKit check; nothing was built or pushed"
  exit 0
fi
echo "[images] pushed $NODE_IMAGE and $PROVER_IMAGE"
echo "[images] record for the paper: engram $TAG, engram-consensus-core $FORK_TAG"
echo "export ENGRAM_IMAGE_TAG=$TAG"
