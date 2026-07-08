# Deployment

## Docker

Builds GenesisRK + Vue frontend. Clones Hangar `v1.9.4` during image build (no sibling checkout needed).

```bash
docker build -f deploy/azure/Dockerfile.genesis -t genesisrk:latest .
docker run -p 8080:8080 \
  -e GITHUB_TOKEN=ghp_... \
  genesisrk:latest
```

Entrypoint: `genesisrk serve --port=8080 --static=/app/frontend/dist`

## Azure Container Apps (recommended)

```bash
cd deploy/azure
cp .env.example .env   # ACR_NAME, RESOURCE_GROUP_NAME, AZURE_SUBSCRIPTION_ID
az login
./container-app.sh all   # build, push to ACR, deploy
```

**Live instance:** https://genesis-app.bravecoast-8b272aef.westeurope.azurecontainerapps.io/

| Script command | Action |
|----------------|--------|
| `./container-app.sh all` | Build, push, deploy |
| `./container-app.sh up` | Deploy existing image |
| `./container-app.sh build-up` | Build + push + deploy |

See [deploy/azure/README.md](../deploy/azure/README.md) for App Service and AKS.

## Environment variables

| Variable | Used by | Description |
|----------|---------|-------------|
| `GITHUB_TOKEN` | API | GitHub releases / rate limits |
| `RANCHER_APPS_API_USER` | Generate | Application Collection API |
| `RANCHER_APPS_API_PASSWORD` | Generate | Application Collection token |

## Production checklist

- [ ] Set `GITHUB_TOKEN` on the container app
- [ ] Configure ingress (HTTPS via Azure or custom domain)
- [ ] Pin image tag instead of `:latest` for reproducible deploys
- [ ] Application Collection credentials only if needed

## Local development

```bash
# Terminal 1 — API with hot-reload frontend
cd frontend && npm run dev    # Vite dev server proxies /api

# Terminal 2 — Go API
go run . serve --port=8080    # API only; point Vite proxy at :8080
```

Or combined production-like:

```bash
cd frontend && npm run build && cd ..
genesisrk serve --port=8080 --static=frontend/dist
```
