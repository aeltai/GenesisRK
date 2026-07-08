# Web UI

The GenesisRK web UI is a Vue 3 SPA served by `genesisrk serve --static=frontend/dist`.

**Live demo:** https://genesisrk.mftools.xyz  
**API explorer:** https://genesisrk.mftools.xyz/api/docs

## Run locally

```bash
cd frontend && npm ci && npm run build && cd ..
genesisrk serve --port=8080 --static=frontend/dist
```

Open http://localhost:8080

## Workflow

The UI mirrors the three-step CLI/TUI flow:

### Step 1 — Configure

| Control | Maps to |
|---------|---------|
| Rancher version | `--rancher` / API `rancherVersion` |
| Source (Community / Prime) | YAML `sourceType` / API `isRPMGC` |
| Distros (K3s, RKE2, RKE1) | YAML `distros` / API `distros` |
| CNI | YAML `cni` / API `cni` |
| Load balancer toggles | YAML `loadBalancer` / API `lb*` fields |
| Windows nodes | YAML `includeWindows` / API `includeWindows` |
| K8s version pickers | YAML `versions` / API `*Versions` |
| Application Collection | YAML `includeAppCollectionCharts` |

Click **Generate** → calls `POST /api/generate` → stores `jobId`.

### Step 2 — Component tree

Interactive tree (same data as TUI):

- **Basic** — Rancher core, Fleet, distro, CNI
- **AddOns** — Monitoring, logging, storage, etc.
- **App Collection** — when enabled

Use checkboxes to select groups, charts, and individual images.

### Step 3 — Export & next steps

| Action | API | Output |
|--------|-----|--------|
| Export image list | `POST /api/export` | `images.txt` download |
| Export Hauler manifest | `POST /api/export/hauler` | `hauler-manifest.yaml` |
| Check availability | `POST /api/check-availability` | Per-image status |
| Scan (Trivy) | `POST /api/scan` + poll | CSV report |
| Destination registry | UI-only | Copy-paste `hangar mirror` commands |

**Next steps** panel shows Hangar and Hauler commands tailored to your destination registry.

## Theme

Toggle dark/light in the header. Preference is stored in `localStorage` (`genesis-theme`).

## Documentation panel

The in-app **Documentation** link opens embedded help synced from the docs. For the full reference see [docs/README.md](README.md).

## Environment

For production deployments set:

```bash
GITHUB_TOKEN=ghp_...                    # GitHub API rate limits
RANCHER_APPS_API_USER=...               # Application Collection
RANCHER_APPS_API_PASSWORD=...
```

See [deploy.md](deploy.md).
