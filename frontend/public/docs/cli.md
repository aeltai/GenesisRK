# CLI reference (imperative)

The `genesisrk` binary exposes subcommands. Imperative mode means you pass **flags on the command line** instead of (or in addition to) a YAML config file.

```
genesisrk [global flags] <command> [command flags]
```

## Global flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--debug` | `-d` | `false` | Verbose debug logging |
| `--policy` | | | Path to container signature policy file |
| `--insecure-policy` | | `false` | Allow-all signature policy |
| `--hide-log-time` | | `false` | Hide timestamps in logs |

## Subcommands

| Command | Description |
|---------|-------------|
| `generate` | Non-interactive; requires `--config` |
| `tui` | Interactive terminal UI |
| `serve` | HTTP API + optional static web UI |
| `version` | Print version |
| `completion` | Shell completion scripts |

---

## `genesisrk generate`

Config-driven, non-interactive generation. Ideal for CI and scripts.

```bash
genesisrk generate --rancher=v2.13.1 --config=config.example.yaml
```

### Required flags

| Flag | Description |
|------|-------------|
| `--rancher` | Rancher semver (e.g. `v2.13.1`) |
| `--config` / `-c` | YAML config file ([schema](config.md)) |

### Output flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--output` | `-o` | `{rancher}-images.txt` | Linux image list |
| `--output-windows` | | | Windows image list |
| `--output-versions` | | `{rancher}-versions.txt` | Supported k8s versions |
| `--output-source` | | | Image list with source tags |
| `--rke-images` | | | RKE1-only list |
| `--rke2-images` | | | RKE2 Linux list |
| `--rke2-windows-images` | | | RKE2 Windows list |
| `--k3s-images` | | | K3s Linux list |
| `--hauler` | | `false` | Also write Hauler Images manifest |
| `--hauler-output` | | `{rancher}-hauler.yaml` | Hauler manifest path |

### Source overrides

Use when you have local clones or custom KDM (air-gapped prep):

| Flag | Description |
|------|-------------|
| `--kdm` | Local path or URL to KDM `data.json` |
| `--chart` | Cloned chart repo directory (repeatable) |
| `--system-chart` | Cloned system-chart repo directory (repeatable) |
| `--dev` | Use dev branch URLs for charts/KDM |
| `--registry` | Rewrite registry prefix in output |
| `--tls-verify` | Require valid TLS (default `true`) |
| `--kdm-remove-deprecated` | Drop deprecated k3s/rke2 versions (default `true`) |
| `--min-kube-version` | Min RKE1 kube version filter (e.g. `v1.28`) |

### Scan flags

| Flag | Default | Description |
|------|---------|-------------|
| `--scan` | `false` | Run Trivy on final image list |
| `--scan-jobs` | `1` | Parallel scan workers (1–20) |
| `--scan-timeout` | `10m` | Per-image timeout |
| `--scan-report` | auto | CSV report path |

### Other

| Flag | Description |
|------|-------------|
| `--auto-yes` / `-y` | Overwrite output files without prompting |

### Examples

```bash
# Basic generate
genesisrk generate --rancher=v2.13.1 --config=config.example.yaml

# Custom output paths + scan
genesisrk generate \
  --rancher=v2.13.1 \
  --config=config.example.yaml \
  -o lists/rancher-images.txt \
  --output-versions lists/k8s-versions.txt \
  --scan --scan-jobs=4

# With Hauler manifest
genesisrk generate --rancher=v2.13.1 --config=config.example.yaml --hauler

# Offline: local KDM + chart repos
genesisrk generate --rancher=v2.13.1 --config=config.example.yaml \
  --kdm=./kdm-data.json \
  --chart=./charts \
  --system-chart=./system-charts
```

---

## `genesisrk tui`

Interactive terminal UI. Equivalent to `generate` but you select distros, CNI, versions, and chart groups in the browser-like tree.

```bash
genesisrk tui --rancher=v2.13.1
```

### Key flags

| Flag | Description |
|------|-------------|
| `--rancher` | **Required** Rancher version |
| `--save-config` | Write selections to YAML after run (becomes declarative config) |
| `--interactive` / `-i` | Text prompts instead of full TUI (legacy) |

All `generate` output/source/scan flags also apply to `tui`.

### TUI controls

| Key | Action |
|-----|--------|
| ↑ / ↓ | Move cursor |
| Space | Toggle selection |
| Enter / → | Expand/collapse group |
| ← | Collapse |
| q / Ctrl+C | Exit immediately |

### Example: TUI → declarative config

```bash
genesisrk tui --rancher=v2.13.1 --save-config=production.yaml
# Later, in CI:
genesisrk generate --rancher=v2.13.1 --config=production.yaml
```

---

## `genesisrk serve`

Starts the REST API and optionally serves the Vue web UI.

```bash
genesisrk serve --port=8080 --static=frontend/dist
```

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `8080` | Listen port |
| `--static` | | Directory for SPA (e.g. `frontend/dist`) |

API reference: [api.md](api.md)

### Environment variables

| Variable | Description |
|----------|-------------|
| `GITHUB_TOKEN` | Avoid GitHub API rate limits |
| `RANCHER_APPS_API_USER` | Application Collection API user |
| `RANCHER_APPS_API_PASSWORD` | Application Collection API token |

---

## `genesisrk version`

```bash
$ genesisrk version
genesisrk 0.2.0-dev (hangar v1.9.4)
```

---

## Imperative vs declarative

| Concern | Imperative (flags) | Declarative (YAML) |
|---------|-------------------|-------------------|
| Distros, CNI, versions | `--config` still needed for generate | `distros`, `cni`, `versions` in YAML |
| Output paths | `-o`, `--output-versions`, etc. | Not in YAML — use CLI flags |
| Scan | `--scan`, `--scan-jobs` | `scan:` block in YAML |
| Hauler | `--hauler` | `export.hauler: true` |
| Chart/group selection | TUI or YAML `groups`/`charts` | `groups`, `charts` in YAML |

You can combine both: `genesisrk generate --rancher=v2.13.1 --config=base.yaml -o custom.txt --hauler`
