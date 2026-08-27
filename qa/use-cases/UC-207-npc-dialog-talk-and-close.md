# UC-207: NPC Dialog — Talk and Close

## Description
Clicking an NPC opens its dialog, `Next` advances a multi-page script, and
`Close` ends the conversation and returns control to the player. This is the
baseline conversation flow every other NPC interaction is built on.

## Preconditions
- Local rAthena stack running (`make server-up`)
- Character on `prontera` near the fountain (`156,191`)
- Prontera's standard NPCs visible (the client draws them; see UC-204 range for entity rendering)

## Test Steps

### Open a dialog
1. Launch: `./build/midgard --autologin --trace=npc,net`
2. Click directly on the Kafra Employee
3. Verify the `npc` trace shows `npc.click` then `npc.contact` with her account id
4. Verify a dialog window appears over the map
5. Verify it contains her greeting text, not an empty box
6. Verify the character did **not** walk toward her

### Advance a multi-page script
1. Talk to Prontera's Guide (`Guide#01prontera`), whose script has several pages
2. Verify a `Next` button appears when the server sends `ZC_WAIT_DIALOG`
3. Click `Next`
4. Verify the trace shows `CZ_REQ_NEXT_SCRIPT` and the following page arrives
5. Verify earlier text remains readable (the box scrolls rather than clearing)

### Close cleanly
1. Continue until a `Close` button appears (`ZC_CLOSE_DIALOG`)
2. Click `Close`
3. Verify `CZ_CLOSE_DIALOG` is sent and `npc.close` appears in the trace
4. Verify the window disappears
5. Verify clicking the ground walks the character again
6. Verify the client is still connected — no disconnect, no error in the log

### Line breaks are preserved
1. Talk to an NPC whose script formats text over several lines
2. Verify the line breaks match the script rather than being re-flowed into one paragraph

## Expected Results
- Dialog opens on click and shows the NPC's own text
- `Next` advances; `Close` ends and returns control
- Movement is suppressed while talking to an NPC, restored afterwards
- No disconnect at any point
