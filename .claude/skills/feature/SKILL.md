---
name: feature
description: Prepare a new Midgard RO feature end-to-end before any code is written — investigate the codebase, grfbrowser and the reference clients; gather screenshots of the original client; plan prerequisite refactoring, debug tooling and step-by-step increments; write docs and QA use cases; open the GitHub issue. Also revises an existing feature issue from Boris's review comments.
argument-hint: "<topic> | revise <issue#>"
allowed-tools: Bash, Read, Write, Edit, Glob, Grep, Agent, WebFetch, WebSearch
---

You are preparing a feature for Midgard RO. The deliverable is a GitHub issue that Boris can review — description, reference screenshots, and a step-by-step plan — backed by `docs/features/<slug>/README.md` on a fresh `feature/<slug>` branch. You do not implement anything here.

Input: `$ARGUMENTS`

- `<topic>` → **plan mode** (phases 0–7 below).
- `revise <issue#>` (or `#<issue#>`) → **revise mode** (last section).

## Principles (Boris's rules — do not relax them)

1. **Investigate before proposing.** Know what already exists: our code, `cmd/grfbrowser`, docs/ADRs, PRs in flight, korangar/roBrowser/midgarts, rAthena. Never plan a third copy of a pipeline that grfbrowser and the client already share.
2. **Reference from the original game.** Real kRO/iRO screenshots first (web, YouTube stills); roBrowser and korangar only as fallback. Enough that Boris can confirm we are talking about the same UI elements and behaviour — every screenshot gets a legend naming the elements.
3. **Debug-first.** Tooling and tests are Step 0 and every later step says how it is *proved* (trace, test, screenshot, use case). Look at what tooling found in PR #78: the three bugs that broke movement were all outside the movement code.
4. **Prerequisite refactoring is Step 0 too** — inside the feature, never a separate gate. Only what this feature needs.
5. **One feature = one issue = one branch = one PR.** Steps are commit-sized checkpoints inside that PR. Each step is a real increment — visual or logical — with a proof. Non-visual steps are fine; scaffolding-only steps are not (merge them into the step that makes them observable).
6. **Docs are part of the plan**, not an afterthought: the feature README and the QA use cases are written now; an ADR only when Step 0 contains an architectural decision.
7. **Boris reviews on GitHub.** Create the issue without asking; he comments; `/feature revise N` applies the comments. Don't blindly agree — if a comment conflicts with what you found, say why in the thread and ask.

## Phase 0 — Setup

- Slug: kebab-case, 2–4 words (`npc-dialog`, `hud-hp-sp-bars`). Title: `Feature: <short title>`.
- `git status --porcelain | grep -v '^?? '` must be empty (untracked build binaries `client`, `midgard`, `grfbrowser`, `grftool` are fine). If dirty, stop and say so — do not stash.
- No collision: `git branch -a | grep feature/<slug>`; `gh issue list --state all --search "<title words>"`. If a matching feature issue exists, switch to revise mode or pick a different slug.
- GRF path: `grep -A3 grf_paths config.yaml`; fall back to `/Users/borisglebov/git/RagnarokClients/data/data.grf`. Set `GRF=<path>` for the commands below.
- `SCRATCH` = the session scratchpad directory from your system prompt (temp files, generated issue body). Never use the repo or `/tmp` for that.
- Read `reference/tooling.md` and `reference/sources.md` in this skill directory before continuing.

## Phase 1 — Investigate

Fan out with `Explore` agents where the sweep is wide (code, reference clients), then read the decisive files yourself. Record findings with `file:line` — they go into the README verbatim.

1. **Project knowledge.** `CLAUDE.md`, `docs/ENGINE_FEATURES.md`, `grep -il '<keywords>' docs/adr/*.md docs/plans/*.md docs/research/*.md docs/investigations/*.md`, the two latest `docs/sessions/*.md`, the PRD section (`docs/prd/PRD.md` §4), RFC #49 (`gh issue view 49`) for what the MVP includes and excludes.
2. **Our code.** Which packages own this subsystem (`internal/engine/*`, `internal/game/*`, `internal/network/packets`, `pkg/*`)? What exists, what is stubbed, what is tested? Which `UIBackend` methods / game states / packet handlers are involved?
3. **In flight.** `gh issue list --state all --search "<kw>"`, `gh pr list --state all --search "<kw>"`, `git worktree list`, `gh pr list --json headRefName,title`. Anything touching the same files goes in the README's "In flight" table so the plan builds on it instead of colliding.
4. **grfbrowser / grftool.** Does `cmd/grfbrowser` already preview or parse this (`preview_sprite.go`, `preview_image.go`, `preview_model.go`, `preview_map.go`, `preview_audio.go`, `map_viewer.go`, `model_viewer.go`)? Which shared package does it call? That package is what the client reuses. If there is no viewer for the asset the feature depends on, adding one to grfbrowser is a Step 0 tooling item (that is how RSM transforms were debugged — see `docs/sessions/2026-01-15-rsm-transform-debugging.md`).
5. **Assets.** Confirm every GRF file the feature needs actually exists: `go run ./cmd/grftool search "$GRF" <ascii-fragment>`. Record exact paths. (Lesson from e76cf49: the login screen asked for two files that were not in the archive and rendered on black.) For UI work also extract the bitmaps — they are the most precise visual reference (see `reference/sources.md`).
6. **Reference implementations.** korangar first (`/Users/borisglebov/git/RagnarokClients/korangar/` — `korangar/src`, `korangar-interface`, `ragnarok-packets`, `ragnarok-formats`), then roBrowser (`roBrowser/src`), then midgarts. For each: the files and a one-paragraph description of the approach. Note where they disagree with each other.
7. **Server side** (only if packets are involved). rAthena is checked out at `docker/rathena/build/rathena/src/map/` — `clif.cpp` for handlers, `clif_packetdb.hpp` for IDs per PACKETVER, `packets.hpp` for structs. Lengths come from `tools/packetlen/gen.py`. Cite the source line for every packet ID; never guess one.

## Phase 2 — Visual reference

Goal: Boris opens the issue and sees the original client showing exactly the elements/behaviour we mean.

- Source priority and capture recipes are in `reference/sources.md`: (1) real kRO/iRO via web search / YouTube stills, (2) roBrowser, (3) korangar, plus GRF bitmaps for UI chrome.
- Save to `docs/features/<slug>/ref-NN-<what>.png`. Downscale to ≤1280 px wide (`sips -Z 1280 <file>`), keep each ≤ 500 KB. Never put anything under `data/` (gitignored).
- **Current state** (only if the client already renders something related): `make server-up`, then `go run ./cmd/client --config config.yaml --screenshot-after 8s`; copy `data/Screenshots/latest.png` to `docs/features/<slug>/current-<what>.png`. Kill the client afterwards (`pkill -f "cmd/client|/midgard$"`).
- **Legend** for every image: one caption line, then a numbered list of the elements it shows and the names the plan uses for them.
- If no reference can be found, say so in the issue and list it under Open questions — do not substitute a guess.

## Phase 3 — Prerequisites (Step 0a)

Only refactoring this feature needs: a layer-rule violation in its path, a duplicated pipeline it would extend, a function it would grow past 50 lines, a missing seam needed to test it. For each: `file:line`, why it blocks the feature, the minimal change. If the change is architectural (crosses a layer in CLAUDE.md, changes an interface used by more than one package), it is still Step 0 — add "write ADR-0xx" to Step 0 and draft it in Phase 6.

## Phase 4 — Debug tooling & tests (Step 0b)

`reference/tooling.md` lists what exists. Decide, per subsystem the feature touches:

- **Trace channel** (`internal/trace`): reuse `move`/`pick`/`net`/`render` or add one named after the subsystem. List the events (`<channel>.<verb>`) worth emitting.
- **F3 overlay fields** (`internal/game/ui/debug_overlay.go` + `internal/game/debug_fields.go`): which live values make the feature debuggable from a screenshot.
- **Screenshot scenario**: the exact command (`--screenshot-after`, `--trace`, map, account) and what must be visible in `latest.png` when the step is done.
- **grfbrowser preview** if the feature depends on an asset without a viewer.
- **Logs**: what is logged at info/warn — a missing asset must say so, never a swallowed error.
- **Tests**: table-driven unit tests in the owning package; packet round-trips with known bytes (`internal/network/packets/packets_test.go` pattern); QA use cases `qa/use-cases/UC-<nnn>-<slug>.md` (number ranges in `qa/README.md`; take the next free number in the right range).
- **Not built yet — don't assume**: the JSON-RPC debug API (ADR-005) and the visual-diff pipeline (ADR-016). If the feature genuinely needs them, that is Step 0 work and must be sized honestly.

## Phase 5 — Plan the steps

Step 0 = prerequisites + tooling. Then Steps 1..N, each:

```
### Step N — <the increment, one sentence>
- **Changes:** packages / files
- **Done when:** what a person can observe (screen, log line, packet, test)
- **Proved by:** test name / `--trace=` recipe / screenshot scenario / UC-nnn
- **Reference:** ref-NN (visual steps only)
```

Rules: order by dependency; typically 4–8 steps; every step observable or verifiable; last step is **Docs** (`docs/ENGINE_FEATURES.md` if a package was added, ADR status → Accepted, session log). All steps land on `feature/<slug>` in one PR; one or a few commits per step, in step order; PR body links the issue and closes it.

Finish with **Done when (feature)** — the acceptance list Boris tests — **Out of scope**, and **Open questions** (numbered; only things you genuinely could not resolve).

## Phase 6 — Docs

- `docs/features/<slug>/README.md` from `templates/feature-readme.md`. This is the canonical record; the issue body is generated from it. Use relative image links (`./ref-01-login.png`) so the URL swap in Phase 7 works.
- `qa/use-cases/UC-<nnn>-<slug>.md`, one per acceptance scenario, format from `qa/README.md`. Link them from the plan's "Proved by" lines.
- ADR draft (`docs/adr/ADR-0xx-<slug>.md`, Status: Proposed) only when Step 0 has an architectural decision. Next number: `ls docs/adr | sort | tail -1`.
- Do not edit `ENGINE_FEATURES.md`, the PRD or session logs now — those are the plan's Docs step.

## Phase 7 — Publish

```bash
git checkout -b feature/<slug> main
git add docs/features/<slug> qa/use-cases/UC-*-<slug>.md            # + docs/adr/ADR-0xx-<slug>.md if drafted
git commit -m "docs(feature): <slug> — investigation, references and plan"
git push -u origin feature/<slug>
SHA=$(git rev-parse HEAD)
```

Issue body = README with image links rewritten to SHA-pinned raw URLs (pinned so the links survive branch deletion; the repo merges with merge commits, so the SHA stays reachable from `main`):

```bash
sed "s#](\./#](https://raw.githubusercontent.com/avatar29A/midgard-ro/$SHA/docs/features/<slug>/#g" docs/features/<slug>/README.md > "$SCRATCH/issue.md"
gh label create feature --description "Feature: investigation + step-by-step plan" --color "1d76db" 2>/dev/null || true
gh issue create --title "Feature: <title>" --label feature --body-file "$SCRATCH/issue.md"
```

Then back-link: put `**Issue:** #N` in the README header, commit `docs(feature): link #N`, push. Report the issue URL, the branch, the step list in one line each, and the open questions. Stay on `feature/<slug>` — implementation continues there.

Never create the issue before the docs are pushed (the images would be broken). Never commit `client`, `midgard`, `grfbrowser`, `grftool` binaries or anything under `data/`.

## Revise mode — `/feature revise <issue#>`

1. `gh issue view N --json title,body,comments,labels`. The slug is in the README link in the body; `git checkout feature/<slug>` (pull first).
2. Read every comment after our last "Applied:" comment (or all, if none). Comments are Boris's review; each one is either a change to make or a question to answer.
3. Apply changes to `docs/features/<slug>/README.md` (and UCs / ADR if affected). Where a request contradicts the investigation, do not apply it silently — reply with the evidence (`file:line`, packet source, screenshot) and ask.
4. Regenerate the issue body with the same `sed` as Phase 7 (new SHA after commit+push), `gh issue edit N --body-file`, and append a line to the README's **Revision log** (`YYYY-MM-DD — <what changed> (per comment)`).
5. `gh issue comment N --body` — "Applied: …" as a bullet list, then any questions.
