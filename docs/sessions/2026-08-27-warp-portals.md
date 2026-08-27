# 2026-08-27 — Warp portals: moving across locations

Issue #91, branch `feature/warp-portals`, worktree `midgard-ro-warp`.

Walk through Prontera's gates and doors: the blue portal on every warp, the
door cursor over it, the loading screen while the next map loads, the room
with the original's indoor camera. What follows is what the plan got wrong
and what only running it found.

## What the plan got wrong

**The loader belongs to `InGameState`, not `LoadingState`.** The plan had
`LoadingState` drive the phases. But landmine 5 wanted the in-game state —
its packet handlers, stats, dialog, camera — to survive a warp, and that only
works if the state that owns the map also owns loading it. `LoadingState` is
now the handshake only; `InGameState.beginMapLoad`/`finishMapLoad` do the
rest, for the first map and every warp alike.

**The same-map rule is "same map and not yet loaded", not "and loading".** The
held login-time `0x0091` is delivered the instant its handler registers, which
was *before* `beginMapLoad` had run: it read as a local teleport and `0x007D`
went out with no map. The load now starts before the handlers exist.

**A 12 ms per-frame budget was too small.** Each frame carries ~23 ms of its
own, so 12 ms of loading per frame made a 1243 ms load take 1444 ms. At 24 ms
it takes 1307 — and the bar still moves 25 times a second.

## What only testing found

**Screenshots were distorting the thing they measured.** Every unattended
capture encoded a 2560×1440 PNG on the render thread, most of a second each;
a 1.3 s load measured as 7.3 s under a 300 ms cadence and *still* 7.3 s at
1 s. The encode now runs on a worker goroutine at best-speed compression;
the same run measures 1.9 s. Every earlier unattended number in this project
that was taken while capturing is suspect by the same amount.

**Added light over a sunlit doorway is white.** The portal's first capture was
a white column. The texture is blue; the blend is additive; the doorway
behind it is bright. Tinting `(0.55, 0.8, 1.0)` at nine tenths made it the
original's blue.

**`0x01D7` is mis-framed on every map entry — and always was.** Right after
`0x007D` the server sends `ZC_SPRITE_CHANGE2`; our length table has 11 bytes,
this PACKETVER sends 15, and the client logs a resync and recovers. It is in
every run back to `main`. Not fixed here; it is a `tools/packetlen` entry.

**The void is sky, not black, once the water is gone.** Per-cell water removed
the sea between `prt_in`'s rooms and exposed the scene's sky-blue clear.
Indoor maps now clear to black, as the original does.

**The indoor rule was reversed on the first interactive run.** The plan
followed the original: rotation off indoors, zoom free. Boris zoomed out to
377 in `prt_in` and saw every room of the building complex floating in
black — the "seeing aside" the feature was meant to stop, and it is the
zoom that causes it, not the turning. Indoors now hold the zoom at the
default distance (snapping there on entry) and allow rotation.

**The portal was the wrong shape, and testing it found two rendering faults
underneath.** The plan built a tall spinning tube from roBrowser's
`Cylinder.js`. That primitive is the original's *casting circle*, not its
warp: the reference frames showed a flat ringed disc, and Boris's capture of
the original confirmed it — flat, four cells across, concentric rings with a
swirl, four-pointed sparkles around them. Rebuilding it turned up:

- **Added light cannot hold a pattern on a pale floor.** On Prontera's
  pavement every value above about a third clipped to white and the rings
  became one smear. Blending over the floor reads on both dark stone and
  pale pavement.
- **Ordinary alpha blending punched a hole in the scene.** The scene is drawn
  into an offscreen buffer that the interface composites *by its alpha*, and
  everything else in it writes alpha 1. `ONE_MINUS_SRC_ALPHA` wrote the
  portal's shape into that alpha channel and the interface showed through it
  as hard cyan and black bands. `BlendFuncSeparate(SRC_ALPHA,
  ONE_MINUS_SRC_ALPHA, ZERO, ONE)` fixes it. The additive version had hidden
  the fault by saturating alpha to 1 — every future effect that blends into
  the scene needs the separate alpha.

Tuning an effect through the client is slow — a build, a login and a map load
per look. `PORTAL_DUMP=<file> go test ./internal/engine/scene/ -run
TestDumpPortalTexture` writes the generated texture composited over a pale and
a dark floor, which is where the pattern was actually settled.

**Clicking a warp was a dead click, everywhere the warp sits in scenery.**
The cursor turned into a door and nothing happened. The trace showed the
click registering and the walk going out; what it did not show was a reply,
because the destination was `prtf004` at `prt_fild08 170,378` — inside the
arch of Prontera's wall, where the GAT says nobody can stand. An unpathable
walk is exactly what rAthena answers with silence, which the movement work in
PR #78 had already learned once. The trigger box is the way out: `xs,ys`
covers the cells around a warp, so standing beside one is standing in it.
Clicks now aim at the nearest walkable cell instead of the warp's own.

Worth remembering as a shape of bug: **the client asked for something
impossible and the server said nothing**, so nothing in the log looked wrong.
Checking the GAT against the destination is what found it in a minute.

## What no reference client had

Nothing in the archive describes the warp portal: the original draws
`EF_WARPZONE2` itself, korangar loads a sprite that does not exist in kRO
data, roBrowser has `// TODO: Warp STR file`. The client's own `jobname.lub`
names `1_ETC_01` for class 45, which is why we drew a white blob in every
doorway. The portal is rebuilt from roBrowser's `Cylinder.js` — the port of
the original's primitive that was never wired up — and `ring_blue.tga`.

## Numbers

| Map | Load | Frames |
|-----|-----:|-------:|
| prontera | 1286–1307 ms (1243 frozen) | 32 |
| prt_in | 344–377 ms | ~12 |
| door warp, step to standing inside | ~0.6 s | |

## Left for later

- `0x01D7` length in `lengths.go` (above).
- `0x0AC7` map-server change is decoded and logged, not followed.
- The portal's proportions are by eye against the real-client frames.
- The zoom range outdoors is still 100–800 against the original's ~1.75×
  the default; a global cap is a separate decision.
- Async (goroutine) parsing: the loader is phased so it could move, but the
  numbers did not need it.
- Two captures within one second share a filename (`screenshot-HHMMSS.png`);
  the second overwrites the first.
- `Entities: 0` on the F3 overlay was never wired — it predates this feature.
