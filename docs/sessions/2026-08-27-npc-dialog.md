# 2026-08-27 — NPC dialog and interaction

Issue #86, branch `feature/npc-dialog`, worktree `midgard-ro-npc`.

Click an NPC and talk to it: text, Next, Close, and menus. What follows is the
part worth keeping — what the plan got wrong, and what only testing found.

## What the plan got wrong

**Both dialog buttons were the wrong files.** The plan pointed at
`login_interface/btn_next.bmp` and `basic_interface/btn_close.bmp`. roBrowser
names them with no folder prefix, which resolves to the **root** of the
interface folder, where a full six-bitmap set lives. The subfolders hold
different buttons that share the names — and it is the `login_interface` one
that genuinely has no hover art, which is what the plan had recorded as a
property of the button rather than of that particular copy.

**`0x0BA8` looked like it had replaced `CZ_CHOOSE_MENU`.**
`PACKET_CZ_CHOOSE_MENU_ZERO` is declared under `PACKETVER_RE_NUM >= 20211103`,
which our server satisfies. It has not: `clif_packetdb.hpp` registers `0x00b8`
and never mentions `0x0ba8`. Reading the packet table rather than the struct
list is what settled it.

**Neither window has a title bar,** so `win_msgbox.bmp` — which the plan said
to reuse — was the wrong chrome. Boris's screenshots showed it and roBrowser's
stylesheet agreed.

**Empty menu entries do not count toward the selection index.**
`menu_countoptions` counts only non-empty options into `sd->npc_menu`, which is
the bound the server checks. So `a::b` offers two choices and `b` is choice 2.
Getting this wrong is not a wrong branch — `clif_parse_NpcSelectMenu` calls
`clif_GM_kick`.

## What only testing found

**NPC text carries markup we had never seen.** After the color codes came
`<NAVI>[northwest]<INFO>prontera,55,350,0,000,0</INFO></NAVI>`, printed raw on
screen. The original renders the label as a link. There may be more tags —
`<ITEM>`, `<URL>` — and nothing was invented for ones no script here uses.

**Cancelling a menu stranded the player.** `buildin_menu` handles 255 with
`st->state = END` and sends nothing back, because the original client has
already closed its own window. Our Close button only appears when the server
asks, so the text window sat there with no way out. The trace showed it plainly:
`npc.cancel`, then silence.

**The line pitch was half the original's.** Measuring ref-01 — a lossless
capture at native size — put the text rows at 138, 159, 180, 201, 222: an exact
21px pitch with 9–11px glyphs. Ours were 13 apart with the same size glyphs.

**A blank line between messages double-spaced everything.** Every `mes` is its
own packet — page one of the Prontera Guide is five of them — so the break had
to go at the page boundary instead, which is the only boundary the script marks.

**A "slow" Close was rAthena giving up on us.** A Close appearing a minute after
the last Next looked like lag. The net trace showed the server answering in
16ms with a *menu*, which we ignored because menus were not built yet, and
`SECURE_NPCTIMEOUT` force-closing the conversation sixty seconds later.

## The batching order, twice

`ui2d` draws images, then solids, then text. The dialog buttons were invisible
and still clickable: the window painted its background with `DrawRect` — a
solid — over buttons that are images. **Invisible but clickable is the
signature of this fault**, since hit tests do not care what is on top.

The cursor had vanished under the Basic Info experience bars for the same
reason a day earlier. Two occurrences is a pattern: **a panel built from solids
cannot host an image widget**. Both were worked around locally — the dialog
fills itself from a tinted 1×1 texture — but the constraint itself is still
there and deserves a proper fix.

## Debug tooling that paid for itself

Three clicks at an NPC missed before anything worked, and a bare "no hit" could
not say why. Adding the projected screen position of every candidate to the
`npc` trace separated three different faults at a glance: no NPC tracked, boxes
in the wrong place, or aim in the wrong place. It was the third.

## Left for later

- Navigation labels are shown but not clickable; that needs the minimap.
- Warps get the talk cursor, since nothing the server sends distinguishes them.
- Shops answer `CZ_CONTACTNPC` with `0x00C4`, which is its own feature.
