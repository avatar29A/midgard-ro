# UC-218: Chat — every line coloured by where it came from

## Description
The chat box paints each line by its kind, as the original does: your own words
and the server's replies green, other people white, whispers yellow, errors red.
Two packets carry a colour of their own and must be honoured rather than
overridden.

## Preconditions
- Local rAthena stack running (`make server-up`)
- `make server-gm` applied
- Character `MidgardTest` on `prontera 156,191`
- Reference: `docs/features/chat-commands/ref-02-chat-palette.png`

## Test Steps

### The four kinds side by side
1. Launch: `./build/midgard --autologin --trace=cmd,hud \
   --say "@help" --say "/where" --say "/nonsense" --say "hello" \
   --screenshot-after 25s`
2. Verify four visibly different colours in the box
3. Compare each against ref-02 and record any that do not match

### Server replies are green, not yellow
1. Verify rAthena's two welcome lines on login are **green**
2. This is a change from today, where they are yellow — verify it moved
3. Verify every `@` command reply is the same green
4. Verify the green matches roBrowser's `#00FF00` for `0x008E`

### Our own words
1. `--say "hello"` with the whisper field empty
2. Verify the echoed line is green, matching the original's `PUBLIC|SELF`
3. Verify the speaker prefix is still distinguishable from the message body

### Someone else's words
1. Connect a second client, or use `@kami`-adjacent means to produce a line
   from another character
2. Verify it is white, not green — the colour is what tells them apart

### Whispers
1. Whisper to `MidgardTest` from a second session, or use `#` to trigger one
2. Verify the whisper is **yellow** (`#FFFF00`)
3. Verify it is not purple — that is today's colour and must change
4. Verify a failed whisper ("... is not online.") is distinguishable from a
   delivered one

### A packet that carries its own colour (`0x02C1`)
1. `--say "@rates"`
2. Verify the lines are rAthena's light green `#B5FFB5`
3. Verify they are **not pink** — the colour on the wire is BGR, and reading it
   as RGB turns `0xB5FFB5` into a light pink. This is the specific failure to
   look for.
4. `--say "@mobinfo poring"` and verify the same handling

### Broadcast colour prefixes (`0x009A`)
1. `--say "@kami hello"` — verify the line is **yellow** and reads `hello`
2. `--say "@kamib hello"` — verify the line is **blue** and reads `hello`
3. Verify neither renders the literal prefix: no `bluehello`, no `sssshello`
4. Verify a WoE-style `"ssss"` message likewise loses its prefix

### Colour codes inside the text still win
1. Produce a line containing `^FF0000` markup (NPC text or a script)
2. Verify the inline code overrides the line's kind colour, as it does today
3. Verify the rest of the line returns to the kind colour afterwards

### Legibility
1. Verify every colour is readable against the half-transparent dark backdrop
2. Verify every colour is still readable when the chat is dragged over Prontera's
   pale pavement
3. Repeat at 1280×720 and 1920×1080

## Expected Results
- Own words and server replies green; others white; whispers yellow; errors red
- `0x02C1` honours its colour, decoded from BGR — never pink
- `0x009A` prefixes are consumed, not printed
- Inline `^RRGGBB` codes still take precedence
- Every colour legible over both the dark backdrop and pale ground
