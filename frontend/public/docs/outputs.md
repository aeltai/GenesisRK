# Outputs & exports

GenesisRK produces several artifact types. All modes (CLI, YAML, API, UI) can generate the same outputs.

## Primary outputs

### `images.txt`

Plain text, one image reference per line. Primary input for Hangar and Hauler.

```
docker.io/rancher/rancher:v2.13.1
docker.io/rancher/rancher-agent:v2.13.1
docker.io/rancher/fleet:v0.12.4
...
```

| Mode | How |
|------|-----|
| CLI generate | `-o path.txt` or default `{rancher}-images.txt` |
| CLI tui | Written after tree selection |
| YAML | Implicit — always generated |
| API | `POST /api/export` |
| UI | **Export image list** button |

**Use with Hangar:**

```bash
hangar mirror -f v2.13.1-images.txt -d registry.example.com --copy-all-platforms
hangar save -f v2.13.1-images.txt -d ./bundle
hangar load -d ./bundle -r registry.example.com
```

### `*-versions.txt`

Supported Kubernetes versions per distro for the selected Rancher release.

```
K3s, RKE2 versions for Rancher v2.13.1:

K3s Versions:
v1.32.11+k3s3
v1.31.12+k3s1

RKE2 Versions:
v1.34.3+rke2r1
```

| Mode | How |
|------|-----|
| CLI | `--output-versions path.txt` or default `{rancher}-versions.txt` |
| API/UI | Included in export job temp dir (versions file written during `finish()`) |

### Per-distro lists (optional)

| Flag | File | Contents |
|------|------|----------|
| `--k3s-images` | custom path | K3s Linux only |
| `--rke2-images` | custom path | RKE2 Linux only |
| `--rke2-windows-images` | custom path | RKE2 Windows only |
| `--rke-images` | custom path | RKE1 Linux only |
| `--output-windows` | custom path | All Windows images |
| `--output-source` | custom path | Images with source tags |

---

## Hauler manifest (optional)

Off by default. Hauler-compatible `Images` manifest for [rancher/hauler](https://github.com/rancher/hauler) store workflows.

```yaml
apiVersion: content.hauler.cattle.io/v1
kind: Images
metadata:
  name: 2.13.1-rancher-images
spec:
  images:
    - name: docker.io/rancher/rancher:v2.13.1
```

| Mode | How |
|------|-----|
| CLI | `--hauler` or `--hauler-output=path.yaml` |
| YAML | `export.hauler: true` |
| API | `POST /api/export/hauler` |
| UI | **Export Hauler manifest** button |

**Use with Hauler:**

```bash
hauler store add --files v2.13.1-hauler.yaml
hauler store save --filename rancher-store.tar.zst
# transfer to air-gap, then:
hauler store load --filename rancher-store.tar.zst
```

---

## Scan report (optional)

CSV vulnerability report from Trivy (via Hangar scan integration).

| Mode | How |
|------|-----|
| CLI generate/tui | `--scan` |
| YAML | `scan.enabled: true` |
| API | `POST /api/scan` → poll → `GET /api/scan/report/{id}` |
| UI | **Scan** button → **Download report (CSV)** |

When `--scan` is used during generate, scan summaries are also appended as `# scan: CRITICAL=0 HIGH=2` comments on each line in `images.txt`.

---

## Saved YAML config

Not an image output — but the TUI can **export your selections** as declarative config:

```bash
genesisrk tui --rancher=v2.13.1 --save-config=production.yaml
```

Reuse in CI:

```bash
genesisrk generate --rancher=v2.13.1 --config=production.yaml
```

---

## Output comparison

| Artifact | Required | Default path | Hangar | Hauler |
|----------|----------|--------------|--------|--------|
| `images.txt` | yes | `{rancher}-images.txt` | mirror/save/load | store input |
| `versions.txt` | yes | `{rancher}-versions.txt` | — | — |
| Hauler YAML | no | `{rancher}-hauler.yaml` | — | store add |
| Scan CSV | no | `{output-base}-scan-report.csv` | scan | — |
| Saved config | no | user-defined | — | — |
