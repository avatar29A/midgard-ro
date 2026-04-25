# rAthena Server — Distribution Research & Recommendation

**Related:** RFC [#49](https://github.com/avatar29A/midgard-ro/issues/49), Track A issue [#50](https://github.com/avatar29A/midgard-ro/issues/50)
**Date:** 2026-04-24
**Status:** Decision — awaiting sign-off from @avatar29A

---

## 1. Context

RFC #49 picked **rAthena** (not Hercules) as the server for MVP. This doc resolves task **A1**: pick which rAthena distribution/image we bootstrap on.

Constraints from the RFC:

- Local-only, not public
- Prontera + `prt_fild01–03` must work out of the box
- One pre-provisioned Novice account is enough
- Client must keep handling both `0x0069` (old) and `0x0AC4` (modern) era packets — so the server's packet version choice only needs to match *one* of those eras
- Setup must be reproducible in under 5 minutes on a clean machine (issue #50 Done-When)
- Primary dev platform is macOS (Apple Silicon)

## 2. Options considered

### A. Official `rathena/rathena` `tools/docker/` (in-repo)

Lives inside the main rAthena repo at [`tools/docker/`](https://github.com/rathena/rathena/tree/master/tools/docker). Two services: a **builder** container (Debian-based, compiles rAthena from the checkout) and a **MariaDB** container. SQL files in `sql-files/` are auto-imported into MariaDB on first startup.

- **Default packet version:** `20211103` (Nov 3, 2021) — belongs to the modern `0x0AC4` era our client already handles
- **DB credentials:** `ragnarok` / `ragnarok` / `ragnarok` (user/pass/db)
- **Ports:** MariaDB 3306; rAthena login 6900, char 6121, map 5121 (standard rAthena)
- **State:** actively maintained as part of the main repo
- **Scope:** explicitly dev-only — docs state "not suitable for production"
- **Pre-seeded admin/user accounts:** none by default (have to insert via SQL)
- **Apple Silicon:** no official notes. Base image is Debian; builder compiles from source so an `arm64` build is feasible. Docker Desktop on Apple Silicon also runs `amd64` under Rosetta 2 cleanly if native builds hit friction.

### B. `cmilanf/docker-rathena` (community)

Alpine-based, [single-container](https://github.com/cmilanf/docker-rathena) running login+char+map together. Tracks rAthena master. Has an optional `MYSQL_ACCOUNTSANDCHARS` flag that pre-creates 2 GM + 5000 bot accounts — convenient, but designed for OpenKore bot testing, not our use case.

- Pro: single image, fewer moving parts
- Con: unclear maintenance cadence (no recent commit date surfaced)
- Con: all-in-one container is harder to debug when one subsystem misbehaves
- Con: Alpine + musl occasionally surfaces obscure rAthena bugs that don't hit glibc builds

### C. `florentortiz/RAthena-Docker`

Full stack: rAthena + FluxCP (PHP web admin) + PHPMyAdmin + WSProxy (for roBrowser). Useful for a public-facing demo server; **overkill for MVP** — we only need the three game servers + a DB, and adding FluxCP/PHPMyAdmin widens the attack surface and maintenance burden for no MVP win.

### D. `cdelorme/rathena-docker`

Demonstrates **distributed** rAthena deployment (login, char, map on separate hosts). Interesting for future scaling; orthogonal to MVP.

### E. Hub-only images: `davidsiaw/rathena`, `study/rathena`

Pre-built, convenient, but pin an unknown upstream commit. Rejected — we want to build from a known rAthena SHA.

## 3. Decision

**Use Option A — the official `rathena/rathena` `tools/docker/` setup as our base, vendored into `docker/rathena/` in this repo** (copy + pin, not submodule).

### Why

1. **Source of truth.** It lives in the rAthena repo, so it's always in sync with whatever rAthena commit we pin. If the rAthena devs change server CLI flags or DB schema, the Docker config changes with it.
2. **Dev-focused by design.** The official docs call it out as a development environment, which is exactly our use case. Production concerns (TLS, private DB network, resource limits) are deliberately absent — we don't need them.
3. **Two-service separation** (builder + MariaDB) makes it easy to exec into either for debugging without teardown.
4. **Packet version alignment.** Default `20211103` maps to the `0x0AC4`-era packets our client already handles. The fact that we also keep the `0x0069` code path means we aren't locked to this version and can downgrade later (e.g. `20151104`) without a client change if needed.
5. **Customization surface is clean.** `BUILDER_CONFIGURE` env var lets us pass `--enable-packetver=...` without forking anything. A small SQL seed file (below) handles the Novice account and Prontera save-point without touching upstream configs.
6. **Apple Silicon risk is low.** Even in the worst case (builder needs `amd64`), Rosetta 2 handles it without meaningful performance loss for a local dev server. Most likely an `arm64` native build just works — we'll confirm in A2.

### Why not the others

- **cmilanf** is close to workable, but all-in-one container + unclear maintenance + Alpine/musl is three small risks stacked in exchange for no real MVP benefit.
- **florentortiz** pulls in FluxCP / PHPMyAdmin — features we aren't asked for and don't need to maintain.
- **cdelorme** solves scaling, not bootstrapping.
- Pre-built hub images pin unknown upstream commits.

## 4. Vendoring plan (for A2, not this doc)

```
midgard-ro/
└── docker/
    └── rathena/
        ├── docker-compose.yml          # copy of tools/docker/docker-compose.yml, pinned tag
        ├── builder.env                  # BUILDER_CONFIGURE with --enable-packetver=20211103
        ├── seed/
        │   └── 01_mvp_novice.sql        # insert test account + pre-created Novice char
        └── README.md                    # "see docs/research/rathena-setup.md"
```

Pin to an rAthena git SHA in the builder's `git checkout` step so CI and local setups are byte-identical.

## 5. Open risks

1. **Apple Silicon compile.** If the official Debian builder image doesn't have a multi-arch tag, we'll either build locally from the Dockerfile with `--platform linux/arm64` or fall back to Rosetta. Validated in A2.
2. **Port 3306 collision.** Guide calls this out — users with a local MySQL will need to remap. Documented in A3.
3. **Client date 2021-11-03 feature set.** Some RO clients expect specific opcodes around this date. Our client already handles the modern-era packets, so the risk is low, but A2's smoke test (login → char select) will confirm.
4. **First-run compile is slow.** Not a correctness issue but counts against the <5-minute Done-When. If compile itself takes >5 min on Apple Silicon, we'll cache the built binary as a named Docker volume so subsequent runs skip recompile.

## 6. Next actions

- Sign-off on this doc → commit
- Begin **A2**: vendor `docker-compose.yml` under `docker/rathena/`, pin rAthena SHA, author `seed/01_mvp_novice.sql`, verify `docker compose up` on Apple Silicon
- **A3**: operational doc (first-run, reset-db, log locations)
- **A4**: default `cmd/client` config to `localhost:6900` behind a config flag (Korangar server remains reachable for comparison)

---

## Sources

- [rAthena official Docker user guide](https://rathena.github.io/user-guides/installing/docker/)
- [rathena/tools/docker/README.md](https://github.com/rathena/rathena/blob/master/tools/docker/README.md)
- [rathena/tools/docker/docker-compose.yml](https://github.com/rathena/rathena/blob/master/tools/docker/docker-compose.yml)
- [rathena/src/config/packets.hpp (default packetver 20211103)](https://github.com/rathena/rathena/blob/master/src/config/packets.hpp)
- [cmilanf/docker-rathena](https://github.com/cmilanf/docker-rathena)
- [florentortiz/RAthena-Docker](https://github.com/florentortiz/RAthena-Docker)
- [cdelorme/rathena-docker](https://github.com/cdelorme/rathena-docker)
- [Docker on Apple Silicon — ARM64, Rosetta, performance](https://oneuptime.com/blog/post/2026-01-16-docker-mac-apple-silicon/view)
