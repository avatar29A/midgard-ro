# UC-221: Character creation — a free name creates, a taken one is refused

## Description
Creation succeeds with a free name and is refused with a name already on the
server. Name uniqueness is only knowable server-side — there is no query for
it — so the refusal path is the feature, not a fallback.

## Preconditions
- Local rAthena stack running
- `midgard-test` with `MidgardTest` in slot 0 — a known-taken name
- At least one free slot

## Test Steps

### A free name creates the character
1. Launch: `./build/midgard --username midgard-test --password midgard-test --stop-at charselect --trace=char,net --make-char "QaFreshName" --screenshot-after 20s`
2. Verify `char.create-send` appears in the trace
3. Verify a `0x0A39` of **36 bytes** is sent (`--trace=net`)
4. Verify `0x0B6F` comes back and `char.create-ok` is traced
5. Verify character select returns with `QaFreshName` in the slot chosen
6. Verify the character can be entered and reaches Prontera
7. Verify in the database that its six stats are all **1** and that it holds
   **48** status points (`make server-shell-db`) — the server assigns both
   (`char/char_clif.cpp:1278-1285`, `char/char.cpp:2800`)

### A taken name is refused
1. Repeat with `--make-char "MidgardTest"`
2. Verify `0x006E` comes back with error **`0x00`**
3. Verify `char.create-refused` is traced with the code
4. Verify the screen says the name is already taken and **stays open**
5. Verify no character was created (slot still empty, database unchanged)

### Locally-rejectable names never reach the wire
Our server's rules, from `char_athena.conf`: 4–23 characters
(`char_name_min_length: 4`, `NAME_LENGTH 24`), and letters, digits or space
only (`char_name_option: 1` with `char_name_letters`).

1. Try an empty name, a 3-character name, a 24-character name, and one
   containing a symbol such as `!`
2. Verify each is refused on the screen with a reason
3. Verify `--trace=net` shows **no** `0x0A39` for any of them
4. Verify a 4-character alphanumeric name **is** accepted and does send

## Expected Result
A free name creates a character with the stats the screen showed; a taken name
is reported and costs nothing.

## Notes
Refusal codes (`char_clif.cpp:1330-1352`): `0x00` name taken, `0xFF` denied,
`0x01` underaged, `0x03` slot not eligible. Only `0x00` is expected here; the
others should still render as a message rather than silence.

`QaFreshName` accumulates in the database across runs — delete it between
passes, or vary it, or `make server-reset`.
