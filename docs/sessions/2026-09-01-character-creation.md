# 2026-09-01 — Character creation

Issue #109, branch `feature/character-creation`.

Double-click an empty slot, build a character, press Create. Race, sex, hair
style, hair color, name. What follows is what the plan got wrong and what only
running it found.

## What the plan got wrong

**The screen.** The first plan was built around `login_interface/win_make.bmp`,
the classic layout with the hexagonal stat radar, because that is the
screenshot Boris supplied. It is in our archive and it is the wrong screen: at
`PACKETVER 20211103` the request carries no stats at all, and the server writes
1 to each itself. The layout to build is `make_character_ver2/bg_makebg.bmp`,
which has a control for every field the packet carries and none for anything it
does not. Boris settled it by sending a screenshot of the modern screen and
asking whether it was closer to the server. It was — exactly.

**And then the wrong file for that screen.** The revised plan named
`bg_back2.tga` as the frame, with a legend describing race cards and a hair
grid. `bg_back2.tga` is the modern character *select* — a fifteen-slot grid
with an info panel. I had extracted it, checked its dimensions, and written a
legend for it without ever opening the image. `bg_makebg.bmp` is 794×422, the
size of the retail screenshot, which is the check that would have caught it at
a glance.

**Both assumptions about sex and stats inverted.** The plan assumed stats were
distributable and sex came from the account. At our packet version stats are
not sent at all, and sex *is* — the client chooses it. The account's sex is
only where the screen starts.

## What only running it found

**The client believed every account had 15 slots.** It has 9.
`HC_ACCEPT_ENTER 0x006B` does not carry a creatable count; its bytes are
`total`, `premium_start`, `premium_end`, and we read the second as one. The
real number rides on `HC_ACCEPT_ENTER2 0x082D`, which the server sends first
and we ignored entirely. The `char` trace channel found this on its first run —
about twenty minutes after being added, which is the argument for tooling being
Step 0 rather than an afterthought.

**Two packet ids were stale and silent.** `CH_MAKE_CHAR` was `0x0067` and
`HC_ACCEPT_MAKECHAR` was `0x006D`. Both are correct for older clients and
wrong for ours, and nothing would have said so — a creation would simply never
have worked. Found by reading the server's headers, not by any check we run.
A generator that read `char/char_clif.cpp` would have caught both; ours reads
only the map server's table, so no char-server packet is in it.

**The connection closed while the screen was up.** Boris opened creation, left
it, and came back to a network error across the frame. The server drops a
session that has said nothing for 60 seconds, and creation is a screen you sit
on. Both screens now ping every 20 seconds. The error had also been swallowed —
it reached the screen and nothing else, so there was no timestamp to reason
from and the cause took a detour through the char server's source.

**A three-pixel gap survived being written, built and run**, because there was
no way to open the creation screen without a person double-clicking a slot.
`--stop-at charcreate` exists now. The fix for it was then wrong in the other
direction: I moved the whole Hair Color control down, out through the bottom of
a box painted into the frame, because I had placed that box "roughly 296..360"
by eye. Reading the frame's own pixels put it at 299..358 and the room came
from tightening the grid rows instead.

**The archive files the two races inconsistently.** Sprites: both races get a
folder. Palettes: humans sit bare under `data\palette\머리\`, Doram get one.
It looks like an oversight and tidying it would break every human hair color,
so it is a table test with both races in it.

**A test of mine was wrong rather than the code.** Cross-checking the request
against `ClientPacketLength` failed, and the honest reading was that the table
does not cover char-server packets — not that the packet was wrong.

## Numbers

- 11 commits, each building and testing on its own.
- 2 stale packet ids corrected; 1 slot count that was off by six.
- 1 new file format read: external `.pal`, which nothing in the engine could
  read before.
- 4 QA flags added: `--stop-at charselect`, `--stop-at charcreate`,
  `--make-char`, and the `char` trace channel.
- 4 rounds of British spellings caught by the linter, now a memory.
- Proved by creating `QaNovice01` against the running server: stats all 1 and
  48 status points, neither of which the client sends.
