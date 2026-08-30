# 2026-08-31 — Chat commands: the GM packets and the palette

Issue #94, branch `feature/chat-commands-colors`. Finishes what PR #99 left
open: steps 4, 5 and 6, plus the F3 fields deferred from step 0b.

#99 merged with steps 0–3 — parsing, routing, the GM account, and the five `/`
commands we answer ourselves. This is the rest: the three `/` commands that
carry their own packets, the colour palette, and the docs.

## What the plan had right

Unusually, most of it. The three packet ids in the plan (`0x0140`, `0x0099`,
`0x019C`) all matched the generated client table, with the lengths the plan
predicted — 22 bytes fixed for `/mm`, variable for both broadcasts. The
`slash.json` written back in step 0 already recorded the aliases (`/mapmove`,
`/nb`, `/nlb`) and the packet each command uses, so the implementation was
transcription rather than research. That file paying off three steps later is
an argument for writing the reference before the code.

## What only writing it found

**`byte(0x0140)` does not compile.** The packet layer's established idiom for
writing a header is `pkt[0] = byte(ID); pkt[1] = byte(ID >> 8)`, and it works
for every id the client had sent up to now — because all of them are under
256. `byte()` of a *constant* that overflows is a compile error, not a
truncation, so the first id above `0x00FF` broke the pattern. The package had
`writeU32` but no `writeU16`; adding it and using `writeU16(pkt, 0, ID)` is
both correct and shorter.

**A symmetric colour proves nothing.** The first version of the BGR test used
rAthena's light green `0xB5FFB5` — the realistic value, and the one the
research doc names. Its red and blue bytes are equal, so a decoder that had
forgotten to swap would have passed. Rewritten with an asymmetric colour, then
checked by deleting the swap and confirming the test failed. Worth doing for
anything whose whole job is to transform a value: pick the input that can tell
the transform apart from the identity.

**The broadcast marker is ambiguous and cannot be fixed.** `ZC_BROADCAST` has
no colour field; the server prepends the literal word `blue` or `ssss` and
expects the client to cut four characters off. So `@kami blueberries` arrives
as `blueberries` and displays as "berries". There is no length, no flag, and
no escape — the original client reads it exactly the same way. Pinned as a
test case rather than left as a surprise, because the first person to see it
will otherwise file it as a bug.

**"Sent" is not "ran".** The GM three deliberately print nothing on success,
and the F3 outcome says `sent` rather than `done` for the same reason: whether
the server accepted a GM command is the server's to say, and for a non-GM the
answer is silence. That silence is the *point* of sending these as packets —
as `@` text a refused command falls through to ordinary chat and shouts itself
at the map — so a client-side "done" would contradict the feature.

**The research doc named the wrong commands for `0x02C1`, and running it is
what caught that.** Step 0's reference said `@rates`, `@mobinfo` and
`@iteminfo` answer on the coloured packet. Sending `@rates` to our own server
and tracing the reply showed no `0x02C1` at all — it calls
`clif_displaymessage` four times (`atcommand.cpp:8843`). The 7 call sites the
doc counted are real, but they belong to `@cash`, `@points`, `@request` and
`@auction`. The number was right and the names were wrong, which is the
failure mode a generated table is meant to prevent — and these names were the
hand-written half.

**And `0x02C1` cannot easily be seen at all.** Having found the right four,
none of them could be made to send one: `@cash` and `@points` reach
`clif_messagecolor` only when `battle_config.cashshop_show_points` is off, and
it defaults on, so a successful `@cash 100` reports through `pc_getcash` on
`0x008E` instead. `@request` wants another GM online; `@auction` wants
auctions enabled. Three runs went into establishing this. The decoder is
therefore proved against the server's own `clif_messagecolor_target` — exact
offsets, exact swap expression — and by unit test, **not** by a live packet.
Both docs now say so rather than implying it was seen working.

## The overlay field earns itself immediately

`Cmd: /nonsense -> unknown` versus `Cmd: /mm prontera 150 150 -> sent`. On
screen those two look the same: the chat box is unchanged either way. The
first means the name never resolved and the fix is in our table; the second
means it reached the server and the silence is the account's group level. The
trace channel says the same thing, but a trace has to be enabled before the
run that needed it.

## Numbers

- 3 new client packets, all cross-checked against the generated server table.
- 1 pre-existing helper gap (`writeU16`) found by needing it.
- 1 factual error corrected in the step 0 research doc.
- The palette: 6 kinds, 2 packets that override it.
- Live: `@help` green, `/where` light yellow, `/nonsense` red, all in one
  screenshot, with `Cmd: /nonsense -> unknown` on F3.
- `go test -race ./...`, `gofmt` and `go vet` clean (`go vet` reports two
  findings, both pre-existing on `main` in files this branch does not touch).
