# Feature: <Title>

**Branch:** `feature/<slug>` · **Issue:** _(filled after creation)_ · **Parent:** #49 (MVP scope) <!-- + RFC #N if one applies -->
**Status:** Planned · **Created:** YYYY-MM-DD

## Goal

One paragraph. What the player sees or does once this is done, in Prontera / prt_fild terms. Name the six-feature MVP item it advances (walking, music, UI, mobs, NPCs, combat).

## Reference (original client)

![ref-01 — <caption: what screen, which client/version, where it came from>](./ref-01-<what>.png)

1. **<Element name used in the plan>** — what it is, where it sits, how it behaves
2. **<Element>** — …
3. …

<!-- Repeat per image. UI chrome: also link the extracted GRF bitmaps (./grf-<name>.png). -->

**Current state (ours):** ![current — <caption>](./current-<what>.png) <!-- delete if nothing renders yet -->

## What exists today

| Area | What exists | Status |
|------|-------------|--------|
| `internal/…` | … (`file.go:NN`) | ✅ done / 🟡 stub / ❌ missing |
| `cmd/grfbrowser` | has/lacks a viewer for … — uses `internal/engine/…` → **reuse this** | |
| Tests | … | |
| Docs | ADR-0xx, docs/plans/…, docs/research/… | |

**In flight** (touches the same files — build on it, don't collide):

| PR / branch | What | Overlap |
|-------------|------|---------|
| #NN `feature/…` | … | … |

## Reference implementations

| Source | Where | Approach (one paragraph) |
|--------|-------|--------------------------|
| korangar | `korangar/src/…` | … |
| roBrowser | `src/…` | … |
| midgarts | … | … |

<!-- Where they disagree, say so and which one we follow, and why. -->

## Assets

| Asset | GRF path | Exists? (`grftool search`) |
|-------|----------|-----------------------------|
| … | `data/texture/유저인터페이스/…` | ✅ / ❌ |

## Protocol <!-- delete if no packets -->

| Packet | ID (our PACKETVER) | Direction | Source |
|--------|--------------------|-----------|--------|
| `ZC_…` | `0x0000` | S→C | `docker/rathena/build/rathena/src/map/clif_packetdb.hpp:NN` |

## Step 0 — Prerequisites & tooling

### 0a. Refactoring (only what this feature needs)

- [ ] `file.go:NN` — why it blocks the feature → minimal change
- [ ] ADR-0xx — _(only if architectural)_

### 0b. Debug tooling & tests

- [ ] Trace channel `<name>` in `internal/trace` — events: `<name>.request`, `<name>.ack`, …
- [ ] F3 overlay fields: …
- [ ] Screenshot scenario: `go run ./cmd/client --config config.yaml --trace=<ch> --screenshot-after 8s` → `latest.png` must show …
- [ ] grfbrowser preview for … _(if needed)_
- [ ] Logs: … says so at warn when missing
- [ ] Tests: `<pkg>/<file>_test.go` (table-driven), packet round-trips
- [ ] Use cases: UC-nnn, UC-nnn (in `qa/use-cases/`)

## Steps

### Step 1 — <the increment, one sentence>
- **Changes:** `internal/…`, `internal/…`
- **Done when:** …
- **Proved by:** `go test ./internal/…` / `--trace=<ch>` shows … / screenshot scenario / UC-nnn
- **Reference:** ref-01 ①②

### Step 2 — …

### Step N — Docs
- [ ] `docs/ENGINE_FEATURES.md` _(if a package was added)_
- [ ] ADR-0xx → Accepted _(if drafted)_
- [ ] Session log `docs/sessions/YYYY-MM-DD-<slug>.md`

All steps land on `feature/<slug>` in one PR (one or a few commits per step, in order). The PR closes the issue.

## Done when (feature)

- …
- …

## Out of scope

- …

## Open questions

1. …

## Investigation notes <!-- optional appendix: anything worth keeping that didn't fit above -->

…

## Revision log

- YYYY-MM-DD — created
