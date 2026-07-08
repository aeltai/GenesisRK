# Config reference (declarative YAML)

Declarative mode means **all selections live in a YAML file**. The CLI only needs `--rancher` and `--config`:

```bash
genesisrk generate --rancher=v2.13.1 --config=production.yaml
```

The TUI can produce this file with `--save-config`. The REST API `POST /api/generate` accepts the same fields as JSON (imperative-over-HTTP).

Full example: [config.example.yaml](../config.example.yaml)

---

## Schema

```yaml
# ── Step 1: Infrastructure ──────────────────────────────────────────

distros:                    # required for meaningful output
  - k3s
  - rke2
  # - rke                     # RKE1

sourceType: community       # community (default) | prime-gc

cni: cni_calico             # cni_canal | cni_calico | cni_flannel | cni_cilium
                            # flannel only valid for k3s

loadBalancer: true          # include K3s Klipper/Traefik, RKE2 NGINX/Traefik
includeWindows: false       # Linux-only (default) vs Linux+Windows node images

includeAppCollectionCharts: false   # live-fetch from api.apps.rancher.io
                                    # requires RANCHER_APPS_API_USER/PASSWORD env

versions:                   # per-distro k8s versions; omit "all" in list form
  k3s:
    - v1.32.11+k3s3
  rke2:
    - v1.34.3+rke2r1
  rke:
    - v1.28.15

# ── Step 2: Charts & components ─────────────────────────────────────

groups:
  - basic                   # Rancher core + selected distro + CNI + Fleet
  - addon_monitoring
  - addon_logging
  # - addons                # all addon charts
  # - addon_storage
  # - addon_security
  # - addon_backup-restore
  # - app_collection        # when includeAppCollectionCharts: true

charts:                     # optional: specific charts (overrides/pin groups)
  - rancher-monitoring
  - rancher-logging

# ── Export options ────────────────────────────────────────────────────

export:
  hauler: false             # write Hauler Images manifest YAML
  haulerOutput: v2.13.1-hauler.yaml   # optional path

# ── Vulnerability scan ───────────────────────────────────────────────

scan:
  enabled: false
  jobs: 1
  timeout: 10m
  report: ""                  # default: {output-base}-scan-report.csv
```

---

## Field reference

### `distros`

List of cluster types to include in the **Basic** preset.

| Value | Description |
|-------|-------------|
| `k3s` | K3s node images |
| `rke2` | RKE2 node images |
| `rke` | RKE1 (legacy) node images |

### `sourceType`

| Value | Charts source | KDM source |
|-------|---------------|------------|
| `community` (default) | GitHub `rancher/charts` | `releases.rancher.com` |
| `prime-gc` | `cnrancher/pandaria-catalog` | `charts.rancher.cn` |

### `cni`

| Value | Distros |
|-------|---------|
| `cni_canal` | RKE2, RKE1 |
| `cni_calico` | K3s, RKE2, RKE1 |
| `cni_flannel` | K3s only |
| `cni_cilium` | K3s, RKE2 |

### `loadBalancer`

When `true` (default), includes ingress/LB images:

- K3s: Klipper LB, Traefik
- RKE2: NGINX Ingress, Traefik

Set `false` to exclude all LB/ingress images from Basic.

### `includeWindows`

When `true`, includes Windows node images for K3s/RKE2. Default `false` (Linux only).

### `includeAppCollectionCharts`

When `true`, fetches charts and container images from Application Collection (`dp.apps.rancher.io`).

Requires environment variables:

```bash
export RANCHER_APPS_API_USER=your-user
export RANCHER_APPS_API_PASSWORD=your-token
```

### `versions`

Map of distro → list of exact version strings from KDM. Use specific versions for reproducible lists; the generator validates against KDM capabilities.

### `groups`

Select chart/image groups in Step 2.

| Group | Contents |
|-------|----------|
| `basic` | Rancher server, Fleet, selected distro, CNI |
| `addons` | All addon charts |
| `addon_monitoring` | rancher-monitoring, etc. |
| `addon_logging` | rancher-logging, etc. |
| `addon_storage` | Storage-related charts |
| `addon_security` | Security charts |
| `addon_backup-restore` | Backup/restore charts |
| `app_collection` | Application Collection (requires `includeAppCollectionCharts`) |

### `charts`

Explicit chart names. When set, pins selection to these charts in addition to (or instead of) broad `groups`.

### `export`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `hauler` | bool | `false` | Write Hauler-compatible Images manifest |
| `haulerOutput` | string | `{rancher}-hauler.yaml` | Output path |

CLI equivalent: `--hauler` / `--hauler-output`

### `scan`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Run Trivy after generation |
| `jobs` | int | `1` | Parallel workers |
| `timeout` | duration | `10m` | Per-image timeout |
| `report` | string | auto | CSV report path |

CLI equivalent: `--scan`, `--scan-jobs`, `--scan-timeout`, `--scan-report`

---

## What stays on the CLI (not in YAML)

These are **imperative-only** — pass as flags even when using `--config`:

| Flag | Purpose |
|------|---------|
| `--rancher` | Rancher version (required) |
| `-o` / `--output` | Output file paths |
| `--kdm`, `--chart`, `--system-chart` | Local offline sources |
| `--registry` | Registry rewrite |
| `--dev` | Dev branch for charts/KDM |
| `--save-config` | TUI only: export YAML after run |

---

## Example configs

### Minimal RKE2 + basic

```yaml
distros: [rke2]
cni: cni_calico
versions:
  rke2: [v1.34.3+rke2r1]
groups: [basic]
```

### Production with scan + Hauler

```yaml
distros: [k3s, rke2]
cni: cni_calico
loadBalancer: true
versions:
  k3s: [v1.32.11+k3s3]
  rke2: [v1.34.3+rke2r1]
groups:
  - basic
  - addon_monitoring
  - addon_logging
export:
  hauler: true
scan:
  enabled: true
  jobs: 4
  timeout: 15m
```

### Prime GC (China)

```yaml
sourceType: prime-gc
distros: [rke2]
cni: cni_calico
versions:
  rke2: [v1.34.3+rke2r1]
groups: [basic]
```

---

## GitOps pattern

```yaml
# .github/workflows/genesisrk.yml
- run: genesisrk generate --rancher=v2.13.1 --config=config/production.yaml -o artifacts/images.txt
- uses: actions/upload-artifact@v4
  with:
    name: rancher-images
    path: artifacts/images.txt
```
