# UC-219: Character creation — reaching the screen from an empty slot

## Description
An empty character slot must read as empty and open the creation screen on a
double-click. Cancelling must return to character select having sent nothing.

Today empty slots are skipped entirely (`charselect_native.go:215`), so they
draw nothing and answer nothing.

## Preconditions
- Local rAthena stack running (`make server-up`)
- An account with at least one free slot — all three test accounts qualify
  (one character each, `character_slots = 9`); see `docs/TEST_ACCOUNTS.md`
- `--stop-at charselect` available (Step 0b)

## Test Steps

### The slot reads as empty
1. Launch: `./build/midgard --username midgard-test --password midgard-test --stop-at charselect --trace=char --screenshot-after 8s`
2. Verify slot 0 shows `MidgardTest`
3. Verify slots 1 and 2 draw a slot frame with no character in them
4. Verify an empty slot is visibly distinguishable from an occupied one

### It answers a double-click
1. Double-click slot 1
2. Verify `char.slot-click` appears in the trace with `slot=1` and `empty=true`
3. Verify the creation screen opens
4. Verify its background matches `make_character_ver2/bg_back2.tga` — the
   Human and Doram race cards, the sex toggle, the preview area with turn
   arrows, the name field, the hair style grid and colour swatches, and the
   Go back / Create buttons

### Cancel sends nothing
1. Press **Go back**
2. Verify character select returns with slot 1 still empty
3. Verify `--trace=net` shows **no** `0x0A39` was sent
4. Verify the character count in the database is unchanged (`make server-shell-db`)

## Expected Result
Empty slots are visible and clickable; the creation screen opens on them and
leaves no trace behind when cancelled.

## Notes
Double-clicking an *occupied* slot must keep its current behaviour — entering
the game — and must not open creation.
