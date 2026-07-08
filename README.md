<div align="center">
  <h1>GenesisRK</h1>
  <p>Generate Rancher image lists for air-gapped deployments</p>
  <p>
    <a href="https://github.com/aeltai/Hangar-Genesis">GitHub</a> ·
    <a href="https://genesis-app.bravecoast-8b272aef.westeurope.azurecontainerapps.io/">Live demo</a> ·
    <a href="docs/config.md">Config</a> ·
    <a href="docs/api.md">API</a>
  </p>
</div>

GenesisRK builds comprehensive container image lists and Kubernetes version manifests for Rancher offline deployments. It sits on top of [Hangar](https://github.com/cnrancher/hangar) — use GenesisRK to generate lists, then Hangar to mirror, save, and load images in your air-gapped registry.

## Install

```bash
# From source
go install github.com/aeltai/hangar-genesis@latest

# Or download a release binary (see Releases)
```

Requires Go 1.24+ and a checkout of [Hangar](https://github.com/cnrancher/hangar) at `v1.9.4` as a sibling directory (`../hangar`), or adjust the `replace` directive in `go.mod`.

```bash
git clone https://github.com/cnrancher/hangar.git ../hangar
cd ../hangar && git checkout v1.9.4
cd ../Hangar-Genesis
go build -o bin/genesisrk .
```

## Quick start

```bash
# Interactive TUI
genesisrk tui --rancher=v2.13.1

# Config-driven (CI / scripts)
genesisrk generate --rancher=v2.13.1 --config=config.example.yaml

# Web UI + API
genesisrk serve --port=8080 --static=frontend/dist
```

## Terminal demos

Record or replay with [VHS](https://github.com/charmbracelet/vhs):

```bash
vhs docs/vhs/tui.tape      # -> docs/vhs/tui.gif
vhs docs/vhs/generate.tape # -> docs/vhs/generate.gif
vhs docs/vhs/serve.tape    # -> docs/vhs/serve.gif
```

| Mode | Command |
|------|---------|
| TUI | `genesisrk tui --rancher=v2.13.1` |
| Generate | `genesisrk generate --rancher=v2.13.1 --config=config.example.yaml` |
| Serve | `genesisrk serve --port=8080 --static=frontend/dist` |

## Optional Hauler export

Hauler manifests are off by default. Enable via flag, config, or UI:

```bash
genesisrk generate --rancher=v2.13.1 --config=config.example.yaml --hauler
```

```yaml
# config.yaml
export:
  hauler: true
```

## Air-gapped workflow

```bash
genesisrk generate --rancher=v2.13.1 --config=config.example.yaml
hangar mirror -f v2.13.1-images.txt -d registry.example.com --copy-all-platforms
```

See [docs/airgap-testing.md](docs/airgap-testing.md) for an AWS e2e script.

## Subcommands

| Command | Description |
|---------|-------------|
| `genesisrk generate` | Non-interactive, YAML config |
| `genesisrk tui` | Interactive terminal UI |
| `genesisrk serve` | REST API + web UI |
| `genesisrk version` | Print version |

## Deploy

Docker and Azure Container Apps: [docs/deploy.md](docs/deploy.md)

## License

Apache 2.0 — see [LICENSE](LICENSE). Built on [Hangar](https://github.com/cnrancher/hangar) by SUSE Rancher.
