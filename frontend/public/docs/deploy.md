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

**Custom domain:** https://genesisrk.mftools.xyz (after DNS + `./custom-domain.sh bind`)

### Custom domain (genesisrk.mftools.xyz)

Azure Container Apps requires DNS before the hostname can be bound. From `deploy/azure`:

```bash
./custom-domain.sh dns     # print CNAME + TXT records for your DNS provider
./custom-domain.sh check   # verify propagation
./custom-domain.sh bind    # add hostname + managed TLS certificate
```

Add in the **mftools.xyz** zone:

| Type | Name | Value |
|------|------|-------|
| CNAME | `genesisrk` | `genesis-app.bravecoast-8b272aef.westeurope.azurecontainerapps.io` |
| TXT | `asuid.genesisrk` | *(run `./custom-domain.sh dns` for the verification ID)* |

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
