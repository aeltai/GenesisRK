#!/usr/bin/env bash
# Air-gapped end-to-end test for GenesisRK + Hangar.
# Runs on AWS air-gapped infra; requires SSH access to a host with hangar and genesisrk installed.
#
# Required env:
#   REGISTRY       Private registry host (e.g. registry.63.176.138.74.sslip.io)
#   SSH_HOST       SSH target for the air-gapped host
#   RANCHER_VERSION Rancher version to test (default: v2.13.1)
#
# Optional:
#   CONFIG         Path to genesis config YAML (default: config.example.yaml)
#   SAMPLE_COUNT   Number of images to verify in registry (default: 5)
#   SSH_USER       SSH user (default: ubuntu)

set -euo pipefail

RANCHER_VERSION="${RANCHER_VERSION:-v2.13.1}"
CONFIG="${CONFIG:-config.example.yaml}"
SAMPLE_COUNT="${SAMPLE_COUNT:-5}"
SSH_USER="${SSH_USER:-ubuntu}"
WORKDIR="${WORKDIR:-/tmp/genesisrk-e2e-$$}"

if [[ -z "${REGISTRY:-}" || -z "${SSH_HOST:-}" ]]; then
  echo "REGISTRY and SSH_HOST are required."
  echo "Example:"
  echo "  REGISTRY=registry.example.com SSH_HOST=airgap-host.example.com $0"
  exit 1
fi

echo "==> Generating image list locally with genesisrk"
mkdir -p "$WORKDIR"
genesisrk generate --rancher="$RANCHER_VERSION" --config="$CONFIG" -o "$WORKDIR/images.txt" || {
  echo "Note: if config needs network for KDM, run generate on the air-gapped host instead."
}

echo "==> Copying artifacts to air-gapped host"
scp "$WORKDIR/images.txt" "${SSH_USER}@${SSH_HOST}:${WORKDIR}/images.txt" 2>/dev/null || \
  ssh "${SSH_USER}@${SSH_HOST}" "mkdir -p $WORKDIR && cat > $WORKDIR/images.txt" < "$WORKDIR/images.txt"

echo "==> Saving images with hangar (on air-gapped host or via SSH)"
ssh "${SSH_USER}@${SSH_HOST}" bash -s <<EOF
set -euo pipefail
WORKDIR="$WORKDIR"
REGISTRY="$REGISTRY"
SAMPLE_COUNT="$SAMPLE_COUNT"
mkdir -p "\$WORKDIR/bundle"
head -n 20 "\$WORKDIR/images.txt" > "\$WORKDIR/sample-images.txt"
hangar save -f "\$WORKDIR/sample-images.txt" -d "\$WORKDIR/bundle" --jobs=2
echo "==> Loading bundle into private registry"
hangar load -d "\$WORKDIR/bundle" -r "\$REGISTRY" --jobs=2
echo "==> Verifying sample manifests in registry"
while IFS= read -r img; do
  [[ -z "\$img" || "\$img" =~ ^# ]] && continue
  repo_tag="\${img#*/}"
  repo="\${repo_tag%%:*}"
  tag="\${repo_tag##*:}"
  code=\$(curl -s -o /dev/null -w '%{http_code}' "https://\${REGISTRY}/v2/\${repo}/manifests/\${tag}" || echo "000")
  echo "  \$img -> HTTP \$code"
  [[ "\$code" == "200" || "\$code" == "401" ]] || exit 1
done < <(head -n "\$SAMPLE_COUNT" "\$WORKDIR/sample-images.txt")
echo "E2E air-gap test passed."
EOF

echo "Done."
