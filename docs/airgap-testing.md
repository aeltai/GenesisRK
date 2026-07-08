# Air-gapped testing with GenesisRK

GenesisRK generates image lists; [Hangar](https://github.com/cnrancher/hangar) mirrors, saves, and loads them in air-gapped environments.

## Prerequisites

- `genesisrk` and `hangar` installed on a bastion or air-gapped host
- Private container registry reachable from the air-gapped cluster
- SSH access to the test host (for the bundled script)

## Quick run

```bash
export REGISTRY=registry.63.176.138.74.sslip.io
export SSH_HOST=your-airgap-host.example.com
export RANCHER_VERSION=v2.13.1

chmod +x test/e2e/airgap.sh
./test/e2e/airgap.sh
```

## What the script does

1. Generates `images.txt` with `genesisrk generate --config=...`
2. Copies the list to the air-gapped host
3. Runs `hangar save` on a sample of images
4. Runs `hangar load` into your private registry
5. Verifies manifests exist via registry HTTP API

## Manual workflow

```bash
# 1. Generate lists
genesisrk generate --rancher=v2.13.1 --config=config.example.yaml

# 2. Mirror or save/load with hangar
hangar mirror -f v2.13.1-images.txt -d $REGISTRY --copy-all-platforms
# or
hangar save -f v2.13.1-images.txt -d ./bundle
hangar load -d ./bundle -r $REGISTRY
```

## Optional Hauler manifest

```bash
genesisrk generate --rancher=v2.13.1 --config=config.example.yaml --hauler
# produces v2.13.1-hauler.yaml for rancher/hauler store workflows
```
