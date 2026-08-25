# Audio

Background music and sound effects, on top of [beep](https://github.com/gopxl/beep).

The package has two layers:

| Type | Job |
|---|---|
| `Manager` | Owns the speaker. Plays whatever bytes or files it is handed, at a volume. |
| `LocationPlayer` | Answers the only question the game actually has: *what should be playing where the player is?* |

Most code wants `LocationPlayer`. It is already wired: `game.initAudio` builds
both and hands the player to `states.Manager`, so states just call
`PlayLocationBGM` / `PlayFallbackBGM`.

---

## Quick start

```go
// Anything that is not a map — login, character select, loading.
s.manager.PlayFallbackBGM()

// A map is up. The track repeats until another location plays.
s.manager.PlayLocationBGM(s.MapName)
```

Those two helpers live on `states.Manager`, tolerate audio being unavailable
(`BGM` is nil), and log failures rather than breaking the state machine. Music
is a nicety — it must never take the game down with it.

Driving the player directly:

```go
player := audio.NewLocationPlayer(manager, table, bgmDir)

_ = player.PlayFallback()
_ = player.PlayLocation("prt_fild08")

player.Track()          // "bgm/12.mp3"
player.UsingFallback()  // false
```

---

## Where the audio comes from

The two kinds of audio live in completely different places, and this trips up
everyone once:

| | Sound effects | Background music |
|---|---|---|
| Location | **Inside the GRF**, under `data/wav` | **Outside the GRF**, in a `BGM` folder |
| Format | RIFF PCM, 16-bit mono 22050 Hz | MPEG-2 Layer III, ~80 kbps 22.05 kHz |
| Count (kRO 2023) | 3103 `.wav` | 182 `.mp3` |
| Loaded by | `assets.Manager.Load` | `os.ReadFile` from `bgm_dir` |

**There is not a single `.mp3` in `data.grf` or `rdata.grf`.** RO clients ship
the music as loose files next to the archives. Looking for music *inside* the
GRF only finds the *mapping* that says which track belongs to which map.

`DefaultBGMDir(grfPath)` returns `<dir of the GRF>/BGM`, the stock client
layout, and is the default when `audio.bgm_dir` is unset in `config.yaml`. With
no music installed the game still runs; it logs a warning and stays silent.

---

## How the location → track mapping works

### The source of truth

`data/mp3nametable.txt`, inside the GRF. Every line looks like:

```
prontera.rsw#bgm\\08.mp3#
```

Parsing rules, all of which `ParseNameTable` implements — read them before
writing your own parser, because each one is a real trap:

1. **Comments start with `//`.** They are not just a header: retired mappings
   are left in the file commented out, so `//prontera.rsw#bgm\\55.mp3#` sits a
   few lines from the live `prontera.rsw#bgm\\08.mp3#`. Take the wrong one and
   you get the wrong music.
2. **Comments are CP949 (Korean), not UTF-8.** Never decode them. Skip the line
   before touching its bytes; the entries themselves are pure ASCII.
3. **The separator is `#`, and there is a trailing one.** A valid line splits
   into at least three fields: resource, track, and the empty remainder.
4. **The path separator is an escaped backslash — `bgm\\08.mp3`, two
   characters.** Naively replacing `\` with `/` gives `bgm//08.mp3`.
   `NormalizeTrack` collapses it to `bgm/08.mp3`.
5. **Keys are `.rsw` resource names**, lowercase in practice but do not rely on
   it.
6. **Duplicate keys exist.** Last one wins.

### The lookup key

The key is the map's **resource name** — the same id the server sends and the
same base name as the map's `.gat`/`.gnd`/`.rsw` files. `NormalizeLocation`
accepts every shape that name shows up in, which matters here because
`InGameState.MapName` arrives carrying `.gat`:

```go
audio.NormalizeLocation("PRONTERA.RSW")      // "prontera"
audio.NormalizeLocation("prontera.gat")      // "prontera"
audio.NormalizeLocation(`data\prontera.gnd`) // "prontera"
```

**Location ids are the real kRO map names, which are abbreviated and not
guessable.** Prontera's surrounding fields are `prt_fild00` … `prt_fild11`, not
`prontera_fld`. Its indoor maps are `prt_in`, `prt_castle`, `prt_church`. When
in doubt, look the name up instead of constructing it — `TrackFor` resolves
without touching playback:

```go
if track, known := player.TrackFor(id); !known {
    // id will fall back to the title theme
}
```

### Coverage, measured against kRO 2023 `data.grf`

| | |
|---|---|
| Maps (`.rsw`) in the GRF | 988 |
| Entries in the name table | 1142 |
| Maps **with** a track | 947 (95.9%) |
| Maps **without** a track | 41 |
| Table entries for maps not in the GRF | 195 |
| Distinct tracks | 191 |
| Tracks shared by more than one map | 173 |
| Referenced tracks **absent** from a stock BGM folder | 16 |

Three things follow, and they are why the fallback exists:

- **~4% of real maps have no music at all.** Instance and event maps mostly —
  `1@pdb`, `airplane2`, `evt_bomb`. A missing entry is normal, not a bug.
- **The table references tracks nobody ships.** 16 of them are numbered
  `bgm/182.mp3` and above, while a stock folder holds `01`–`181` plus `997`. A
  perfectly valid location can still have no playable file.
- **Most tracks are shared.** `prt_fild01` and `prt_fild08` are both
  `bgm/12.mp3`. Walking between them must *not* restart the music — and does
  not: `PlayLocation` is a no-op when the resolved track is already playing.
  Rely on that instead of tracking the previous location yourself.

---

## The fallback

`DefaultFallbackTrack` is `bgm/01.mp3`, whose ID3 title is literally `Title` —
the theme the real client plays on its title screen. It covers two cases:

- **Deliberately**, via `PlayFallback()`: the non-game screens — login,
  character select, connecting, loading.
- **Automatically**, via `PlayLocation()`: any location the name table does not
  know, including typos and made-up ids.

So an unknown location is never an error and never silence:

```go
_ = player.PlayLocation("prontera_fld") // not a real map
player.Track()          // "bgm/01.mp3"
player.UsingFallback()  // true
player.Location()       // "prontera_fld"
```

`SetFallbackTrack("")` disables it, making those cases silent instead. An error
from `PlayLocation` means something else went wrong — almost always that the
`.mp3` is not in the BGM folder.

---

## Formats

`Manager.PlayBGM` picks its decoder from the path's extension, so the same call
handles both kinds of asset:

- `.mp3` → `beep/mp3` (background music)
- `.wav` → `beep/wav` (sound effects, and WAV music if you have any)

Looping needs a seekable stream, which both decoders provide. Anything else is
rejected with a clear error rather than being fed to the wrong decoder.

`PlayBGMFile` is the file-on-disk entry point; `PlaySFX` takes the bytes
`assets.Manager.Load` returns.

---

## Layering

`internal/engine/` may only import `pkg/` (see `CLAUDE.md`), so this package
must not reach for `internal/assets`. It declares what it needs instead:

```go
type AssetLoader interface {
    Load(path string) ([]byte, error)
}
```

`assets.Manager` satisfies it, and `game.initAudio` passes it in. `BGMPlayer`
does the same for `LocationPlayer` → `Manager`, which is also what lets the
location tests run with no audio device — CI has no speaker.

---

## Gotchas

- **Never call `speaker.Init` in a test.** CI has no audio device. Test against
  the `BGMPlayer` interface with a fake, as `location_test.go` does.
- **Volumes are 0.0–1.0** and clamp; they are converted to dB internally.
- **`Manager.Close()` must run** — `game.Close` does it.
- **On Linux, `oto` needs ALSA headers** (`libasound2-dev`) at build time
  because it links via `#cgo pkg-config: alsa`.

---

## Verifying a mapping by hand

```go
table, _ := audio.LoadNameTable(assetManager)
track, found := table.Lookup("prt_fild08") // "bgm/12.mp3", true
```

The kRO music files are ID3-tagged, so a track can be confirmed without
listening to it:

```sh
mpg123 -n 1 /path/to/BGM/08.mp3 2>&1 | grep Title   # "Theme of Prontera"
```
