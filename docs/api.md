# REST API

Start the server with:

```bash
genesisrk serve --port=8080 --static=frontend/dist
```

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/rancher-versions` | List Rancher release versions |
| GET | `/api/step1-options?rancher=v2.13.1` | Distro/version capabilities |
| POST | `/api/generate` | Build image tree for a job |
| POST | `/api/export` | Export `images.txt` for a job |
| POST | `/api/export/hauler` | Export Hauler Images manifest YAML |
| POST | `/api/check-availability` | Check registry reachability |
| POST | `/api/scan` | Start async Trivy scan |
| GET | `/api/scan/status/{id}` | Scan job status |
| GET | `/api/scan/report/{id}` | Download scan CSV |
| GET | `/api/logs` | Recent server log lines |
| GET | `/api/release-notes` | GitHub release notes |

## Export hauler manifest

```bash
curl -X POST http://localhost:8080/api/export/hauler \
  -H 'Content-Type: application/json' \
  -d '{"jobId":"...","selectedComponentIDs":["basic"],"chartNames":[],"selectedImageRefs":[]}' \
  -o hauler-manifest.yaml
```

## Health

The root path serves the Vue UI when `--static` is set. API routes take precedence over static files.
