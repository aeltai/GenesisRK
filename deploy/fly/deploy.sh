#!/usr/bin/env bash
# Deploy GenesisRK to Fly.io.
#
# One-time setup (already done for this repo):
#   flyctl apps create genesisrk --org personal
#   flyctl secrets set GITHUB_TOKEN=ghp_xxx -a genesisrk
#
# Usage:
#   bash deploy/fly/deploy.sh           # deploy (builds on Fly remote builder)
#   bash deploy/fly/deploy.sh --local   # build locally with docker then deploy
#   bash deploy/fly/deploy.sh --open     # deploy and open the app URL

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$REPO_ROOT"

APP_NAME="${APP_NAME:-genesisrk}"
FLY_TOML="$SCRIPT_DIR/fly.toml"

if ! command -v flyctl >/dev/null 2>&1; then
  echo "flyctl not found. Install: brew install flyctl"
  exit 1
fi

echo "[$(date '+%H:%M:%S')] Deploying $APP_NAME to Fly.io (toml: $FLY_TOML)..."

LOCAL_BUILD=0
OPEN=0
for arg in "$@"; do
  case "$arg" in
    --local) LOCAL_BUILD=1 ;;
    --open)  OPEN=1 ;;
  esac
done

if [[ "$LOCAL_BUILD" == "1" ]]; then
  # Build locally (amd64) and push to the Fly registry, then deploy.
  flyctl deploy --config "$FLY_TOML" --app "$APP_NAME" --dockerfile deploy/azure/Dockerfile.genesis --local-only
else
  flyctl deploy --config "$FLY_TOML" --app "$APP_NAME"
fi

echo "[$(date '+%H:%M:%S')] Deployed. URL: https://$APP_NAME.fly.dev"
flyctl status --app "$APP_NAME" || true

if [[ "$OPEN" == "1" ]]; then
  flyctl apps open --app "$APP_NAME" || true
fi
