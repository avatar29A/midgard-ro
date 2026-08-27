# UC-217: Chat — `/` client commands

## Description
`/` commands are handled by the client, not the server: some are answered from
our own state, some send a dedicated packet. Nothing on the server parses a
leading `/`, so a `/` command must never leave as chat text.

## Preconditions
- Local rAthena stack running (`make server-up`)
- `make server-gm` applied (needed only for the GM commands below)
- Character `MidgardTest` on `prontera 156,191`

## Test Steps

### Answered without touching the server
1. Launch: `./build/midgard --autologin --trace=cmd,net --say "/where" --screenshot-after 20s`
2. Verify the box shows the map name and the character's cell
3. Verify the coordinates match the F3 overlay
4. Verify `cmd.local` appears in the trace and **no packet is sent** for it
5. Verify `/bgm` toggles the music and says which state it moved to
6. Verify `/sound` toggles sound effects and says so
7. Verify `/h` and `/help` both list the `/` commands we implement

### Answered by a round trip
1. `--say "/who"` (and separately `/w`)
2. Verify `cmd.server` reports `0x00C1` sent
3. Verify the player count appears in the box, decoded from `0x00C2`
4. Verify the count is plausible (1 with only this client connected)

### GM commands that carry their own packet
1. `--say "/mm prontera 150 150"`
2. Verify `cmd.server` reports `0x0140`, and the character moves to that cell
3. `--say "/b hello"` — verify the announcement appears
4. `--say "/lb hello"` — verify the local announcement appears
5. Verify each was sent as its own packet, **not** as `@mapmove` / `@kami` text

### The same commands as a non-GM
1. `make server-player`, relaunch
2. `--say "/mm prontera 150 150"`
3. Verify nothing happens and, critically, that **nothing is broadcast** —
   the chat box must not show `@mapmove` or `/mm` as a spoken line
4. This is why Step 4 sends real packets instead of atcommand text; verify it

### An unknown command
1. `--say "/nonsense"`
2. Verify the box shows an unknown-command line
3. Verify `cmd.unknown` appears in the trace
4. Verify **no packet is sent** and the text is not broadcast
5. Verify a bare `/` alone is handled without a crash or an empty line

### A command is not a sentence, and a sentence is not a command
1. Put a name in the whisper field and `--say "/where"`
2. Verify it still runs locally and no whisper is sent
3. `--say "hello there"` with the field empty — verify it is spoken publicly
4. `--say "hello there"` with the field filled — verify it is whispered
5. Verify a line that merely *contains* a slash (`and/or`) is spoken, not parsed

### Out-of-scope commands answer honestly
1. `--say "/sit"`, `--say "/!"`, `--say "/invite Bob"`
2. Verify each gives the unknown-command line rather than doing nothing silently
3. Verify none of them is broadcast

## Expected Results
- `/where`, `/bgm`, `/sound`, `/h` answer from the client with no packet
- `/who` round-trips `0x00C1` → `0x00C2`
- `/mm`, `/b`, `/lb` send their own packets and work as a GM
- As a non-GM those commands fail silently — never broadcast
- Unknown and out-of-scope `/` commands say so, send nothing, broadcast nothing
- The whisper field never changes how a `/` command is handled
