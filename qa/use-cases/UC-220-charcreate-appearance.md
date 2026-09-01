# UC-220: Character creation — sex from the account, hair from the arrows

## Description
The creation screen takes the account's sex rather than asking for it, and the
hair arrows cycle style and colour with the preview following.

Account sex already reaches the client — `client.Session()` returns it and
`CH_ENTER` already carries it (`charselect.go:126`).

## Preconditions
- Local rAthena stack running
- Two accounts of different sex: `midgard-sword` (M) and `midgard-mage` (F)
  — see `docs/TEST_ACCOUNTS.md`

## Test Steps

### The sprite matches the account, male
1. Launch: `./build/midgard --username midgard-sword --password midgard-sword --stop-at charselect --screenshot-after 8s`
2. Double-click an empty slot
3. Verify the preview sprite is **male**
4. Verify it stands on the shadow ellipse painted into the background

### The sprite matches the account, female
1. Repeat with `--username midgard-mage --password midgard-mage`
2. Verify the preview sprite is **female**
3. Verify no control on the screen offers to change sex

### Hair style and colour
1. Click the left/right arrows either side of the sprite
2. Verify the hair style changes and the preview redraws each time
3. Verify cycling past the last style wraps to the first
4. Click the arrow above the sprite
5. Verify the hair colour changes and wraps the same way
6. Verify style and colour are independent — changing one leaves the other

## Expected Result
Sex is taken, never asked. Hair style and colour cycle and the preview follows
both.

## Notes
The sex byte is sent back in `0x0A39` unchanged. The server accepts only male
or female and refuses anything else (`char/char.cpp:1424-1434`), so a wrong
value here surfaces as a `0xFF` refusal, not a wrong sprite.
