# Getting started

## Install

**From source** (requires [Hangar](https://github.com/cnrancher/hangar) `v1.9.4` as a sibling directory):

```bash
git clone https://github.com/aeltai/Hangar-Genesis.git
git clone https://github.com/cnrancher/hangar.git ../hangar
cd ../hangar && git checkout v1.9.4
cd ../Hangar-Genesis
go build -o bin/genesisrk .
```

**Docker** (clones Hangar during build — no sibling checkout needed):

```bash
docker build -f deploy/azure/Dockerfile.genesis -t genesisrk:latest .
docker run -p 8080:8080 genesisrk:latest
```

## Your first image list

### Option A — declarative (recommended for CI)

```bash
cp config.example.yaml my-rancher.yaml
# edit distros, versions, groups…
genesisrk generate --rancher=v2.13.1 --config=my-rancher.yaml
```

Produces `v2.13.1-images.txt` and `v2.13.1-versions.txt`.

### Option B — interactive TUI

```bash
genesisrk tui --rancher=v2.13.1
```

Use arrow keys, Space to toggle, Enter to confirm. Press **q** or **Ctrl+C** to exit at any time.

Save your selections for later:

```bash
genesisrk tui --rancher=v2.13.1 --save-config=my-rancher.yaml
```

### Option C — web UI

```bash
cd frontend && npm ci && npm run build && cd ..
genesisrk serve --port=8080 --static=frontend/dist
```

Open http://localhost:8080

## Terminal demos

![Version and help](assets/version.gif)

![Generate from config](assets/generate.gif)

![TUI](assets/tui.gif)

![Serve API + UI](assets/serve.gif)

Regenerate GIFs:

```bash
vhs docs/vhs/version.tape
vhs docs/vhs/generate.tape
vhs docs/vhs/tui.tape
vhs docs/vhs/serve.tape
```

## Next steps

After you have `images.txt`:

```bash
# Mirror into your private registry (requires network on a connected host)
hangar mirror -f v2.13.1-images.txt -d registry.example.com --copy-all-platforms

# Or save as tar bundle for air-gap transfer
hangar save -f v2.13.1-images.txt -d ./bundle
hangar load -d ./bundle -r registry.example.com
```

Optional [Hauler manifest](outputs.md#hauler-manifest):

```bash
genesisrk generate --rancher=v2.13.1 --config=my-rancher.yaml --hauler
```

See [airgap-testing.md](airgap-testing.md) for a full e2e script on AWS infra.
