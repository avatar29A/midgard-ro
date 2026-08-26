# UC-207: NPC Dialog — Cancel and Index Bounds

## Description
Cancelling a menu sends 255, and no interaction can produce an index the server
rejects. This is a disconnect guard, not a cosmetic one: rAthena's
`clif_parse_NpcSelectMenu` calls `clif_GM_kick` when the index is `0` or greater
than the number of items the script offered.

## Preconditions
- As UC-206

## Test Steps

### Cancel closes without a kick
1. Launch with `--autologin --trace=npc,net`
2. Talk to the Guide until a menu appears
3. Cancel it (`Close` button / cancel affordance)
4. Verify `CZ_CHOOSE_MENU` is sent with `select = 255`
5. Verify the window closes
6. Verify the client is **still connected** and the character can move
7. Verify the server log (`make server-logs`) shows no "Invalid menu selection" warning

### Out-of-range is refused before it reaches the wire
1. Drive the dialog state machine in a unit test with an index of `0`
2. Verify the encoder refuses it and no packet is produced
3. Verify a **warn** log names the index and the item count
4. Repeat with an index greater than the item count — same refusal
5. Verify `255` is accepted, since it is the cancel sentinel and not an item index

### Rapid interaction
1. Open a menu and click a choice several times quickly
2. Verify only one `CZ_CHOOSE_MENU` is sent per menu
3. Verify no disconnect

### Talking to an NPC we do not track
1. Trigger a dialog whose npcId is not in the entity registry (a fake NPC —
   rAthena sends one via `clif_sendfakenpc` when the id is not a nearby unit)
2. Verify the dialog still opens and is driven by the id in the packet
3. Verify no nil-lookup crash

## Expected Results
- Cancel sends 255 and closes cleanly
- Index 0 and out-of-range indices never reach the wire, and are logged at warn
- No disconnect under repeated or unusual interaction
- A dialog from an untracked NPC id works
