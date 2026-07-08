# Deployment

GenesisRK ships as a single container with the Go API and Vue frontend.

## Docker

```bash
docker build -f deploy/azure/Dockerfile.genesis -t genesisrk:latest .
docker run -p 8080:8080 genesisrk:latest
```

## Azure Container Apps (recommended)

```bash
cd deploy/azure
cp .env.example .env   # set ACR_NAME, RESOURCE_GROUP_NAME
./container-app.sh all
```

See [deploy/azure/README.md](../deploy/azure/README.md) for App Service and AKS options.

## Environment variables

| Variable | Description |
|----------|-------------|
| `GITHUB_TOKEN` | Avoid GitHub API rate limits in production |
| `RANCHER_APPS_API_USER` | Application Collection API user |
| `RANCHER_APPS_API_PASSWORD` | Application Collection API token |

## Live demo

https://genesis-app.bravecoast-8b272aef.westeurope.azurecontainerapps.io/
