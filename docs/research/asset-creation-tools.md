# Making our own graphics in RO's style — tool survey (September 2026)

Every sprite, window skin and texture the client draws today comes out of
Gravity's `data.grf`. This note answers one question: **if we wanted to make
new art that sits next to the original without looking foreign, what would we
use in 2026, and what part of that can Claude do?**

Status: research only. Nothing here is implemented; §6 proposes what would be.

---

## 0. Verdict in five lines

1. **Claude does not draw.** No Claude model or product generates raster
   images; that is a deliberate Anthropic choice. Claude Design produces
   layouts as HTML, not pictures. What Claude *does* do is everything around
   the picture: drive the generators below over MCP, write the packers our
   formats need, and act as a vision-based reviewer against the originals.
2. **Two generators fit RO's sprites specifically**: PixelLab (8-direction
   rotation + skeleton-driven animation, has an official MCP server) and
   Retro Diffusion (reference-image style lock, palette constraint, an
   `8_dir_rotation` and walking-animation workflow, MCP server and API).
   General models (Gemini, GPT-image, Midjourney, Flux) produce
   "pixel-looking" illustrations, not grid-aligned pixel art.
3. **The RO-native editors (ActEditor, GRF Editor) are Windows-only.** On this
   Mac that means Wine/CrossOver or a VM. The alternative is to write the two
   small packers we lack (`PNG → SPR/ACT`, and a loose-file asset source) in
   Go, which Claude can do with round-trip tests against our parsers.
4. **The measured canvas is small.** A player body is at most 68×80 px, a head
   30×28, a Poring 85×66, an item 14×16 — all inside the 128×128 tier every
   generator offers.
5. **Legal caveat.** Feeding Gravity's sprites to a generator as style
   references, or training a LoRA on them, is exactly the grey area
   `public-deployment.md` §7 already flags. Fine for a private test; decide
   before anything ships.

---

## 1. What "RO style" is, technically

Measured with our own parser (`pkg/formats.ParseSPR`) on `data.grf`:

| Sprite | Frames | Max frame | Average frame | Palette |
|---|---|---|---|---|
| Novice body, male (`인간족\몸통\남\초보자_남`) | 110 | 68×80 | 42×66 | 256 |
| Head style 1, male (`인간족\머리통\남\1_남`) | 15 | 30×28 | 26×25 | 256 |
| Poring (`몬스터\poring`) | 49 | 85×66 | 40×33 | 256 |
| Kafra NPC (`npc\4_f_kafra1`) | 8 | 47×95 | 42×89 | 256 |
| Apple item (`아이템\사과`) | 1 | 14×16 | 14×16 | 256 |

What that implies for any tool we pick:

- **Indexed color.** Every SPR carries its own 256-color palette; index 0 is
  transparent. Hair and clothes recolors are *palette swaps*, not new sprites
  (`data\palette\머리\머리<style>_<sex>_<color>.pal`, 1024 raw RGBA bytes —
  see `pkg/formats.ParsePAL`). A generator that emits 24-bit PNG needs a
  quantize step, and one that respects an input palette saves that step.
- **Eight directions, fixed action sets.** A player body sprite is 13–14
  actions × 8 directions (`pkg/formats/action_names.go`: idle, walk, sit,
  pick up, standby, attack ×3, damage, die, dead, cast, ready, freeze); a
  monster is 5–8 actions × 8. Frames, anchors and per-frame delays live in the
  `.act`. So "a new monster" is roughly 40–60 distinct frames, "a new player
  body" about 110.
- **Layered characters.** A character on screen is body + head + headgear
  layers composed at ACT anchor points. New *classes* mean new bodies; new
  *looks* mean headgear or palettes. Heads and bodies must agree on the anchor
  convention or they float apart.
- **Hand-drawn, not palette-limited.** The originals read as pixel art but
  use the full 256 colors with soft shading and a dark outline; contemporary
  observers judged them hand-drawn and cleaned in an editor rather than
  rendered from 3D. Any generator will need its "chunky 16-color" bias
  prompted away.
- **UI is bitmaps, not sprites.** Window chrome and screens are 8-bit BMP
  with magenta `#FF00FF` as the key (or 32-bit TGA where alpha is needed);
  `ro-ui-system.md` §3 has the sizes. `bg_makebg.bmp` at 794×422 is typical.
  Nothing about these needs a pixel-art generator: they are ordinary raster
  UI, and vector-to-bitmap gets there.

## 2. What Claude can and cannot do here

**Cannot:** generate or edit pixels itself. Not in the API, not in Claude
Code, not in Claude Design (which builds designs out of code). There is no
image output content block in the Messages API.

**Can, and this is most of the pipeline:**

| Role | How |
|---|---|
| Drive a generator | PixelLab and Retro Diffusion both ship MCP servers; `claude mcp add` and the tools appear in this session. Community Aseprite MCP servers run Aseprite in batch mode with generated Lua. |
| Write the packers we lack | `pkg/formats` parses SPR/ACT/PAL but writes none of them; `pkg/grf` reads only. A `PNG sheet → SPR+ACT` packer, a `.pal` writer and a loose-file asset source are each a few hundred lines with round-trip tests against the existing parsers. |
| Review against the original | Claude reads images. "Here is Gravity's Poring at 4×, here is ours, list the differences" is a real QA step, and `grfbrowser` already previews SPR/ACT so both sides render the same way. |
| Make UI chrome directly | Window frames, buttons and 9-slices are geometric. Claude writes the SVG or a Go program that emits the BMP, then the magenta key and 8-bit conversion are one ImageMagick line (`ro-ui-system.md` §3.1). Claude Design is the right tool for *mocking* a screen before pixels exist. |
| Palette work | Quantize to 256 with index 0 reserved, derive hair-color variants by hue rotation of a base palette, verify every `.pal` is 1024 bytes. Pure math, well suited. |

## 3. Generators

| Tool | What it does that RO needs | Claude integration | Cost (as advertised) |
|---|---|---|---|
| **PixelLab** | `rotate`: one south-facing reference → 8 directions, with high/low top-down view choice; `animate-with-skeleton`: template skeletons for walk/idle/etc., editable; canvases 16–128 (rotate) and up to 256 (animation); forced palettes on the `pixflux` generator, reference images on `pro`. | Official MCP (`pixellab-code/pixellab-mcp`, HTTP transport, bearer token); also a Python client and REST API. | Subscription, roughly $12–50/month by tier; commercial use allowed, "do not train new models with the images"; skeleton animation needs Tier 1. |
| **Retro Diffusion** | RD Pro takes up to 9 reference images and an `input_palette`; workflows `rd_animation__8_dir_rotation`, `rd_advanced_animation__walking`, battle sprites, tilesets; `return_spritesheet` gives a PNG grid; a free `fix_pixel_art` re-snaps output to the pixel grid; custom user styles. Sizes 16–384 by style. | Official MCP at `mcp.retrodiffusion.ai` (`claude mcp add --transport http …`), REST API with `check_cost: true` for a free quote, plus an Aseprite extension. | Per image: Fast ≈ $0.015–0.03, Plus ≈ $0.025, Pro $0.18, animations $0.07–0.25. Aseprite extension $65 one-time (Lite $20). |
| **Scenario** | Train a LoRA on 10–30 of your own images and generate in that locked style. No animation, no rotation. | Web app and API; no MCP found. | From $15/month; custom training $45/month. |
| **Gemini "Nano Banana 2"** | Up to 14 reference images for style; good at *concepts*. Output is pixel-style illustration, not grid-aligned; no alpha (chroma-key green background + white outline + HSV keying is the documented workaround). | Via Google's API; Claude can orchestrate but there is no MCP we found. | Per image, Google pricing. |
| GPT-image, Midjourney, Flux | Same class as Gemini for this purpose: attractive retro *look*, uneven pixel grid, too many colors. Usable for a mood board, not for frames. | — | — |
| Ludo, Layer.ai, AutoSprite, Sprite-AI | Broader "game asset" suites. AutoSprite animates a sprite you bring; Layer is batch production for mobile studios. None adds anything over PixelLab/RD for 8-direction indexed sprites. | — | $8–60/month |

Two things every comparison agreed on: purpose-built pixel tools keep the grid
and palette; general models do not, and downsampling their output looks like
downsampled output. And every serious workflow ends in a pixel editor for
cleanup — nobody ships generator frames untouched.

## 4. Editors and RO-native tooling

| Tool | Role | Runs here? |
|---|---|---|
| **Aseprite** | Indexed-color editor with animation timeline, palette import/export, Lua scripting and `--batch`. The cleanup and anchor-alignment stage. Retro Diffusion plugs in as an extension. Several community **Aseprite MCP servers** (e.g. `ayigityol/aseprite-mcp`, 43 tools) let Claude script it. | macOS, paid or build from source. |
| **ActEditor** (Tokeiburu) | The community's SPR/ACT editor: import images, anchors, per-frame delay, tweening, palette editor. | Windows .NET/WPF. Wine/CrossOver or a VM on this Mac. |
| **GRF Editor** (Tokeiburu) | Builds and patches GRFs; also a scriptable C# library (`GrfHolder.Commands.AddFile`). | Windows. Same caveat. |
| **zrenderer** (zhad3) | Renders SPR/ACT to PNG server-side. Good for producing reference sheets of originals to feed a generator or a reviewer. | Docker image. |
| **Blender + 8-Directions Render plugin** (v2, Feb 2026, Blender 3.6+), **Blender To Pixels** (free), **Pixelize** add-on | The 3D route: model once, render 8 directions × animation, quantize. Consistent across directions by construction; the RO look then needs hand cleanup. **PixZels** does the same from orthographic drawings without Blender. | macOS. |
| **rAthena wiki** (Spriting, Acts), **Spriters Resource** | Written conventions and ripped sheets, for reference only. | — |

## 5. What our repo is missing

| Gap | Where | Why it matters |
|---|---|---|
| No SPR/ACT/PAL **writers** | `pkg/formats` | Anything generated has to become SPR+ACT to be drawn. Round-trip tests against the existing parsers make this safe. |
| No GRF **writer**, no loose-file source | `pkg/grf`, `internal/assets` | `Manager.Load` searches `grf_paths` in reverse, so a `custom.grf` listed last would override cleanly — but we cannot make one. The original client reads a loose `data\` folder before its GRFs; adding that source is smaller than a GRF writer and is what a Mac workflow wants. |
| No palette-aware quantizer | — | Generators emit 24-bit PNG; SPR wants 256 indexed with index 0 transparent, and hair wants a *base* palette whose hue variants become the `.pal` files. |
| Preview exists | `cmd/grfbrowser` | Already renders SPR/ACT and bitmaps, so a packed result can be checked before the client loads it. Keep using it; do not build a second viewer. |

## 6. Pipelines that would actually work

**A. New monster or NPC (the realistic first target).**
Reference sheet of an original (zrenderer or `grfbrowser` export) → Retro
Diffusion RD Pro with 3–9 originals as `reference_images` and an
`input_palette` derived from them, or PixelLab `create_character` → 8
directions (`rd_animation__8_dir_rotation` / PixelLab `rotate`) → animations
(PixelLab skeleton templates; RD walking workflow) → Aseprite cleanup and
anchor alignment (Claude can script the boring parts over MCP) → `sprtool
pack` (to be written) → `data\sprite\몬스터\<name>.spr/.act` in the loose data
dir → `grfbrowser` → in game. Claude's review step: side-by-side with a
Gravity monster of the same class at 4×.

**B. Headgear or item.**
Same as A but 1 frame (items) or 8 directions × a few actions (headgear), on
canvases ≤ 32 px. Cheapest place to test whether the style holds.

**C. UI chrome (window skins, screens like `bg_makebg.bmp`).**
No generator at all. Claude Design or hand-written SVG for the layout →
rasterize → 8-bit BMP with magenta key → `data\texture\유저인터페이스\…`.
This is the one asset class Claude can carry end to end alone today.

**D. A locked house style.**
Scenario LoRA on our *own* approved sprites once we have 10–30 of them (not
on Gravity's, see §7), or Retro Diffusion custom user styles. Only worth it
after A has produced something we like.

**E. New player body (the expensive one).**
~110 frames that must share anchors with every existing head and headgear.
The 3D route (Blender, 8-direction render) is the only one that keeps 110
frames consistent without weeks of hand work, and even then the RO look is
applied by hand afterwards. Not a first project.

## 7. Legal note

Style is not protected; specific sprites are. Using Gravity's sprites as
generator *references* or LoRA *training data* produces derivatives of
copyrighted work. That is tolerable for a private experiment and a problem
for anything published — the same posture `public-deployment.md` §7 takes on
the GRF itself. Generated output licensing: PixelLab and Retro Diffusion
grant commercial use of what they generate; Scenario's paid plans do too.

## 8. Proposed next step

A small feature, not a program: `/feature asset-pipeline`, Step 0 being the
three tools in §5 (SPR/ACT/PAL writers, loose data dir, quantizer) with
round-trip tests, and Step 1 being one Poring-class monster taken end to end
through pipeline A with both generators, so the choice between them is made
on a real result rather than on marketing pages. Budget for that experiment is
tens of dollars of generation credits.

Open questions:

1. Is a Windows VM acceptable on this machine for ActEditor/GRF Editor, or do
   we commit to Go packers only?
2. Which generator first? Retro Diffusion's per-image pricing suits an
   experiment; PixelLab's skeleton animation suits production of many frames.
3. Do we care about *new* art for the MVP at all, or only about the ability
   to make it (e.g. for a clean-room distribution later)?

## Sources

- PixelLab: [MCP page](https://www.pixellab.ai/mcp), [pixellab-mcp on GitHub](https://github.com/pixellab-code/pixellab-mcp), [Rotate tool](https://www.pixellab.ai/docs/tools/rotate), [Animate with skeleton](https://www.pixellab.ai/docs/tools/animate-with-skeleton), [Create 8-directional sprite](https://www.pixellab.ai/docs/tools/create-8-rotations-pro), [FAQ](https://www.pixellab.ai/docs/faq)
- Retro Diffusion: [site](https://retrodiffusion.ai/), [API examples](https://github.com/Retro-Diffusion/api-examples), [MCP server](https://mcpservers.org/servers/retro-diffusion/retro-diffusion-mcp), [rd-animation on Replicate](https://replicate.com/retro-diffusion/rd-animation/api), [Aseprite extension](https://astropulse.itch.io/retrodiffusion), [Runware write-up](https://runware.ai/blog/retro-diffusion-creating-authentic-pixel-art-with-ai-at-scale)
- Scenario: [pricing](https://www.scenario.com/pricing), [custom-trained models](https://help.scenario.com/en/articles/scenario-platform-models/)
- Gemini: [Generating game sprites with Nano Banana Pro — lessons learned](https://roboticape.com/2026/03/07/generating-game-sprites-with-gemini-image-generation-nano-banana-pro-lessons-learned/), [SpriteCook on Nano Banana pixel art](https://www.spritecook.ai/blog/nanobanana-pixel-art-for-games)
- Comparisons: [Ludo — 7 best AI sprite generators 2026](https://ludo.ai/compare/best-ai-sprite-generators), [Sprite-AI — best pixel art generators 2026](https://www.sprite-ai.art/blog/best-pixel-art-generators-2026), [Retro Diffusion vs PixelLab](https://gamedevaihub.com/retro-diffusion-vs-pixellab/)
- Claude: [Introducing Claude Design](https://www.anthropic.com/news/claude-design-anthropic-labs), [Can Claude generate images? (2026)](https://www.dreampixelforge.com/blog/can-claude-generate-images)
- Aseprite MCP: [ayigityol/aseprite-mcp](https://github.com/ayigityol/aseprite-mcp), [Vollkorn-Games/aseprite-mcp](https://github.com/Vollkorn-Games/aseprite-mcp), [ext-sakamoro/AsepriteMCP](https://github.com/ext-sakamoro/AsepriteMCP)
- RO tooling: [Tokeiburu/ActEditor](https://github.com/Tokeiburu/ActEditor), [Act Editor on rAthena](https://rathena.org/board/files/file/3304-act-editor/), [Tokeiburu/GRFEditor](https://github.com/Tokeiburu/GRFEditor), [zhad3/zrenderer](https://github.com/zhad3/zrenderer), [rAthena wiki — Spriting](https://github.com/rathena/rathena/wiki/Spriting), [rAthena wiki — Acts](https://github.com/rathena/rathena/wiki/Acts), [Basics of Ragnarok arting](https://rathena.org/board/topic/140768-tutorial-basics-of-ragnarok-arting-adding-custom-items/), [is Ragnarok's art pixel art? (GameDev.net)](https://www.gamedev.net/forums/topic/633661-is-ragnarok39s-art-pixel-art/4995826/)
- 3D route: [8 Directions Render Plugin for Blender](https://auteddy.itch.io/8-directions-render-plugin-for-blender), [Blender To Pixels](https://astropulse.itch.io/blender-to-pixels), [Pixelize add-on](https://github.com/LeonardoDocs/Pixelize), [PixZels](https://pixel-salvaje.itch.io/pixzels)
