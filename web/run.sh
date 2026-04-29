#!/usr/bin/env bash
# Wrapper for running TypeScript / JavaScript / npm commands inside the
# HydraKVM web builder container.
#
# Usage (from anywhere in the repo):
#
#   web/run.sh npm install
#   web/run.sh npm run check
#   web/run.sh npm run build
#   web/run.sh npm run watch
#   web/run.sh             # interactive shell (when stdin is a TTY)
#
# The first invocation builds the container image; later invocations reuse
# it. Force a rebuild with: `podman build -t hydrakvm-web-builder web/`.

set -euo pipefail

dir="$(cd "$(dirname "$0")" && pwd)"
repo_root="$(cd "$dir/.." && pwd)"
image="hydrakvm-web-builder"

if ! podman image exists "$image"; then
  podman build -t "$image" "$dir"
fi

# Mount the web/ source as /work (so npm sees package.json / node_modules
# in the expected location) and the server's embed target so the build can
# emit dist/ artifacts directly to the embed path.
flags=(
  --rm
  -v "$dir":/work
  -v "$repo_root/server/internal/http/web/dist":/server/internal/http/web/dist
)
[ -t 0 ] && flags+=(-i)
[ -t 1 ] && flags+=(-t)

exec podman run "${flags[@]}" "$image" "$@"
