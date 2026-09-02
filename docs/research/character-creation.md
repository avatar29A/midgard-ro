# Character creation: what our server accepts

Written while building #109. Everything here is read off the rAthena we build
(`docker/rathena/build/rathena`, pinned in `docker/rathena/pin.txt`) at
`PACKETVER 20211103`, not from a wiki. Re-check after a `make server-rebuild`
onto a newer tree.

## 1. `CH_MAKE_CHAR` has three shapes, and we get the newest

Behind `PACKETVER` guards in `common/packets.hpp:122-157`:

| PACKETVER | id | Carries |
|---|---|---|
| `>= 20151001` **← ours** | `0x0A39` | name[24], slot, hair_color, hair_style, **job**, **sex** |
| `>= 20120307` | `0x0970` | name[24], slot, hair_color, hair_style |
| older | `0x0067` | name[24], **str, agi, vit, int, dex, luk**, slot, hair_color, hair_style |

**The six stat bytes exist only in the oldest form.** The handler does not
merely ignore them at our version — under `PACKETVER >= 20151001` it reads
`job` and `sex` from the packet and assigns a literal `1` to each stat
(`char/char_clif.cpp:1272-1285`).

That is the whole reason the classic "Make Your Characters" screen, with its
hexagonal stat radar, cannot work at this packet version. Its own arithmetic
confirms which era it belongs to: below `PACKETVER 20120307` the server
requires the six stats to total exactly 30 with none below 1
(`char/char.cpp:1437-1446`), and the reference screenshot shows six fives.

New characters get `start_status_points` instead — **48** on our build
(`char/char.cpp:2800`) — and spend them in the stat window.

## 2. The replies move too, and we sit on a boundary

| Direction | Packet | Id | Source |
|---|---|---|---|
| ← | `HC_ACCEPT_MAKECHAR` | **`0x0B6F`** | `common/packets.hpp:259-270` |
| ← | `HC_REFUSE_MAKECHAR` | `0x006E` | `common/packets.hpp:273-277` |

The accept's guard is
`PACKETVER_MAIN_NUM >= 20201007 || PACKETVER_RE_NUM >= 20211103`. Ours is
*exactly* the RE boundary. One version earlier it is `0x006D` with a different
body — and `0x006D` is what our client had hardcoded until this feature, along
with `0x0067` for the request. **Both were wrong and both were silent.**

Refusal codes, mapped from `char_make_new_char`'s negative returns in
`char/char_clif.cpp:1330-1352`:

| Wire | Meaning |
|---|---|
| `0x00` | Name already taken |
| `0x01` | Underaged |
| `0x03` | Slot not eligible |
| `0xFF` | Denied — bad name, bad job, bad sex, slot in use, account limit |

`0x00` is the only answer to "is this name free". Nothing queries that, so a
client cannot pre-check it.

## 3. What our server allows

| Thing | Value | Source |
|---|---|---|
| Creatable jobs | Novice **and** Summoner (Doram) | `char/char.cpp:2814-2818` — we define `RENEWAL` (`config/renewal.hpp:24`) at `PACKETVER_RE 20211103`, so `allowed_job_flag = 3` |
| Sex | Male or female, per character, sent by the client | `char/char.cpp:1424-1434` |
| Name length | 4 – 23 | `char_athena.conf:149`; `NAME_LENGTH 24` in `common/mmo.hpp:154` |
| Name characters | Letters, digits, space | `char_athena.conf:160,164` |
| Slots | The account's own count, **not** `MAX_CHARS` | see §4 |

## 4. Two slot counts, and only one of them is the account's

`HC_ACCEPT_ENTER 0x006B` carries the characters but **not** a creatable count.
Its three bytes are `total`, `premium_start` and `premium_end`
(`common/packets.hpp:240-251`) — `total` being `MAX_CHARS`, the server's
compile-time ceiling, 15 on a stock build.

The account's own limit rides on **`HC_ACCEPT_ENTER2 0x082D`** as `producible`
(`char/char_clif.cpp:435-451`), which the char server sends *before* the
character list from `PACKETVER >= 20130000` (`char_clif.cpp:477-481`).

Our client read `0x006B`'s second byte as an available-slot count and so
believed every account had 15 slots when ours have 9. Reading `0x082D` is the
fix; `--trace=char` reports both numbers.

## 5. Sessions are dropped after a minute of silence

`stall_time` defaults to 60 seconds (`common/socket.cpp:235,1016`). Character
select and character creation are both screens a person *reads* — picking a
name and a hair style takes longer than that on its own — so both send
`PACKET_PING 0x0187` every 20 seconds (`char/char_clif.cpp:1395,1604`).
Without it the connection closes mid-screen and reads as the screen breaking.

## 6. Things that will bite

- **The generated client packet table does not cover this.**
  `internal/network/packets/clientlengths.go` comes from
  `tools/packetlen/gen.py`, which reads the **map** server's
  `clif_packetdb.hpp`. No char-server packet is in it — not `CH_ENTER`, not
  `PING`, not `CH_MAKE_CHAR`. Both stale ids in §2 would have been caught
  automatically had the generator also read `char/char_clif.cpp`. It does not.
- **`grftool search` caps at 50 results** with no indication. Use
  `grftool list` with a glob for any inventory; its glob matches the basename
  only, not the path.
- **Korean directory names are EUC-KR inside the archive.** Decoding a listing
  as UTF-8 with replacement characters produces a path `extract` cannot find.
- **The GRF holds four creation layouts** from four client eras.
  `login_interface/win_make.bmp` is the classic hexagon screen;
  `make_character_ver2/bg_makebg.bmp` is the one that matches our packet
  version. `make_character_ver2/bg_back2.tga` is *not* a creation screen at
  all — it is the modern character select.
- **New characters start on `iz_int03`**, the server's configured start point,
  not Prontera.
