# Self-hosted rAthena (MVP)

Wraps the official rAthena Docker setup with our MVP-specific seed SQL: a
single Novice test account pre-spawned in Prontera. Implements Track A of
RFC [#49](https://github.com/avatar29A/midgard-ro/issues/49).

See [`docs/research/rathena-setup.md`](../../docs/research/rathena-setup.md)
for the distribution-choice rationale.

## Quick start

```bash
# 1. Clone rAthena at our pinned SHA into ./build/rathena and stage seed SQL
./setup.sh

# 2. Build images and start all services
docker compose up
```

First run takes ~5 minutes (Alpine package install + rAthena compile).
Subsequent `docker compose up` invocations skip both.

## What's here

| File | Purpose |
|---|---|
| [`pin.txt`](pin.txt) | The rAthena git SHA we build from (single source of truth) |
| [`setup.sh`](setup.sh) | Clones rAthena at the pin and copies our seed into upstream `sql-files/` |
| [`docker-compose.yml`](docker-compose.yml) | 5-service stack: db + builder + login + char + map |
| [`seed/zzz_mvp_novice.sql`](seed/zzz_mvp_novice.sql) | MVP test account + pre-created Novice char |
| `build/` | gitignored rAthena clone created by `setup.sh` |

## Connection details

| Service | Host port |
|---|---|
| Login | `localhost:6900` |
| Char | `localhost:6121` |
| Map | `localhost:5121` |
| MariaDB | `localhost:3306` (`ragnarok` / `ragnarok` / `ragnarok`) |

Test account: `midgard-test` / `midgard-test` — has one Novice character
named `MidgardTest` spawned at `prontera (156, 191)`.

Packet version: `20211103` (modern era — exercises the `0x0AC4` code path
in [`internal/network/client.go`](../../internal/network/client.go)).

## Reset

```bash
# Wipe DB only (rAthena code stays cached)
docker compose down --volumes

# Wipe everything (forces re-clone + re-compile next time)
docker compose down --volumes
rm -rf build/
```

## Apple Silicon (M-series Macs)

On Apple Silicon we recommend [colima](https://github.com/abiosoft/colima)
as the Docker runtime. Suggested startup for native `arm64` with
Rosetta-backed `amd64` fallback:

```bash
colima start --vm-type=vz --vz-rosetta --cpu 4 --memory 6
```

The Alpine 3.23 base in the upstream Dockerfile has a native `arm64` tag,
so the build runs natively without Rosetta in the common case. If
`docker compose build` ever fails to find an `arm64` variant of a
dependency, prefix with `DOCKER_DEFAULT_PLATFORM=linux/amd64`.

## Port collisions

If `localhost:3306` is already in use (e.g. local MySQL), edit
`docker-compose.yml`'s `db.ports` to remap, e.g. `"3307:3306"`. The other
ports (6900, 6121, 5121) rarely collide.

## NOT for production

The upstream `tools/docker/` setup is dev-only — the project explicitly
states it is not suitable for production. Don't expose these ports
publicly. See [`docs/research/rathena-setup.md`](../../docs/research/rathena-setup.md) §3.
