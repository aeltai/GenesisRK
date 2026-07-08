# Configuration reference

GenesisRK uses a single YAML config for `genesisrk generate`. The TUI can save selections to the same format with `--save-config`.

## Example

See [config.example.yaml](../config.example.yaml) in the repo root.

## Fields

| Field | Type | Description |
|-------|------|-------------|
| `distros` | list | `k3s`, `rke2`, `rke` |
| `sourceType` | string | `community` (default) or `prime-gc` |
| `includeAppCollectionCharts` | bool | Fetch Application Collection charts/images |
| `cni` | string | `cni_canal`, `cni_calico`, `cni_flannel`, `cni_cilium` |
| `loadBalancer` | bool | Include LB/ingress images (default true) |
| `includeWindows` | bool | Include Windows node images |
| `versions` | map | Per-distro Kubernetes versions |
| `groups` | list | `basic`, `addons`, `addon_*`, `app_collection` |
| `charts` | list | Specific chart names (overrides groups) |
| `export.hauler` | bool | Write Hauler Images manifest (default false) |
| `export.haulerOutput` | string | Hauler manifest path |
| `scan.enabled` | bool | Run Trivy scan after generate |
| `scan.jobs` | int | Scan parallelism |
| `scan.timeout` | duration | Per-image timeout |
| `scan.report` | string | CSV report path |

## CLI equivalents

```bash
genesisrk generate --rancher=v2.13.1 --config=config.yaml
genesisrk generate --rancher=v2.13.1 --config=config.yaml --hauler
genesisrk tui --rancher=v2.13.1 --save-config=my-config.yaml
```
