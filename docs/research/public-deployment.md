# Going Public — Server Deployment & Client Readiness Research

**Related:** RFC [#49](https://github.com/avatar29A/midgard-ro/issues/49) (MVP scope — deliberately local-only), Track A issue [#50](https://github.com/avatar29A/midgard-ro/issues/50), [`docs/research/rathena-setup.md`](rathena-setup.md)
**Date:** 2026-09-01
**Status:** Draft — awaiting scope decision from @avatar29A

---

## 1. Why this doc exists

Everything we have was built for one machine. RFC #49 says "local-only, not
public" and [`docker/rathena/README.md`](../../docker/rathena/README.md) ends
with a section titled **NOT for production**. That was the right call for the
MVP, and it means the step to a public service is not "open the firewall" —
it touches the server configuration, the deployment shape, the client's
connection handling, and the project's licensing posture.

This doc inventories what breaks, why, and in what order to fix it. It does
not decide anything; §9 lists the questions that need Boris's answer first.

---

## 2. First: what does "public" mean?

Three quite different targets hide behind the word. They share a lot of work
but differ sharply in the tail.

| | **A. Private alpha** | **B. Open demo** | **C. Real public server** |
|---|---|---|---|
| Who connects | Invited testers, known by name | Anyone with the client + a GRF | Anyone, expected to stay |
| Accounts | Hand-seeded, as today | Self-registration | Self-registration + recovery |
| Characters | Throwaway | Wiped on any redeploy | Must survive forever |
| Uptime | Best effort | Best effort, announced downtime | Real SLO, backups, restore drills |
| Abuse surface | Trust-based | Rate limits, ipban, GM coverage | Moderation, appeals, bot defense |
| Extra work over A | — | Registration, asset onboarding, reconnect | Backups, migrations, moderation, support |

**Recommendation: make A the next milestone and B the target.** A is
reachable in roughly a week of focused work and immediately buys the thing
we cannot get locally — a second person on the same map over real WAN
latency, which is the single largest untested axis in this client. C should
not be attempted until the client is feature-complete enough that a
character is worth keeping; committing to persistent characters early turns
every schema change into a migration.

The rest of this doc is written for **A → B**. Where C adds work, it is
called out.

---

## 3. What the RO protocol forces on us

These are not choices. They fall out of the wire format and constrain every
hosting decision downstream.

### 3.1 The client is redirected twice, by raw IPv4

Login hands back the char server's address, char hands back the map
server's. Both arrive as **four raw bytes**, not a hostname:

- [`internal/game/states/login.go:219-221`](../../internal/game/states/login.go#L219-L221) — `fmt.Sprintf("%d.%d.%d.%d", ...)` over the char-server entry
- [`internal/game/states/charselect.go:335`](../../internal/game/states/charselect.go#L335) — `info.GetIP()` for the map server

rAthena resolves the hostname in `char_ip` / `map_ip` **once, at server
startup**, and puts the resulting IPv4 on the wire.

**Consequences:**

1. **We need a stable public IPv4.** Not a hostname, not a load balancer with
   rotating addresses, not IPv6. Only the *login* address in
   [`config.yaml`](../../config.example.yaml) is a hostname the client
   resolves itself; the other two hops are whatever the server announces.
2. **No CDN, no Cloudflare proxying, no DNS failover** for char/map. Cloudflare's
   free tier does not proxy arbitrary TCP anyway (Spectrum is enterprise).
3. **Changing the server's IP is a client-visible event only for the login
   address.** char/map follow automatically once the server config is updated —
   but a running server must be restarted to pick up a new IP.
4. **NAT must be 1:1.** If the host's public IP differs from its interface IP
   (typical on cloud VMs with a floating/elastic IP), `char_ip` / `map_ip` must
   be set to the *public* address explicitly, and `conf/subnet_athena.conf`
   must not shadow it — see §4.1.

### 3.2 The password crosses the internet in cleartext

`CA_LOGIN` (0x0064) carries a 24-byte plaintext password
([`internal/network/packets/packets.go:77-84`](../../internal/network/packets/packets.go#L77-L84)),
and rAthena's stock `use_MD5_passwords: no` then stores it in the `login`
table in cleartext too. There is no TLS anywhere in the RO protocol.

**Consequences:**

1. Anyone on the path — public Wi-Fi, a transit provider, our own hoster —
   can read credentials. This is inherent to RO and true of every private
   server in existence, but it must be stated to users, not glossed over.
2. **Password reuse is the real risk.** Registration must refuse to look like
   a normal account signup: no email as the login name, a visible warning,
   and ideally a generated password rather than a chosen one.
3. `use_MD5_passwords: yes` protects the *database at rest* (unsalted MD5 —
   weak, but far better than cleartext if the DB leaks). It does not protect
   the wire. Enable it, and understand what it does and does not buy.
4. Since we own both ends, a real fix is possible later: a TLS or Noise
   wrapper negotiated before the login handshake. It requires patching
   rAthena (a fork we then have to maintain) — worth an RFC of its own, out
   of scope for A/B.

### 3.3 Three long-lived TCP ports, no HTTP

6900 / 6121 / 5121, plus 8888 if the web server is enabled. Nothing here is
HTTP, so ordinary reverse proxies, ACME, and HTTP-level DDoS scrubbing do not
apply. Anything in front must be an L4 TCP proxy that preserves the address
the server announces (see §3.1).

---

## 4. Server-side work

### 4.1 P0 — blockers, nothing works publicly without these

| # | What | Where | Detail |
|---|---|---|---|
| S1 | `char_ip` / `map_ip` still `127.0.0.1` | [`tools/docker/asset/char_conf.txt`, `map_conf.txt`](../../docker/rathena/docker-compose.yml) (mounted from the clone) | Every remote client is redirected to *its own* loopback and hangs at char select. Must become the public IPv4. These files live inside the pinned rAthena clone and are mounted directly — vendor our own copies under `docker/rathena/conf/` first, exactly as we already did for `battle_conf.txt`. |
| S2 | `subnet_athena.conf` maps `255.0.0.0` → `127.0.0.1` | rAthena clone, `conf/subnet_athena.conf` | rAthena's LAN/WAN logic overrides `char_ip`/`map_ip` for clients it believes are on the same subnet. With a `/8` mask over `127.0.0.1` this is harmless locally and wrong publicly. Must be emptied or set to the real private subnet. |
| S3 | Inter-server credentials are stock `s1` / `p1` | `conf/char_athena.conf`, `conf/map_athena.conf` | These authenticate char→login and map→char. Left at default on a public login port, anyone can attach a rogue char/map server to our login server. Change both, and the matching row in the `login` table. |
| S4 | MariaDB published on `0.0.0.0:3306` with `ragnarok`/`ragnarok` | [`docker/rathena/docker-compose.yml`](../../docker/rathena/docker-compose.yml) | Remove the `ports:` mapping entirely — only the compose network needs it. Rotate the password and the `MYSQL_ROOT_PASSWORD` out of the compose file into an env file / secret. |
| S5 | Seeded test account is `group_id 99` (Admin, all 313 at-commands) | [`docker/rathena/seed/zzz_mvp_novice.sql`](../../docker/rathena/seed/zzz_mvp_novice.sql) | The seed exists so `@go`/`@item`/`@monster` are reachable in dev. A public deployment must not run this seed at all — split the seed into `dev` and `public` sets and only stage the one that matches. |
| S6 | No self-registration | `login_athena.conf: new_account: no` | Nobody but us can get an account. Two options in §4.4. |
| S7 | PIN code is on and forced, client cannot answer | `char_athena.conf: pincode_enabled: yes`, `pincode_force: yes` | Our client has no handler for the pincode packets — only lengths for 0x08B8/0x08B9 exist ([`lengths.go:702-703`](../../internal/network/packets/lengths.go#L702-L703)). Seeded accounts sidestep it; a freshly registered account will be asked to set a PIN and the client will not respond. **Set `pincode_enabled: no`** for now; implementing PIN properly is a client feature (§5, C4). |
| S8 | Deployment shape is a dev shape | [`docker/rathena/docker-compose.yml`](../../docker/rathena/docker-compose.yml) | The whole rAthena source tree is bind-mounted and the servers run out of it; there is no `restart:` policy, no healthchecks (`char` and `map` use bare `depends_on`, which does not wait for readiness), no resource limits, no log rotation. See §4.3. |

### 4.2 P1 — hardening

- **`use_MD5_passwords: yes`** (§3.2). Note this is not retroactive — existing
  cleartext rows must be converted or wiped. Do it before the first real user.
- **`ipban_enable: yes`** is already on with dynamic ban after 7 failures in
  5 min; verify it actually fires once the port is public, since it depends on
  the `ipbanlist` table and the login server seeing real client IPs (an L4
  proxy that hides them defeats it).
- **`conf/packet_athena.conf`** already has `enable_ip_rules: yes` and DDoS
  counters (`ddos_interval: 3000`, `ddos_count: 5`). These are naive but free —
  leave on, and know they will not stop a real flood.
- **Host firewall**: allow exactly 6900/6121/5121 inbound, deny 3306 and 8888
  unless deliberately used. Do not rely on Docker's `ports:` alone —
  Docker writes its own iptables rules that bypass UFW on Linux.
- **`use_web_auth_token: yes`** is on and implies the rAthena `web-server`
  (8888). Our client stores the token from `AC_ACCEPT_LOGIN2` but never calls
  the web server, so it can stay unexposed. Confirm nothing breaks with it off.
- **`char_del_delay: 86400` / `char_del_option: 2`** — character deletion needs
  a birthdate or email confirmation. Neither is implemented client-side;
  decide the policy before users have characters worth deleting.
- **GM policy**: `gm_allow_group: 99`. Who gets a GM account, and is it a
  separate account from the one that plays? (Answer: yes, always.)
- **`area_size: 28`** in [`docker/rathena/conf/battle_conf.txt`](../../docker/rathena/conf/battle_conf.txt)
  is double the stock 14, and the file itself notes "a populated one would want
  this back near stock." Cost is quadratic-ish in players per map. Keep 28 for
  A (few testers), plan to revisit for B.

### 4.3 P1 — turn the dev stack into a deployable one

Concretely, what changes in the compose file:

1. **Bake an image instead of mounting source.** Build rAthena at the pin in
   CI (GitHub Actions → GHCR), producing an image with the binaries, `db/`,
   `npc/` and our `conf/import/` baked in. This also removes the
   **8 GB-RAM build requirement from the production host** — the compile
   happens in CI, the host only runs the binaries and can be a 2 GB box.
   This is the single highest-leverage item in this section.
2. **Separate `docker-compose.public.yml`** rather than editing the dev one.
   Different images, no source mounts, no 3306, `restart: unless-stopped`,
   `mem_limit` per service, and `logging: driver: json-file` with
   `max-size`/`max-file` so logs cannot fill the disk.
3. **Real readiness gating.** `char` and `map` currently `depends_on: [login]`
   with no condition, so on a cold boot they can start before login is
   accepting and sit in a retry loop. Add healthchecks (a TCP probe on each
   port is enough) and `condition: service_healthy`.
4. **Config as a mount, secrets as env.** `conf/import/*.txt` vendored in this
   repo (as `battle_conf.txt` already is), with the IP and the inter-server
   password injected at deploy time rather than committed.
5. **Backups.** `mysqldump` of the `ragnarok` schema on a timer, off-host.
   Cheap now, impossible to retrofit after the first data loss. Required for B,
   non-negotiable for C — and a restore must actually be rehearsed once.
6. **Reproducible teardown.** `make server-reset` wipes the volume. Whatever
   equivalent exists in production must be impossible to run by accident.

### 4.4 Registration — two options

| | **Option 1: `_M`/`_F` suffix login** | **Option 2: small web signup** |
|---|---|---|
| How | `new_account: yes`; user logs in with `name_M` and any password, rAthena creates the account | A tiny service (Go, one handler) inserts into the `login` table |
| Client work | None — it already sends whatever the user typed | None for signup; a link on the login screen |
| Server work | One config line + `allowed_regs` / `time_allowed` throttle (already `1` per `10s`) | New service, its own deploy, its own abuse surface |
| Abuse | Trivially scriptable; throttled only by IP | CAPTCHA / email / invite code possible |
| Fits | **A, and probably B** | C |

**Recommend Option 1 for A and B.** It is one config line and no code, and
`allowed_regs: 1` / `time_allowed: 10` already rate-limits it. Its weakness —
scriptable account creation — does not matter until characters are worth
something. FluxCP was explicitly rejected in
[`rathena-setup.md`](rathena-setup.md) §2C for widening the attack surface;
that reasoning still holds.

One catch: with Option 1 the user's *first* login attempt with `name_M` gets
a rejection ("account created, log in again"). The client currently shows
that as a bare error string — see C5.

---

## 5. Client-side work

### 5.1 Blockers

| # | What | Where | Detail |
|---|---|---|---|
| C1 | Server address is config-file-only | [`ui2d_backend.go:789-793`](../../internal/game/ui/ui2d_backend.go#L789-L793), [`config.go:43-49`](../../internal/config/config.go#L43-L49) | The login screen has username and password fields and nothing else. A public user has to hand-edit YAML to find the server. Needs at minimum a server field, better a small server list (name + address) with the public server as the shipped default. |
| C2 | Password sits in `config.yaml` in cleartext | [`config.go:48`](../../internal/config/config.go#L48), [`config.example.yaml`](../../config.example.yaml) | Fine for `midgard-test`; wrong the moment a user types a password they might have used elsewhere. Either stop persisting it (remember username only) or use the OS keychain. Note `--password` on argv is already flagged as `ps`-visible in [TEST_ACCOUNTS.md](../TEST_ACCOUNTS.md) — same problem, wider blast radius on a public server. |
| C3 | Nothing recovers from a dropped connection | [`ingame.go:629-631`](../../internal/game/states/ingame.go#L629-L631) | A network error sets `ErrorMsg` and the loop keeps running against a dead socket. On a LAN this never happens; over WAN it happens on every Wi-Fi hiccup, sleep/wake and route flap. Minimum viable: detect the drop, tear the session down, return to the login screen with an honest message. Proper reconnect (re-auth and re-enter the map) is a feature of its own — [`ingame.go:1690`](../../internal/game/states/ingame.go#L1690) already notes as much. |
| C4 | No PIN code support | only lengths for 0x08B8/0x08B9 exist | Mitigated for now by turning the feature off server-side (S7). Becomes required if we ever want it on, which for C we probably do. |
| C5 | Login/char errors are raw strings | [`login.go:89`](../../internal/game/states/login.go#L89), [`:290`](../../internal/game/states/login.go#L290) | "IP blocked", "IP banned", `Connection failed: %v`. A public user needs to distinguish *server down*, *wrong password*, *banned*, *account created — log in again* (§4.4) and *already logged in* — the last one being the stranded-session case `make server-kick` exists for locally, which a public user cannot run. |
| C6 | Missing GRF is a log warning, not an error | [`game.go:145-151`](../../internal/game/game.go#L145-L151) | `AddArchive` failure only logs a warning and startup continues into a client that can render nothing. The single most likely first-run failure for a public user — they must be told, on screen, which file is missing and where to point it. |

### 5.2 WAN readiness — what is already right, and what is missing

Already right:

- **Movement prediction with server reconciliation** (`predictWalk` +
  `ZC_NOTIFY_PLAYERMOVE`, [`prediction_test.go`](../../internal/game/states/prediction_test.go))
  — exactly the shape WAN latency needs. Its behavior at 100–200 ms has never
  been observed, only at ~0 ms.
- **Off-thread socket reads** ([`client.go:184`](../../internal/network/client.go#L184)),
  so a slow link cannot stall the frame loop.
- **Keep-alive at 10 s** against rAthena's ~30 s timeout
  ([`ingame.go:184`](../../internal/game/states/ingame.go#L184)) — comfortable
  margin even with jitter.
- **Late-handler packet holding** (`holdForLateHandler`) — packets arriving
  before a state registers are not lost. Reordering under latency is where
  this earns its keep.
- Go sets `TCP_NODELAY` by default, so no Nagle stall to fix.

Missing:

- **No latency measurement at all.** `ShowPing` is declared in
  [`config.go:55`](../../internal/config/config.go#L55) and referenced nowhere
  else. `CZ_REQUEST_TIME` already goes out every 10 s; timing its reply gives
  an RTT almost for free, and we will want that number in every bug report
  from a remote tester.
- **`packetver` is effectively hardcoded** to the pinned 20211103. Fine while
  we control the server; the moment we advertise a public address someone will
  point a different client or an old build at it. A version handshake / clear
  mismatch error is cheap insurance.
- **No update path.** No version string, no build stamp, no "your client is
  out of date". For A, "download the new binary from Releases" is acceptable;
  for B we need at least a version check against the server.
- **No LICENSE file and no signed builds.** macOS Gatekeeper will refuse an
  unsigned, unnotarized binary downloaded from the internet — the first thing
  every macOS tester will hit. Budget for notarization or document the
  right-click-Open workaround.

---

## 6. Infrastructure

### 6.1 Sizing

For A and early B, with the CI-built image from §4.3:

| Resource | Need | Note |
|---|---|---|
| CPU | 2 vCPU | rAthena's map server is largely single-threaded |
| RAM | 2 GB (4 GB comfortable) | **The 8 GB requirement is a *build* requirement, not a runtime one** — it disappears once CI builds the image |
| Disk | 20 GB | Binaries + `db/` + `npc/` + MariaDB + logs |
| Network | Modest | RO is small packets at high frequency; latency matters far more than bandwidth |
| IPv4 | **1 static, dedicated** | Non-negotiable, see §3.1 |

### 6.2 Where

Any VPS provider giving a static IPv4 and a plain Linux box works — the
constraint in §3.1 rules out the interesting managed options, not the boring
ones. Two things do matter:

- **Geography.** Pick the region closest to where testers actually are, and
  decide it once we know (§9). A single region is correct for A/B; RO cannot
  be multi-homed behind anycast.
- **ToS.** Some providers' terms treat game servers, or copyrighted-IP fan
  projects, as prohibited. Read them before committing a domain.

Managed Kubernetes, autoscaling and multi-region are all inapplicable here —
noting so nobody spends a week discovering it.

### 6.3 Observability

Minimum for A: container logs shipped somewhere durable and rotated, plus an
uptime check that opens a TCP connection to 6900/6121/5121 and alerts. rAthena
writes its own logs (`log/login.log`, `log_char`, `log_db`) — decide retention,
since `log_login: yes` records IPs and that is personal data under GDPR if any
tester is in the EU.

---

## 7. Legal and licensing

This section is a flag for Boris, not advice — it needs a real answer before
anything is announced publicly.

1. **Ragnarok Online is Gravity's IP.** Private servers exist in large numbers
   and are broadly tolerated, but tolerated is not licensed. A *public,
   named, promoted* server is a materially different exposure than a client
   on one laptop. Non-commercial, no donations, no cash shop is the
   conventional risk-reduction posture and should be an explicit project rule.
2. **We cannot distribute `data.grf` / `rdata.grf`.** The current onboarding
   ("bring your own", [QUICKSTART §1](../QUICKSTART.md)) is the legally correct
   one and must stay — which makes C6 (a clear on-screen error for a missing
   GRF) an onboarding requirement, not a nicety.
3. **rAthena is GPL-3.0.** We are not modifying it today (config overrides
   only), so no obligations are triggered. If §3.2's TLS wrapper or any other
   patch happens, the fork must be published.
4. **This repo has no LICENSE file.** Under default copyright that means
   nobody may use, fork or redistribute the client — which contradicts
   publishing binaries and calling it an educational project. Pick a license
   before the first public release.
5. **Personal data.** Registration means storing logins, passwords and IPs.
   With EU testers this needs at minimum a stated retention period and a way
   to delete an account.

---

## 8. Staged plan

Estimates are working days of focused effort, not calendar time.

### Stage 0 — decide (½ d)
Answer §9. Without the target (A/B/C) and the region, later stages fork.

### Stage 1 — server deployable at all (2–3 d)
S1, S2, S3, S4, S5, S7, S8-partial. Vendor `char_conf.txt`/`map_conf.txt`/
`subnet_athena.conf` into `docker/rathena/conf/`, add
`docker-compose.public.yml`, move secrets to env.
**Done when:** a second machine on a different network reaches Prontera
against the public IP with a hand-seeded account.

### Stage 2 — client survives the internet (3–4 d)
C1 (server field + shipped default), C3 (drop detection → clean return to
login), C5 (real error messages), C6 (missing-GRF error). Plus RTT display,
which is small and pays for itself in every remote bug report.
**Done when:** a tester on Wi-Fi can lose and regain connectivity without
restarting the binary, and understands every failure the client shows them.

### Stage 3 — self-service (2–3 d)
S6 via `_M` registration (§4.4), C2 (stop persisting passwords), CI image
build → GHCR (§4.3), backups, uptime checks.
**Done when:** a stranger with a GRF can register, create a character and
play without us touching anything.

### Stage 4 — sustainable (ongoing)
Version handshake, signed/notarized builds, LICENSE, log retention policy,
`area_size` revisit, GM/moderation policy.

**Stages 1–3 ≈ 8–10 working days**, assuming the answers in §9 arrive first
and no rAthena surprises. That is the honest number for A; B adds Stage 4's
first three items.

---

## 9. Open questions for @avatar29A

1. **Which target — A, B or C?** Everything downstream branches here.
2. **Where are the testers, geographically?** Picks the region, and it is
   expensive to change later.
3. **Do we already have a host / static IPv4 / domain**, or does that need
   procuring?
4. **Is this announced anywhere public** (Reddit, rAthena forums, RO Discords),
   or invite-only? Changes the abuse posture and the legal exposure in §7.
5. **What is the answer on §7.1** — are we comfortable running a named public
   RO server, non-commercially?
6. **What license for the client?** Needed before any binary is published.
7. **Are characters persistent?** If yes, we owe backups and migrations from
   day one — that is the main cost difference between B and C.
8. **Who is on call** when the map server dies at 3am, and what is the
   acceptable answer ("it's back up tomorrow" is a perfectly good answer for
   A/B — it just has to be stated).

---

## 10. Sources

Repo-internal, all verified against the tree at `16a220b`:

- [`docker/rathena/docker-compose.yml`](../../docker/rathena/docker-compose.yml), [`README.md`](../../docker/rathena/README.md), [`setup.sh`](../../docker/rathena/setup.sh), [`conf/battle_conf.txt`](../../docker/rathena/conf/battle_conf.txt), [`seed/`](../../docker/rathena/seed/)
- rAthena at pin `5addd724`: `conf/login_athena.conf`, `conf/char_athena.conf`, `conf/map_athena.conf`, `conf/packet_athena.conf`, `conf/subnet_athena.conf`, `conf/web_athena.conf`, `tools/docker/asset/*.txt`
- [`internal/network/client.go`](../../internal/network/client.go), [`internal/network/packets/packets.go`](../../internal/network/packets/packets.go), [`internal/game/states/login.go`](../../internal/game/states/login.go), [`charselect.go`](../../internal/game/states/charselect.go), [`ingame.go`](../../internal/game/states/ingame.go), [`internal/config/config.go`](../../internal/config/config.go), [`internal/game/game.go`](../../internal/game/game.go)
- [`docs/research/rathena-setup.md`](rathena-setup.md), [`docs/QUICKSTART.md`](../QUICKSTART.md), [`docs/TEST_ACCOUNTS.md`](../TEST_ACCOUNTS.md)

External:

- [rAthena user guide — installation & configuration](https://rathena.github.io/user-guides/)
- [rAthena `conf/` reference](https://github.com/rathena/rathena/tree/master/conf)
