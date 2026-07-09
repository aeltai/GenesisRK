# Repository layout

**Canonical project:** [github.com/aeltai/GenesisRK](https://github.com/aeltai/GenesisRK)

GenesisRK is a **standalone repository** (not a GitHub fork). It embeds [Hangar](https://github.com/cnrancher/hangar) as the `hangar genesis` CLI and ships the Vue web UI.

## Remotes (suggested)

| Remote | URL | Purpose |
|--------|-----|---------|
| `origin` | `https://github.com/aeltai/GenesisRK.git` | Push GenesisRK releases |
| `upstream` | `https://github.com/cnrancher/hangar.git` | Pull upstream Hangar changes |

Legacy remotes:

- `https://github.com/aeltai/hangar.git` — old fork of cnrancher/hangar; detach via **Settings → Danger zone → Leave fork network** if you still use it.
- `hangar-genesis` redirects to GenesisRK (renamed).

## Branches

| Branch | Role |
|--------|------|
| `main` | Production line (deployed to genesisrk.mftools.xyz) |
| `feature/*` | Short-lived feature work; merge to `main` then delete |
