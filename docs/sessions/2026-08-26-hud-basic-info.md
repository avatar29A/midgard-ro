# 2026-08-26 — Basic Info HUD

Issue #85, branch `feature/hud-basic-info`, worktree `midgard-ro-hud`.

The original client's top-left panel, built in the six steps the feature plan
laid out. What follows is the part worth remembering: what the plan got wrong,
and what cost time.

## Two bugs the tooling found before the panel could be trusted

**The `CharInfo` offsets were guessed, and wrong in a way that looked right.**
Character select had been reporting `Job 40`, `Lv. 0` and `HP 11/11` beside
`SP 40/40`. That reads as three separate faults; it was one. The offsets were
taken from a capture rather than from rAthena's `CHARACTER_INFO`, and `hp` was
being read at offset 66 — where `sp` lives — with `maxhp` at 74, which is
`maxsp`. A character with 40 HP and 11 SP came back as 11 HP and 40 SP: two
real numbers in each other's places, which is exactly why nobody caught it by
looking. `class` was read at 50, the low bytes of `hp`, giving Job 40; `level`
at 90, which is `weapon`, giving Lv. 0.

The layout is version-dependent, and the branch that matters is
`PACKETVER_RE_NUM >= 20211103`, where `hp`, `maxhp`, `sp` and `maxsp` are each
`int64` — which is what makes the record 175 bytes. So `CharInfoSize = 175`
was right for the wrong reason, and everything inside it was wrong.

**Experience arrives over a packet we were not handling.** At
`PACKETVER >= 20170830`, `clif_updatestatus` routes all four experience values
through `0x0ACB` (`ZC_LONGLONGPAR_CHANGE`, 12 bytes, 64-bit value) rather than
`0x00B1`, because the totals no longer fit in 32 bits. Handling only `0x00B0`
and `0x00B1` would have left the experience bars empty with nothing visibly
broken.

**Weight arrives in tenths of a unit.** The server sent `SP_MAXWEIGHT = 20300`
for a Novice; the panel should read 2030. Without the conversion it would have
read `0 / 20300` convincingly.

## What stopped unattended runs

**vsync.** With `vsync: true` and the window occluded — which it is whenever
the client is launched from a shell — macOS stops servicing the display link
and the buffer swap never returns. The process stays alive at ~2% CPU and
renders *zero* frames, so `--screenshot-after` never fires. It looks exactly
like a hang, with no error anywhere. `vsync: false` in the local `config.yaml`
fixes it outright. Several runs were lost to this before it was measured.

**The loading screen's Enter gate.** It held at 100% waiting for a keypress,
so even a rendering client stopped there. Removed at Boris's request.

## Verifying by measurement rather than by eye

roBrowser's `BasicInfo.css` was checked against the bitmap by reading its
pixels: the HP trough runs rows 53–61 across columns 35–169, which is
roBrowser's `(35, 53) 135×8` exactly. That one check made the rest of the
stylesheet trustworthy, and it is where the exp bars' colors and the weight
conversion came from.

A full gauge proves nothing — 100% and a broken proportion look identical — so
the character was damaged directly in the server's database before logging in.
That turned into the better test: by the time the screenshot fired, natural
regeneration had moved the values, and the panel read `18 / 40` and `7 / 11`
rather than the `17 / 4` it started with, each tick visible in the trace as its
own packet.

## Left for later

- The menu buttons open nothing. Each window is its own feature.
- The original fades the button strip to 50% opacity until hovered.
- Dragging is wired to the same `DragHandle` the login and character select
  windows use, but was not verified visually — synthesising a drag needs the
  window's position on screen, which SDL does not expose through the
  accessibility API.
