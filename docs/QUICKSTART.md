# Quick Start: Connect & Login Locally

End-to-end path from a clean checkout to walking around Prontera against
your own rAthena server. Fully validated on macOS Apple Silicon with colima.

> Note: as of 2026-04-25 the client reaches Prontera and renders the map,
> but movement, NPCs, music, and combat are not yet wired (RFC #49 Tracks
> B–F). The map server will time you out after ~30 s — expected.

---

## 1. Prerequisites

You need:

- **macOS** (Apple Silicon validated; Intel should work) or Linux
- **8 GB RAM** for the Docker VM (rAthena's C++ build OOMs on less)
- **~5 GB disk** for the rAthena clone + compiled binaries + DB
- A **legitimate copy of `data.grf` and `rdata.grf`** from a Ragnarok Online
  installation (not redistributable — bring your own)

Install everything else in one shot:

```bash
make env-install-macos
```

That brings in: Go, pkg-config, SDL2, colima, docker, docker-compose. After
the brew install completes, the script reminds you to add the docker plugin
path to `~/.docker/config.json` if it's not already there.

Verify with:

```bash
make env-check
```

All entries should print a version. If any say `NOT INSTALLED`, fix that
before continuing.

---

## 2. Start the Docker VM (colima)

```bash
colima start --memory 8 --cpu 4
```

The 8 GB / 4 CPU defaults are required — rAthena's `skill.cpp` is the
largest C++ TU in the codebase and gets OOM-killed at smaller sizes. Once
the VM is up, `colima start` is a no-op on subsequent runs.

---

## 3. Set your config

```bash
make config
```

Creates `config.yaml` (gitignored) from `config.example.yaml`. Then edit
the GRF paths to point at your `data.grf` and `rdata.grf`:

```yaml
data:
  grf_paths:
    - "/Users/you/path/to/data.grf"
    - "/Users/you/path/to/rdata.grf"
```

The default credentials (`midgard-test` / `midgard-test`) and server
address (`127.0.0.1:6900`) match the seeded test account, leave them
alone for the first run.

---

## 4. Start the server

```bash
make server-up
```

What this does:

1. On first run only: `docker/rathena/setup.sh` clones rAthena at the pinned
   SHA from [`docker/rathena/pin.txt`](../docker/rathena/pin.txt) into
   `docker/rathena/build/rathena/` and copies our seed SQL into upstream's
   `sql-files/`.
2. `docker compose up -d` starts five containers:
   - `midgard-rathena-db` — MariaDB, auto-loads schema + our seed
   - `midgard-rathena-builder` — compiles rAthena (~3 min on M-series)
   - `midgard-rathena-login` — listening on `localhost:6900`
   - `midgard-rathena-char` — listening on `localhost:6121`
   - `midgard-rathena-map` — listening on `localhost:5121`

First run is roughly **5 minutes total** (clone + Alpine package install +
compile + DB init). Subsequent `make server-up` is a few seconds — the
build is cached.

Watch progress with:

```bash
make server-logs
```

When you see `The login-server is ready (Server is listening on the port 6900)`
and `Map Server is now online`, you're good.

---

## 5. Launch the client

```bash
make run
```

You should see, in order:

1. Blue login window with username/password pre-filled — click **Login**.
2. Character select with one entry: **MidgardTest** (Lv 1 Novice).
3. Click **Enter Game** → loading → 3D view of Prontera.

Or use the convenience target that does both in one shot:

```bash
make play   # implicitly: make server-up + make config + go run
```

Server logs will show:

```
'MidgardTest' logged in. (AID/CID: '2000000/150000', IP: '...')
```

---

## 6. What works (today)

| Feature | Status |
|---|---|
| Login → char select → enter game | ✅ |
| 3D map render (GND + RSW + RSM models) | ✅ |
| Camera zoom (scroll) and rotate (right-mouse drag) | ✅ |
| Walking | 🚧 RFC #49 Track B (#51) |
| Music / BGM | 🚧 Track C (#52) |
| HUD (HP/SP, chat) | 🚧 Track D (#53) |
| NPCs and mobs | 🚧 Track E (#54) |
| Combat | 🚧 Track F (#55) |
| Keep-alive / session lifecycle | 🚧 Track B (you'll get a "Network error" after ~30 s — expected) |

---

## 7. Stopping things

```bash
make server-down       # stop server, keep DB volume + compiled binaries
make server-reset      # stop + wipe DB volume; next server-up re-seeds
make server-rebuild    # nuke everything, force re-clone + re-compile
colima stop            # stop the Docker VM entirely
```

---

## 8. Troubleshooting

### `make server-up` fails with `Cannot connect to the Docker daemon`

The VM isn't running. `colima start --memory 8 --cpu 4`.

### Compile dies with `Killed signal terminated program cc1plus`

OOM. Increase colima memory: `colima stop && colima start --memory 8 --cpu 4`.

### `docker: 'compose' is not a docker command`

The Compose v2 plugin isn't installed or isn't on the plugin path. Run
`make env-install-macos` and check `~/.docker/config.json` contains:

```json
"cliPluginsExtraDirs": ["/opt/homebrew/lib/docker/cli-plugins"]
```

(Re-run `make env-install-macos` after editing.)

### Login server crashes at startup with `Can't connect to server on '127.0.0.1'`

Means the cross-server IP overrides aren't mounted. Pull the latest
`docker/rathena/docker-compose.yml` — it must mount
`./build/rathena/tools/docker/asset/inter_conf.txt` (and `char_conf.txt`,
`map_conf.txt`) into the builder/login/char/map services.

### Port 3306 already in use

You have a local MySQL/MariaDB running. Either stop it, or edit
`docker/rathena/docker-compose.yml` to remap (e.g. `"3307:3306"`).

### Client shows "Network error" after a while in-game

Map server timed us out because the client doesn't reply to keep-alive
ticks yet. Tracked in #51 (Track B). Re-launching reconnects fine.

---

## 9. Reference

- [`docker/rathena/README.md`](../docker/rathena/README.md) — server stack details
- [`docs/research/rathena-setup.md`](research/rathena-setup.md) — distribution choice rationale
- [RFC #49](https://github.com/avatar29A/midgard-ro/issues/49) — MVP scope and open questions
- [Track A issue #50](https://github.com/avatar29A/midgard-ro/issues/50) — server work
- `make help` — full target list with descriptions
