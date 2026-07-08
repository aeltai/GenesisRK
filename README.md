<div align="center">
  <h1>GenesisRK</h1>
  <p>Generate Rancher image lists for air-gapped deployments</p>
  <p>
    <a href="https://github.com/aeltai/Hangar-Genesis">GitHub</a> ·
    <a href="https://genesis-app.bravecoast-8b272aef.westeurope.azurecontainerapps.io/">Live demo</a> ·
    <a href="docs/README.md">Documentation</a>
  </p>
</div>

GenesisRK builds comprehensive container image lists and Kubernetes version manifests for Rancher offline deployments. It depends on [Hangar](https://github.com/cnrancher/hangar) — use **GenesisRK** to plan what to mirror, then **Hangar** to mirror, save, and load into your private registry.

## Four ways to run

| Mode | Command / interface | Style |
|------|---------------------|-------|
| **Declarative** | `genesisrk generate --config=…` | YAML config |
| **Imperative** | CLI flags (`-o`, `--scan`, `--hauler`, …) | Command-line flags |
| **Interactive** | `genesisrk tui --rancher=…` | Terminal UI |
| **API / Web** | `genesisrk serve` → http://localhost:8080 | REST + browser |

Full reference: [docs/README.md](docs/README.md)

## Terminal demos

![genesisrk version and help](docs/assets/version.gif)

<details>
<summary>More demos</summary>

![Generate from YAML config](docs/assets/generate.gif)

![Interactive TUI](docs/assets/tui.gif)

![Serve API and web UI](docs/assets/serve.gif)

Regenerate with [VHS](https://github.com/charmbracelet/vhs): `vhs docs/vhs/<name>.tape`

</details>

## Quick start

```bash
# Build (Hangar v1.9.4 required as ../hangar)
git clone https://github.com/aeltai/Hangar-Genesis.git && cd Hangar-Genesis
git clone https://github.com/cnrancher/hangar.git ../hangar && cd ../hangar && git checkout v1.9.4 && cd ../Hangar-Genesis
go build -o bin/genesisrk .

# Declarative — CI-friendly
genesisrk generate --rancher=v2.13.1 --config=config.example.yaml

# Interactive TUI
genesisrk tui --rancher=v2.13.1

# Web UI + REST API
genesisrk serve --port=8080 --static=frontend/dist
```

## Air-gapped workflow

```bash
genesisrk generate --rancher=v2.13.1 --config=config.example.yaml
hangar mirror -f v2.13.1-images.txt -d registry.example.com --copy-all-platforms
```

Optional Hauler manifest: `--hauler` or `export.hauler: true` in config. See [docs/outputs.md](docs/outputs.md).

## Documentation

| Guide | Description |
|-------|-------------|
| [Getting started](docs/getting-started.md) | Install, first run |
| [CLI (imperative)](docs/cli.md) | Every flag and subcommand |
| [Config (declarative YAML)](docs/config.md) | Full schema |
| [REST API](docs/api.md) | Every endpoint + JSON types |
| [Web UI](docs/web-ui.md) | Browser workflow |
| [Outputs](docs/outputs.md) | images.txt, Hauler, scan |
| [Air-gap testing](docs/airgap-testing.md) | AWS e2e script |
| [Deploy](docs/deploy.md) | Docker, Azure |

## License

Apache 2.0 — see [LICENSE](LICENSE). Built on [Hangar](https://github.com/cnrancher/hangar).
