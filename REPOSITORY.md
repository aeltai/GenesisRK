# Repository layout

**Canonical project:** [github.com/aeltai/GenesisRK](https://github.com/aeltai/GenesisRK)

GenesisRK is a **standalone repository** (not a GitHub fork). It embeds [Hangar](https://github.com/cnrancher/hangar) as the `hangar genesis` CLI and ships the Vue web UI.

## Remotes (suggested)

| Remote | URL | Purpose |
|--------|-----|---------|
| `origin` | `https://github.com/aeltai/GenesisRK.git` | Push GenesisRK releases |
| `upstream` | `https://github.com/cnrancher/hangar.git` | Pull upstream Hangar changes |

```bash
git remote set-url origin https://github.com/aeltai/GenesisRK.git
git remote set-url genesis https://github.com/aeltai/GenesisRK.git   # optional alias
git remote add upstream https://github.com/cnrancher/hangar.git      # if missing
```

Legacy remotes:

- `https://github.com/aeltai/hangar.git` — old fork of cnrancher/hangar; detach via **Settings → Danger zone → Leave fork network** if you still use it.
- `hangar-genesis` redirects to GenesisRK (renamed).

## Branches

| Branch | Role |
|--------|------|
| `feature/generate-list-genesis` | **Default branch** — production line (deployed to genesisrk.mftools.xyz) |
| `main` | Legacy standalone-restructure line (protected; do not use for new work) |
| `feature/*` | Short-lived work; merge to default branch then delete |
