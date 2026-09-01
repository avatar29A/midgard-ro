# UC-220: Character creation — race, sex, hair style and hair colour

## Description
Every appearance choice the wire can carry must work on the screen and survive
into the game: race (Human or Doram), sex, one of 23 hair styles, one of 10
hair colours.

Our server accepts both Novice and Summoner at creation — it is a RENEWAL
build at `PACKETVER_RE 20211103`, so `allowed_job_flag = 3`
(`char/char.cpp:2814-2818`). Sex is per character, sent by the client, and only
male or female are accepted (`char/char.cpp:1424-1434`).

Hair colour needs external `.pal` support, which the engine does not have
today — this use case is what proves it was added.

## Preconditions
- Local rAthena stack running (`make server-up`)
- An account with a free slot; see `docs/TEST_ACCOUNTS.md`
- `--stop-at charselect` available (Step 0b)

## Test Steps

### Sex
1. Launch: `./build/midgard --stop-at charselect --trace=char --screenshot-after 8s`
2. Double-click an empty slot
3. Verify the ♂/♀ toggle shows which sex is selected
4. Click ♀ — verify the preview sprite becomes female
5. Click ♂ — verify it becomes male
6. Verify the created character ends up with the sex that was selected

### Race
1. Select the **Human** card — verify the preview is a Novice body
2. Select the **Doram** card — verify the preview is a Doram body
3. Verify Doram is selectable rather than "coming soon" (our server allows it)
4. Verify the job sent is Novice for Human and Summoner for Doram

### Hair style
1. Verify the grid shows **23** thumbnails, and that they change with sex
2. Click several cells — verify the preview's hair changes to match each
3. Verify the selected cell is visibly marked
4. Verify every one of the 23 renders a sprite (none blank or missing)

### Hair colour
1. Verify **10** swatches, the first being "default" (crossed through)
2. Click each — verify the preview's hair recolours
3. Verify the colour applies to the hair only, not the body or clothes
4. Create the character and enter the game
5. Verify the character in Prontera has **the same hair style and colour** as
   the preview showed

### Turn arrows
1. Click the left and right arrows
2. Verify the preview rotates through its directions and the hair follows

## Expected Result
Race, sex, hair style and hair colour all change the preview, and all four
survive creation into the game.

## Notes
Hair sprites live at `data/sprite/인간족/머리통/<sex>/<style>_<sex>.spr` — 42
styles exist per sex, of which 23 have thumbnails. Palettes are external
`.pal` files, 9 colour ids (0–8) per sex, which is why 10 swatches means
"default plus nine".
