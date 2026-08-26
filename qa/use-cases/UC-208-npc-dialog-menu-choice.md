# UC-208: NPC Dialog — Menu Choice

## Description
An NPC that offers choices (`ZC_MENU_LIST`) shows them as a list; picking one
sends its **1-based** index and the script branches accordingly.

## Preconditions
- As UC-207
- An NPC with a menu in reach — Prontera's Guide offers a destination list

## Test Steps

### The menu appears
1. Launch with `--autologin --trace=npc,net`
2. Talk to the Guide until it offers choices
3. Verify `npc.menu` appears in the trace with the item count
4. Verify the window lists each item as a separate, selectable row
5. Verify the item count matches the number of `:`-separated entries in the packet

### Choosing branches the script
1. Select the second item in the list
2. Verify `npc.choose` reports index **2**, not 1 — the wire index is 1-based
3. Verify `CZ_CHOOSE_MENU` carries `select = 2`
4. Verify the script continues down the branch that item names
5. Verify the client is still connected

### Every item is reachable
1. Repeat for the first and last items in the list
2. Verify each sends its own position and branches correctly
3. Verify the last item does not send an index greater than the item count

## Expected Results
- Menu items are listed one per row and all are selectable
- The index sent is the item's 1-based position
- The script branches to match the choice
- No disconnect
