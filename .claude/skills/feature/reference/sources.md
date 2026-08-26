# Where reference material comes from

Priority: **real kRO/iRO client** → roBrowser → korangar. GRF bitmaps complement all three for UI chrome. Always say in the caption which source an image is.

## 1. Real client (preferred)

No original client is installed on this machine (`/Users/borisglebov/git/RagnarokClients/kRO-client/` only holds BGM), so real-client references come from the web.

- **Search**: `WebSearch` for `"ragnarok online" <element> screenshot`, `irowiki <element>`, `ratemyserver <element>`. iRO Wiki (irowiki.org) and RateMyServer have clean UI captures; Reddit r/RagnarokOnline and YouTube for gameplay behaviour (walking, NPC dialog flow, damage numbers).
- **Fetch**: `WebFetch` the page to find the image URL, then `curl -L -o docs/features/<slug>/ref-NN-<what>.png "<url>"`. Check it is really a PNG/JPG (`file <path>`); convert JPG with `sips -s format png in.jpg --out out.png`.
- **YouTube stills** (`yt-dlp` + `ffmpeg` are installed via Homebrew):

  ```bash
  yt-dlp -f "bv*[height<=1080][ext=mp4]/bv*[height<=1080]" -o "$SCRATCH/ref-video.%(ext)s" "https://youtu.be/<id>"
  ffmpeg -hide_banner -loglevel error -ss <sec> -i "$SCRATCH/ref-video.mp4" -frames:v 1 -y docs/features/<slug>/ref-NN-<what>.png
  sips -Z 1280 docs/features/<slug>/ref-NN-<what>.png
  ```

  Pick the timestamp from the video's chapter/description or by scrubbing a few `-ss` values; keep only the frame, never the video. Cite `https://youtu.be/<id>?t=<sec>` under the image. Same `ffmpeg -ss … -frames:v 1` works on local recordings (e.g. `/Users/borisglebov/git/RagnarokClients/data/Korangar/*.mov`).
- Prefer the classic (pre-Renewal / 2010s kRO) look — that is the skin in `data.grf` and `data/skin/default/`.
- Record the source URL under the image. These are reference-only captures kept in docs.

## 2. roBrowser (fallback, exact original UI logic)

`/Users/borisglebov/git/RagnarokClients/roBrowser/` — JS port of the original client. Even without running it, `src/UI/Components/<Window>/<Window>.html|.css|.js` transcribes the original layouts pixel-for-pixel (PR #81 cross-checked the login window against it). Cite the file when you use it as a measurement source.

Running it needs a web server plus a client/GRF setup (`README.md`, `examples/`); only do that if a behaviour cannot be understood from the source.

## 3. korangar (fallback, modern re-implementation)

Built binary: `/Users/borisglebov/git/RagnarokClients/korangar/target/release/korangar` (run from the korangar dir; it needs its own data/client setup — see `wiki/Running.md`). Existing captures: `/Users/borisglebov/git/RagnarokClients/data/Korangar/*.png` (Geffen, tiles) and `korangar/.github/geffen_*.png`. korangar is the *engineering* reference (transform order, packet handling); its UI is deliberately not the original look, so mark korangar images as "behaviour reference, not visual truth".

## 4. GRF bitmaps (UI chrome — most precise)

```bash
go run ./cmd/grftool search  "$GRF" basic_interface
go run ./cmd/grftool search  "$GRF" login_interface
go run ./cmd/grftool extract "$GRF" "data/texture/유저인터페이스/basic_interface/<name>.bmp" "$SCRATCH/grf"
sips -s format png "$SCRATCH/grf/<name>.bmp" --out docs/features/<slug>/grf-<name>.png
```

Already-extracted skin: `/Users/borisglebov/git/RagnarokClients/data/skin/default/` (also `scribbling kid`). `docs/research/ro-ui-system.md` maps windows → texture files. Buttons are three-state sheets (`btn_x.bmp`, `_a` hover, `_b` pressed); windows are 9-sliced. Measure pixels from the extracted bitmap and quote them (as PR #81 did) — that is the proof Boris wants for "same element".

## 5. Our current state

```bash
make server-up
go run ./cmd/client --config config.yaml --screenshot-after 8s
cp data/Screenshots/latest.png docs/features/<slug>/current-<what>.png
pkill -f "cmd/client|/midgard$"
```

Existing captures from earlier work: `data/Screenshots/` (repo, gitignored) and `/Users/borisglebov/git/RagnarokClients/data/Screenshots/`.

## Image hygiene

- `sips -Z 1280 <file>` to cap the long edge; aim ≤ 500 KB (`ls -la`). Crop with `sips -c <h> <w> --cropOffset <y> <x>` when only one region matters.
- Full game frames rarely fit 500 KB as PNG (a 1280-px frame is ~1.2 MB). Save those as JPEG: `sips -s format jpeg -s formatOptions 80 in.png --out ref-NN-<what>.jpg`. Keep PNG only for UI bitmaps and crops where exact pixels matter.
- Names: `ref-NN-<what>.png`, `current-<what>.png`, `grf-<name>.png`. Numbers in the order the plan references them.
- Every image gets a caption + numbered legend in the README. No image without a legend.
