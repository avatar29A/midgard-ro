# Plan: one ordered command list for the 2D interface

## The problem

`internal/engine/ui2d` keeps its work in batches **by primitive type** and
flushes them in a fixed order: images, then solids, then text, then the
cursor's overlay. Submission order is preserved only *within* a type.

So "drawn later is on top" is false across types, by construction:

- a solid always covers an image, whichever was called first
- text always covers both

Every layering bug this project has hit is that one sentence:

| Symptom | Worked around with |
|---|---|
| The Status window's bitmap body was invisible | `WindowOptions.BitmapBody` — stop the frame painting a solid body |
| Skill icons vanished into the window body | `Renderer.FillImageLayer` — fill in the image pass instead |
| Item icons, the same | the same |
| HP/SP bars under the character drew over the map window | moved to `FillImageLayer` |
| The experience bars floated over every window | moved to `FillImageLayer` |
| The Basic Info panel's labels drew over the map | could not be worked around — forced `Renderer.Flush` |

The last one is the tell. Text is drawn after everything, so no amount of
reordering calls could put a window over a label. `Flush` splits the frame
into two layers, which fixes the HUD against the windows and nothing else.

**What is still wrong after `Flush`:** inside a layer, type order still rules.
Two overlapping windows have no z-order between them — all their images draw,
then all their solids. Drag the Inventory over the Map and the Map's bitmap
will cover a solid the Inventory drew, whichever was drawn later. Adding a new
window also means remembering it belongs after the `Flush` call, and nothing
enforces that.

## The fix

One command list. Every draw appends to a single slice, keeping its
submission index. The flush walks it in order and switches shader, texture or
vertex buffer when the next command needs a different one, coalescing runs of
the same kind exactly as the per-type batches already do.

Then the rule is: **drawn later is on top. Always.** No exceptions to
remember, no layer to opt into.

### What this deletes

- `Renderer.FillImageLayer` and its one-pixel white texture (7 call sites)
- `WindowOptions.BitmapBody` and the `WindowState` field behind it (10 refs)
- `Renderer.Flush` and the layer split in `RenderInGameUI`
- the separate `solidVertices` / `imageVertices` / `textVertices` batches

Three special cases replaced by one rule, so it is a net simplification rather
than new machinery.

### What it keeps

The overlay layer stays. The cursor genuinely belongs above everything,
including whatever is drawn after it, and that is a statement about the
cursor rather than a workaround for ordering.

## Cost and risk

**Call sites touched:** none directly. The public API — `DrawRect`,
`DrawImage`, `DrawText` and the rest — does not change; only where the
vertices go. That is 227 call sites that keep working, and the three
workarounds above that get removed.

**Performance.** Worst case is one draw call per primitive if types alternate
every call. A UI frame is a few hundred quads, and real screens draw in runs —
a window's frame images together, then its solids, then its text — so the
coalescing keeps it close to what it is now. This is not a hot path; the map
and sprites are.

**The real risk is visual regression**, and it is broad: this changes how
every screen composites. Login, character select, loading, the in-game HUD and
each of the four windows need looking at before it is trusted.

## Steps

1. **Record the ground truth first.** Screenshots of the login screen,
   character select, the HUD, and each window — Status, Skill Tree, Inventory,
   Map, ESC menu, Sound Configuration — before touching anything. Without
   these there is nothing to compare against, which is the same mistake that
   made the backdrop look broken.
2. **Add the command list beside the batches**, not instead of them: a slice
   of `{kind, vertStart, vertCount, texture}` fed by the existing append
   helpers, with the vertices going into one buffer per shader as they do now.
3. **Switch the flush to walk it**, coalescing consecutive commands that share
   a shader and texture.
4. **Compare against step 1.** Every screen. Differences here are the point of
   the exercise, so look at them rather than through them.
5. **Delete the workarounds** — `FillImageLayer`, `BitmapBody`, `Flush` — and
   the calls to them, and compare again. The windows whose bodies were bitmaps
   should now paint their bodies normally.
6. **Write the rule down** in `docs/ENGINE_FEATURES.md`, replacing the section
   that currently explains the three workarounds.

## Done when

- Drawn later is on top, for every combination of image, solid and text.
- Two overlapping windows stack by the order they were drawn.
- `FillImageLayer`, `BitmapBody` and `Flush` are gone, and nothing needs them.
- Every screen listed in step 1 looks the same as it did, except where the old
  order was the bug.

## Depends on

PR #97, which adds `Flush`. This branch removes it again, so it should land
after that one rather than racing it.
