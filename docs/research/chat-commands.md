# Chat commands — the complete reference

Every command reachable from the chat box: `/` user commands handled by the
client, `@` GM commands handled by the server, and `#` commands that act on
another character.

**Server:** rAthena at `5addd72448e2da3af6c89a97ccb994e5d49d8263`
(`docker/rathena/pin.txt`), PACKETVER **20211103**.
**Supporting work:** #94 (`docs/features/chat-commands/README.md`).
**Searchable version:** [`chat-commands.html`](./chat-commands.html) — the same
data as a filterable page, which beats scrolling 386 rows. Open it straight from
the repository, or read it at
<https://claude.ai/code/artifact/23c22c8b-9641-47e3-99aa-acf57b677751>.

The `@` tables are **generated** from the server tree rather than typed, so they
cannot drift from the server we actually run:

| Table | Source |
|-------|--------|
| Command names, aliases, syntax, effect | `conf/atcommands.yml` |
| Which group may use what | `conf/groups.yml`, resolved through `Inherit` |
| Cross-check that the list is real | `src/map/atcommand.cpp` registration table |

313 commands are registered in `atcommand.cpp` and 313 distinct commands are
documented in `atcommands.yml` — **the two agree exactly**, neither has an entry
the other lacks. (`atcommands.yml` lists `refineui` twice, an upstream
duplicate; it is counted once here.)

---

## 1. How a command reaches the server

Three rules, and they are not symmetric. This is the single most important thing
on this page.

### `@` and `#` travel as ordinary chat

There is **no command packet**. The client sends the line as a normal
`CZ_REQUEST_CHAT` (`0x00F3`) and the server pulls it out before broadcasting —
`clif_process_message` calls `is_atcommand()` on the message body and, when that
returns true, drops the line instead of passing it on
(`src/map/clif.cpp:10554`).

Two consequences worth knowing:

- **A command sent as a whisper is never seen.** Only `clif_process_message` —
  the public-chat path — checks for commands. `clif_parse_WisMessage` does not.
- **A command you are not allowed to use gets broadcast.** When the group check
  fails, `is_atcommand` returns false and the line carries on as ordinary
  speech, so a normal player typing `@mapmove prontera 100 100` shouts it at
  the whole map. This is the original's behaviour, not a bug.

### `/` never reaches the server at all

Nothing server-side parses a leading `/`. The client owns them: it either
answers locally, or sends a **dedicated packet** which the server converts
*back* into an atcommand — `/mm` becomes `@mapmove` (`clif.cpp:11558`), `/b`
becomes `@kami` (`clif.cpp:11994`), `/lb` becomes `@lkami` (`clif.cpp:13610`).

So sending `/where` as chat text does not run anything; it says "/where" to
everyone in range.

### The symbols are configurable

`@` and `#` are `atcommand_symbol` and `charcommand_symbol`
(`src/map/atcommand.cpp:89-90`), settable per server. Ours are the defaults.

---

## 2. `/` — user commands

These are the client's own. Ours implements none of them yet; the **Status**
column says what #94 plans.

Sources: roBrowser's `src/Controls/ProcessCommand.js` is a transcription of the
original client's handler and is authoritative for behaviour; the packet ids
come from `clif_packetdb.hpp` resolved at our PACKETVER; the wider inventory
(commands roBrowser does not implement) is from
[onlinegamecommands.com](https://www.onlinegamecommands.com/ragnarok-slash-commands/)
and is marked as such.

### 2a. Answered by the client — no packet at all

| Command | Syntax | Effect | Status in #94 |
|---------|--------|--------|---------------|
| `/where` | `/where` | Prints the current map name and the character's cell. | **Step 3** |
| `/h`, `/help` | `/h` | Lists the `/` commands the client implements. | **Step 3** |
| `/bgm`, `/music` | `/bgm` | Toggles background music, and says which way it went. | **Step 3** |
| `/sound` | `/sound` | Toggles sound effects. | **Step 3** |
| `/bv` | `/bv <0-127>` | Sets BGM volume. | out of scope |
| `/v` | `/v <0-127>` | Sets sound-effect volume. | out of scope |
| `/effect` | `/effect` | Toggles spell and gate effects. | out of scope — we render none |
| `/mineffect`, `/minimize` | `/mineffect` | Reduces effects to a minimum. | out of scope |
| `/fog` | `/fog` | Toggles fog. | out of scope |
| `/lightmap` | `/lightmap` | Toggles light-map effects. | out of scope |
| `/quake` | `/quake` | Toggles the screen-shake effect. | out of scope |
| `/miss` | `/miss` | Toggles miss notifications. | out of scope |
| `/skillfail` | `/skillfail` | Shows a red line when a skill fails. | out of scope |
| `/camera` | `/camera` | Toggles smooth camera movement. | out of scope |
| `/skip` | `/skip` | Toggles frame skipping. | out of scope |
| `/noctrl`, `/nc` | `/nc` | Auto-attack without holding Ctrl. | out of scope — no combat |
| `/noshift`, `/ns` | `/ns` | Cast on a target without holding Shift. | out of scope |
| `/notrade`, `/nt` | `/nt` | Declines trade offers automatically. | out of scope |
| `/showname` | `/showname` | Draws character names in a different font. | out of scope |
| `/monsterhp` | `/monsterhp` | Toggles monster health bars. | out of scope |
| `/stateinfo` | `/stateinfo` | Toggles status-icon descriptions. | out of scope |
| `/itemsnap`, `/skillsnap`, `/snap` | `/snap` | Toggle click-snapping for items, skills, fights. | out of scope |
| `/window` | `/window` | Windows snap and dock together. | out of scope |
| `/savechat` | `/savechat` | Writes the message log to a text file. | out of scope |
| `/notalkmsg`, `/notalkmsg2` | `/notalkmsg` | Hides chat in the chat window. | out of scope |
| `/battlechat` | `/battlechat` | Routes everything to the battle tab. | out of scope |
| `/tip` | `/tip` | Opens Tip of the Day. | out of scope |
| `/buildinfo` | `/buildinfo` | Prints the client build version. | out of scope |
| `/q1`, `/q2`, `/q3` | `/q1` | Quick-spell bindings for the hotkey rows. | out of scope — needs hotkeys |
| `/battlemode`, `/bm` | `/bm` | Skills from hotkey row 2 on the Q–O keys. | out of scope |
| `/set1` | `/set1` | Shorthand for `/noctrl` + `/showname` + `/skillfail`. | out of scope |
| `/aura`, `/aura2` | `/aura` | Reduces or disables the level-99 aura. | out of scope |
| `/emblem` | `/emblem` | Toggles the guild emblem. | out of scope |
| `/font` | `/font` | Moves the description above or below the name. | out of scope |
| `/navi`, `/navi2` | `/navi <map> <x>/<y>` | Navigation system through portals. | out of scope — needs the minimap route layer |

### 2b. Answered by the client, but with a packet

| Command | Syntax | Packet (ours) | Effect | Status in #94 |
|---------|--------|---------------|--------|---------------|
| `/w`, `/who` | `/who` | `CZ_REQ_USER_COUNT` `0x00C1`, 2B → `ZC_USER_COUNT` `0x00C2`, 6B | Shows how many players are online. | **Step 3** |
| `/sit`, `/stand` | `/sit` | `CZ_REQUEST_ACT`, action 2 / 3 | Sits or stands. | out of scope — no sit animation |
| `/memo` | `/memo` | `CZ_REMEMBER_WARPPOINT` | Memorises a warp point. | out of scope |
| `/doridori` | `/doridori` | `CZ_CHANGE_DIRECTION` | Shakes the head. | out of scope |
| `/bangbang` | `/bangbang` | `CZ_CHANGE_DIRECTION` | Turns the character clockwise. | out of scope |
| `/bingbing` | `/bingbing` | `CZ_CHANGE_DIRECTION` | Turns the character anticlockwise. | out of scope |
| `/!`, `/?`, `/ho`, `/lv`, … | `/!` | `CZ_REQ_EMOTION` `0x00BF`, 3B | Shows an emoticon. ~40 aliases in roBrowser's `DB/Emotions.js`. | out of scope — needs emotion sprites |
| `/str+`, `/agi+`, … | `/str+ <n>` | `CZ_STATUS_CHANGE` | Raises a stat by `<n>`. | out of scope — needs the stat window |
| `/in`, `/ex` | `/ex <char name>` | `CZ_SETTING_WHISPER_PC` `0x00CF`, 27B | Allows or blocks whispers from one character. | out of scope |
| `/inall`, `/exall` | `/exall` | `CZ_SETTING_WHISPER_STATE` `0x00D0` | Allows or blocks whispers from everyone. | out of scope |
| `/pvpinfo` | `/pvpinfo` | `CZ_REQ_PVPPOINT` `0x020F`, 10B | PvP points for the current character. | out of scope |
| `/blacksmith` | `/blacksmith` | `0x0217`, 2B | Top ten Blacksmiths. | out of scope |
| `/alchemist` | `/alchemist` | `0x0218`, 2B | Top ten Alchemists. | out of scope |
| `/taekwon` | `/taekwon` | `0x0225`, 2B | Top ten Taekwon Kids. | out of scope |
| `/pk` | `/pk` | `0x0237`, 2B | Top ten player killers. | out of scope |
| `/leave`, `/invite`, `/organize`, `/expel` | `/invite <char name>` | party packets | Party management. | out of scope — no party system |
| `/guild`, `/breakguild`, `/gc` | `/guild <name>` | guild packets | Guild management. | out of scope — no guild system |
| `/chat`, `/q` | `/chat` | chat-room packets | Creates or leaves a chat room. | out of scope |
| `/hi` | `/hi <message>` | friend packets | Greets online friends. | out of scope |
| `/deal` | `/deal` | trade packets | Opens a trade. | out of scope |

### 2c. GM `/` commands — a packet the server turns back into an atcommand

Every row here has an `@` equivalent that does the same thing. The packet is
still worth having: **the server rejects it silently when the account is not a
GM**, whereas sending the atcommand text would broadcast the command to the
whole map.

All ids resolved for PACKETVER 20211103 from `src/map/clif_packetdb.hpp`.

| Command | Syntax | Packet (ours) | Becomes | Effect | Status in #94 |
|---------|--------|---------------|---------|--------|---------------|
| `/mm`, `/mapmove` | `/mm <map> <x> <y>` | `CZ_MOVETO_MAP` `0x0140`, 22B | `@mapmove` | Warps to a map and cell. | **Step 4** |
| `/b`, `/nb` | `/b <message>` | `CZ_BROADCAST` `0x0099`, var | `@kami` | Server-wide announcement in yellow. | **Step 4** |
| `/lb`, `/nlb` | `/lb <message>` | `CZ_LOCALBROADCAST` `0x019C`, var | `@lkami` | Announcement on this map only. | **Step 4** |
| `/hide` | `/hide` | `0x019D`, 6B | `@hide` | Becomes invisible to players. | later |
| `/kill` | `/kill` (right-click menu) | `0x00CC`, 6B | `@kick` | Disconnects the target character. | later |
| `/killall` | `/killall` | `0x00CE`, 2B | `@kickall` | Disconnects everyone. | later |
| `/resetstate` | `/resetstate` | `CZ_RESET` `0x0197`, 4B, type 0 | `@resetstat` | Resets stat points. | later |
| `/resetskill` | `/resetskill` | `CZ_RESET` `0x0197`, 4B, type 1 | `@resetskill` | Resets skill points. | later |
| `/item`, `/monster` | `/item <name> <n>` | **`0x09CE`, 102B** | `@item` / `@monster` | Creates items or spawns monsters. | later |
| `/remove` | `/remove <account>` | `0x01BA`, 26B (or `0x0843`, 6B) | `@jumpto` | Warps you to a character. | later |
| `/shift` | `/shift <char name>` | `0x01BB`, 26B | `@jumpto` | Warps you to a character. | later |
| `/recall` | `/recall <char name>` | `0x01BC`, 26B (or `0x0842`, 6B) | `@recall` | Warps a character to you. | later |
| `/summon` | `/summon <char name>` | `0x01BD`, 26B | `@recall` | Warps a character to you. | later |
| `/rc` | `/rc <char name>` | `0x0212`, 26B | `@mute` | Mutes a character. | later |
| `/check` | `/check <char name>` | `0x0213`, 26B | — | Shows a character's stats. | later |
| `/changemaptype` | `/changemaptype <x> <y> <type>` | `0x0198`, 8B | — | Changes a cell's type. | later |
| `/resetcooltime` | `/resetcooltime` | `CZ_CMD_RESETCOOLTIME`, fixed | `@resetcooltime` | Clears skill cooldowns. | later |
| `/macrochecker` | `/macrochecker <mapname>` | `0x0C0B` | — | Runs the macro detector on a map. | later |

Two version traps, both real at our PACKETVER:

- **`/item` is `0x09CE` at 102 bytes, not `0x013F` at 26.** Both are registered;
  `0x09CE` is added for `PACKETVER >= 20131223` (`clif_packetdb.hpp:1675`) and
  ours is 20211103.
- **`/resetskill` does *not* use `CZ_RESET_SKILL` `0x0BB1`.** That registration
  is guarded by `PACKETVER_MAIN_NUM >= 20220216 || PACKETVER_ZERO_NUM >=
  20220203`, and we are `PACKETVER_RE`, so both are zero and the guard fails.
  `CZ_RESET` type 1 is what applies.

`tools/packetlen/gen.py` cannot confirm any of these: it reads only
`packet(...)` lines — the server→client table — and every command packet is
registered with `parseable_packet(...)`. Extending it is Step 0b of #94.

---

## 3. `@` — GM commands

### 3a. What each group can use

Our test account `midgard-test` is **group 0**, so today it can use exactly two
commands. #94 moves it to **group 99**, which carries the `all_commands`
permission and therefore reaches all 313 regardless of what any group lists
(`conf/groups.yml:236`; `s_player_group::can_use_command`,
`src/map/pc_groups.cpp:326`).

| Group | Id | Level | Inherits | Commands it adds | Total reachable |
|-------|----|-------|----------|------------------|-----------------|
| Player | 0 | 0 | — | 2 | 2 |
| Super Player | 1 | 0 | Player | 29 | 31 |
| VIP | 5 | 0 | Player | 2 | 4 |
| Support | 2 | 1 | Super Player | 12 | 43 |
| Script Manager | 3 | 1 | Support | 7 | 50 |
| Event Manager | 4 | 1 | Support | 26 | 69 |
| Law Enforcement | 10 | 2 | Support | 19 | 62 |
| Admin | 99 | 99 | Support, Law Enforcement | 0 | **313** (via `all_commands`) |

**Player (0)** — `@changedress`, `@resurrect`

**Super Player (1)** — `@alootid`, `@autoloot`, `@autoloottype`, `@autotrade`,
`@breakguild`, `@channel`, `@charcommands`, `@commands`, `@exp`, `@go`,
`@help`, `@hominfo`, `@homstats`, `@iteminfo`, `@jailtime`, `@langtype`,
`@mobinfo`, `@noask`, `@noks`, `@rates`, `@refresh`, `@request`,
`@servertime`, `@showdelay`, `@showexp`, `@showzeny`, `@uptime`, `@whereis`,
`@whodrops`

**VIP (5)** — `@rates`, `@who`

**Support (2)** — `@broadcast`, `@jumpto`, `@localbroadcast`, `@users`,
`@version`, `@where`, `@who`, `@who2`, `@who3`, `@whomap`, `@whomap2`,
`@whomap3`

**Script Manager (3)** — `@addwarp`, `@hidenpc`, `@loadnpc`, `@npcmove`,
`@shownpc`, `@tonpc`, `@unloadnpc`

**Event Manager (4)** — `@allowks`, `@cleanarea`, `@cleanmap`, `@day`,
`@disguise`, `@divorce`, `@gvgoff`, `@gvgon`, `@item`, `@killmonster2`,
`@marry`, `@me`, `@monster`, `@monsterbig`, `@monstersmall`, `@night`,
`@pvpoff`, `@pvpon`, `@raise`, `@raisemap`, `@refreshall`, `@size`,
`@skilloff`, `@skillon`, `@undisguise`, `@zeny`

**Law Enforcement (10)** — `@ban`, `@block`, `@cartlist`, `@disguise`,
`@fakename`, `@follow`, `@hide`, `@itemlist`, `@jail`, `@jailfor`, `@kick`,
`@kill`, `@mapmove`, `@mute`, `@option`, `@recall`, `@speed`, `@stats`,
`@storagelist`

**Admin (99)** — everything.

A command whose **Lowest group** below reads *Admin (99)* is one no group lists
explicitly; it is reachable only through the `all_commands` permission.

### 3b. The most useful ones for testing this client

Pulled out of the full table because these are the ones that set up a test
quickly. All are available at group 99.

| Command | Syntax | What it gives us |
|---------|--------|------------------|
| `@commands` | `@commands` | Lists everything the account may use — the runtime answer to "what does this server support". |
| `@help` | `@help <command>` | Syntax and description for one command, from the same `atcommands.yml` this page is built from. |
| `@go` | `@go <city name\|number>` | Jumps between towns by number or name — the fastest map-load test. |
| `@mapmove` | `@mapmove <mapname> [<x> <y>]` | Any map, any cell. Aliases `@rura`, `@warp`. |
| `@where` | `@where [<char name>]` | Map and coordinates — with no name, your own. |
| `@item` | `@item <item name or ID> <quantity>` | Fills the inventory for testing the item window. |
| `@monster` | `@monster <monster name\|ID> [<number to spawn>]` | Spawns mobs where you stand. |
| `@zeny` | `@zeny <amount>` | Exercises the Zeny field in the Basic Info panel. |
| `@heal` | `@heal [<HP> <SP>]` | Moves HP/SP to test the gauges without waiting for regen. |
| `@speed` | `@speed <1-1000>` | Walk speed — useful for movement-prediction work. |
| `@jobchange` | `@jobchange <job>` | Sprite and job changes. |
| `@refresh` | `@refresh` | Re-sends your position and surroundings; clears desyncs. |
| `@rates` | `@rates` | Arrives on `0x02C1` with a colour, so it is the test case for colour decoding. |
| `@kami` | `@kami <message>` | Yellow broadcast — the `0x009A` path. |
| `@kamib` | `@kamib <message>` | Blue broadcast — the same packet with a `"blue"` text prefix. |

### 3c. Every command

<!-- GENERATED — do not edit by hand. See "How to regenerate" at the foot. -->

| Command | Aliases | Syntax | Effect | Lowest group |
|---------|---------|--------|--------|--------------|
| `@accept` | — | `@accept` | Accepts an invitation to a duel. | Admin (99) |
| `@accinfo` | `accountinfo` | `@accinfo <char name>` / `@accinfo <account ID>` | Searches for a character with the name &lt;char name&gt;. You may use % as a placeholder. Searches login information for the account &lt;account ID&gt;. Displays basic information about… | Admin (99) |
| `@addfame` | `famepoint`, `famepoints` | `@addfame <amount>` | Adds or reduces the player's fame points by &lt;amount&gt;. | Admin (99) |
| `@addperm` | — | `@addperm <permission_name>` | Temporarily add a permission to a player. | Admin (99) |
| `@addwarp` | — | `@addwarp <map name> <x coord> <y coord> <NPC name>` | — | Script Manager (3) |
| `@adjgroup` | — | `@adjgroup <level> <char name>` | Do a temporary adjustment of the group level of a player. | Admin (99) |
| `@adopt` | — | `@adopt <char name>` | Adopts the specified player &lt;char name&gt;. | Admin (99) |
| `@agi` | — | `@agi <amount>` | Raises AGI by given amount. | Admin (99) |
| `@agitend` | — | `@agitend` | End War of Emperium | Admin (99) |
| `@agitend2` | — | `@agitend2` | End War of Emperium SE | Admin (99) |
| `@agitend3` | — | `@agitend3` | End War of Emperium TE | Admin (99) |
| `@agitstart` | — | `@agitstart` | Starts War of Emperium | Admin (99) |
| `@agitstart2` | — | `@agitstart2` | Starts War of Emperium SE | Admin (99) |
| `@agitstart3` | — | `@agitstart3` | Starts War of Emperium TE | Admin (99) |
| `@alive` | — | `@alive` | Revives yourself from death. | Admin (99) |
| `@allowks` | — | `@allowks` | Enables or disables kill stealing on this map. | Event Manager (4) |
| `@allskill` | `allskills`, `skillall`, `skillsall` | `@allskill` | Give you all skills. | Admin (99) |
| `@auction` | — | `@auction` | Opens the auction window. | Admin (99) |
| `@autoloot` | — | `@autoloot <on\|off\|#>` | Makes items go straight into your inventory. | Super Player (1) |
| `@autolootitem` | `alootid` | `@autolootitem None` / `@autolootitem +<Item ID> to add an item ID` / `@autolootitem -<Item ID> to remove an item ID` / `@autolootitem reset to remove all item IDs` | Shows a short help. Makes items of this specific item ID go straight into your inventory. | Admin (99) |
| `@autoloottype` | `aloottype` | `@autoloottype None` / `@autoloottype +<type name/ID> to add an item type` / `@autoloottype -<type name/ID> to remove an item type` / `@autoloottype reset to remove all item types` | Shows a short help. Makes items of this specific item type go straight into your inventory. Type List: healing = 0, usable = 2, etc = 3, weapon = 4, armor = 5, card = 6, petegg = 7,… | Super Player (1) |
| `@autotrade` | `at` | `@autotrade` | Allows you to vend while you are offline. | Super Player (1) |
| `@ban` | `banish` | `@ban <time> <name>\n" "Temporarily ban an account` | time usage: adjustment (+/- value) and element (y/a, m, d/j, h, mn, s) Example: @ban +1m-2mn1s-6y testplayer | Law Enforcement (10) |
| `@baselevelup` | `baselevel`, `baselvl`, `baselvup`, `baselvlup`, `blevel`, `blvl`, `lvup` | `@baselevelup <number of levels>` | Raises your base level the desired number of levels. | Admin (99) |
| `@bodystyle` | — | `@bodystyle <job ID>` / `@bodystyle off` | Restores default job bodystyle. Changes the character's bodystyle to &lt;job ID&gt;. | Admin (99) |
| `@breakguild` | — | `@breakguild` | Breaks the guild of the attached character. You must be the guildmaster to use this command. | Super Player (1) |
| `@broadcast` | — | `@broadcast <message>` | Broadcasts a message with your name (in yellow). | Support (2) |
| `@camerainfo` | `viewpointvalue`, `setcamera` | `@camerainfo` | Shows or updates the client's camera settings. | Admin (99) |
| `@cart` | — | `@cart <cart ID>` | Gives or removes a cart to a player and also change the cart skin. Available cart IDs: 0: remove cart 1-5: normal carts 6-9: new carts | Admin (99) |
| `@cartlist` | — | `@cartlist` | Displays a list of items in the cart. | Law Enforcement (10) |
| `@cash` | — | `@cash <amount> - Gives you the specified amount of cash points` | — | Admin (99) |
| `@changecharsex` | — | `@changecharsex` | Changes your character's gender. | Admin (99) |
| `@changedress` | `nocosplay` | `@changedress` | Removes all character costumes. | Player (0) |
| `@changegm` | — | `@changegm <charname>` | Changes the leader of your guild (You must be guild leader) | Admin (99) |
| `@changeleader` | — | `@changeleader <charname>` | Changes the leader of your party (You must be party leader) | Admin (99) |
| `@changelook` | — | `@changelook <position> <view ID>` | Changes the player's appearance to the specified view ID. Available positions: 1: Top 2: Middle 3: Bottom 4: Weapon 5: Shield 6: Shoes 7: Robe 8: Bodystyle | Admin (99) |
| `@changesex` | — | `@changesex` | Changes your account's gender. | Admin (99) |
| `@channel` | `main` | `@channel` | If you run this command without any parameters, you will get a more detailed help information. | Super Player (1) |
| `@char_ban` | `charban` | `@char_ban <time> <name>` | Temporarily ban a character. time usage: adjustment (+/- value) and element (y/a, m, d/j, h, mn, s) Example: @char_ban +1m-2mn1s-6y testplayer | Admin (99) |
| `@char_block` | `block` | `@char_block <char name>` | Permanently blocks an account. | Admin (99) |
| `@char_unban` | `charunban` | `@char_unban <name>` | Unban a character | Admin (99) |
| `@char_unblock` | `unblock` | `@char_unblock <char name>` | Unblocks an account. | Admin (99) |
| `@charcommands` | — | `@charcommands` | Displays a list of charcommands that you can use. | Super Player (1) |
| `@checkquest` | — | `@checkquest <quest ID>` | Shows status information for the quest with quest ID &lt;quest ID&gt;. | Admin (99) |
| `@clanspy` | — | `@clanspy <clan name\|id>` | You will receive all messages of the clan chat (Chat logging must be enabled) | Admin (99) |
| `@cleanarea` | `cleararea` | `@cleanarea` | Deletes floor items in sight range. | Event Manager (4) |
| `@cleanmap` | `clearmap` | `@cleanmap` | Deletes floor items on the current map. | Event Manager (4) |
| `@clearcart` | — | `@clearcart` | Deletes all items in the cart. | Admin (99) |
| `@cleargstorage` | — | `@cleargstorage` | Deletes all items in the guild storage. | Admin (99) |
| `@clearstorage` | — | `@clearstorage` | Deletes all items in the storage. | Admin (99) |
| `@clearweather` | — | `@clearweather` | Stops all weather effects on the current map. | Admin (99) |
| `@clone` | — | `@clone <charname>` | Spawns a supportive clone of the given player. | Admin (99) |
| `@cloneequip` | `eqclone` | `@cloneequip <char name>` / `@cloneequip <char ID>` | Copies the equipment of player &lt;char name/char ID&gt;. | Admin (99) |
| `@clonestat` | `stclone` | `@clonestat <char name>` / `@clonestat <char ID>` | Copies the status values of player &lt;char name/char ID&gt;. | Admin (99) |
| `@clouds` | — | `@clouds` | Makes all maps to have the cloudy weather effect. | Admin (99) |
| `@clouds2` | — | `@clouds2` | Makes all maps to have another cloudy weather effect. | Admin (99) |
| `@commands` | — | `@commands` | Displays a list of atcommands that you can use. | Super Player (1) |
| `@completequest` | — | `@completequest <quest ID>` | Completes the quest with quest ID &lt;quest ID&gt;. | Admin (99) |
| `@con` | — | `@con <amount>` | Raises CON by given amount. | Admin (99) |
| `@costume` | — | `@costume <costume>` | Changes the player's visible appearance to that of the selected &lt;costume&gt;. Available costumes: Hanbok Oktoberfest Summer Wedding Xmas | Admin (99) |
| `@crt` | — | `@crt <amount>` | Raises CRT by given amount. | Admin (99) |
| `@day` | — | `@day` | Disables night mode and restores regular lighting, all characters are affected. | Event Manager (4) |
| `@delitem` | — | `@delitem <item name> <amount>` / `@delitem <item ID> <amount>` | Deletes &lt;amount&gt; of the specified item &lt;item name/ID&gt; from the player's inventory. | Admin (99) |
| `@dex` | — | `@dex <amount>` | Raises DEX by given amount. | Admin (99) |
| `@disguise` | — | `@disguise <monster name\|ID>` | Change your appearence to other players to a mob. | Event Manager (4) |
| `@disguiseall` | — | `@disguiseall <monster name\|ID>` | Disguises all online characters. | Admin (99) |
| `@disguiseguild` | — | `@disguiseguild <monster name\|ID>` | Disguises all online characters of a guild. | Admin (99) |
| `@displayskill` | — | `@displayskill <skill ID> {<skill level>}` | Displays the skill animation of a skill without really using the skill. | Admin (99) |
| `@displayskillcast` | — | `@displayskillcast <skill ID> {<skill level> <ground target flag> <cast time>}` | Displays the cast animation of a skill without really casting the skill. | Admin (99) |
| `@displayskillunit` | — | `@displayskillunit <skill unit ID> {<skill level> <range>}` | Displays the skill unit animation of a skill unit without really using the skill. | Admin (99) |
| `@displaystatus` | — | `@displaystatus <status ID> <flag> <tick> {<val1> <val2> <val3>}` | Displays the status animation of a status change without really having the status change. | Admin (99) |
| `@divorce` | — | `@divorce` | Divorce player. | Event Manager (4) |
| `@doom` | — | `@doom` | Kills all NON GM chars on the server. | Admin (99) |
| `@doommap` | — | `@doommap` | Kills all non GM characters on the map. | Admin (99) |
| `@dropall` | — | `@dropall [<item type>]` | Throws all your possession on the ground. No type specified will drop all items. | Admin (99) |
| `@duel` | — | `@duel` | Starts a duel. | Admin (99) |
| `@dye` | `ccolor` | `@dye <clothes palette no.>` | Changes your characters clothes color. | Admin (99) |
| `@effect` | — | `@effect <effect id> [<flag>]` | Give an effect to your character. | Admin (99) |
| `@email` | — | `@email <current email> <new email>` | Changes your account e-mail address. | Admin (99) |
| `@enchantgradeui` | — | `@enchantgradeui` | Opens the enchantgrade UI. | Admin (99) |
| `@erasequest` | — | `@erasequest <quest ID>` | Removes the quest &lt;quest ID&gt; from the quest log. | Admin (99) |
| `@evilclone` | — | `@evilclone <charname>` | Spawns an aggressive clone of the given player. | Admin (99) |
| `@exp` | — | `@exp` | Displays current levels and % progress. | Super Player (1) |
| `@fakename` | — | `@fakename <name>` | Changes your name to your choice temporarily. | Law Enforcement (10) |
| `@feelreset` | — | `@feelreset` | Resets a Star Gladiator's marked maps. | Admin (99) |
| `@fireworks` | — | `@fireworks` | Makes all maps to have the fireworks weather effect. | Admin (99) |
| `@fog` | — | `@fog` | Makes all maps to have the fog weather effect. | Admin (99) |
| `@follow` | — | `@follow <char name>` | Follow a player. | Law Enforcement (10) |
| `@font` | — | `@font <type> - value between 0-9` | Sets the client font to &lt;type&gt;. Available types: 0: Default 1: RixLoveangel 2: RixSquirrel 3: NHCgogo 4: RixDiary 5: RixMiniHeart 6: RixFreshman 7: RixKid 8: RixMagic 9: RixJJangu | Admin (99) |
| `@fontcolor` | — | `@fontcolor <color_name>` | Sets channel chat font color for the invoking character only. | Admin (99) |
| `@fullstrip` | — | `@fullstrip <char name>` | Unequips all items currently equipped by &lt;char name&gt;. | Admin (99) |
| `@gat` | — | `@gat` | For debugging (you inspect around gat) | Admin (99) |
| `@gmotd` | — | `@gmotd` | Broadcasts the Message of The Day to all players. | Admin (99) |
| `@go` | — | `@go <city name\|number>` | Warps you to a city. -3: (Memo point 2) 14: louyang 31: mora -2: (Memo point 1) 15: start point 32: dewata -1: (Memo point 0) 16: prison/jail 33: malangdo island 0: prontera 17: jawaii… | Super Player (1) |
| `@grade` | — | `@grade <equip position> <+/- amount>` | — | Admin (99) |
| `@guild` | — | `@guild <guild_name>` | Create a guild. | Admin (99) |
| `@guildlevelup` | `glevel`, `glvl`, `guildlevel`, `guildlvl`, `guildlvlup`, `guildlvup` | `@guildlevelup <# of levels>` | Raise Guild by desired number of levels | Admin (99) |
| `@guildrecall` | — | `@guildrecall <guild name\|ID>` | Warps all online characters of a guild to you. | Admin (99) |
| `@guildspy` | — | `@guildspy <guild name\|id>` | You will receive all messages of the guild chat (Chat logging must be enabled) | Admin (99) |
| `@guildstorage` | `gstorage` | `@guildstorage` | Opens guild storage. | Admin (99) |
| `@gvgoff` | `gpvpoff` | `@gvgoff` | Disables GvG on the current map | Event Manager (4) |
| `@gvgon` | `gpvpon` | `@gvgon` | Enables GvG on the current map | Event Manager (4) |
| `@hair_color` | `haircolor`, `hcolor` | `@hair_color` | Params &lt;hair palette no.&gt; Changes your hair color. | Admin (99) |
| `@hair_style` | `hairstyle`, `hstyle` | `@hair_style <hairstyle no.>` | Changes your hair style. | Admin (99) |
| `@hatch` | — | `@hatch` | Create a pet from your inventory eggs list. | Admin (99) |
| `@hatereset` | — | `@hatereset` | Resets a Star Gladiator's marked monsters. | Admin (99) |
| `@heal` | — | `@heal [<HP> <SP>]` | Heals the desired amount of HP and SP. No value specified will do a full heal. | Admin (99) |
| `@healap` | — | `@healap [<AP>]` | Heals the desired amount of AP. No value specified will do a full AP heal. | Admin (99) |
| `@help` | `h` | `@help <command>` | Shows help for specified command. | Super Player (1) |
| `@hide` | — | `@hide` | Makes you character invisible (GM invisibility). Type again to become visible. | Law Enforcement (10) |
| `@hidenpc` | — | `@hidenpc <NPC name>` | Disable a NPC. | Script Manager (3) |
| `@homevolution` | `homevolve` | `@homevolution` | Evolves your homunculus, if possible. | Admin (99) |
| `@homfriendly` | — | `@homfriendly <level of intimacy> - value between 0-1000` | Sets your homunculus intimacy level to the desired value. | Admin (99) |
| `@homhungry` | — | `@homhungry <level of hunger> - value between 0-100` | Sets your homunculus hunger level to the desired value. | Admin (99) |
| `@hominfo` | — | `@hominfo` | Displays homunculus stats. | Super Player (1) |
| `@homlevel` | `hlvl`, `hlevel`, `homlvl`, `homlvup` | `@homlevel <level>` | Increases the homunculus level by &lt;level&gt;. | Admin (99) |
| `@hommutate` | — | `@hommutate <mutated homunculus ID>` | Mutates your homunculus to &lt;mutated homunculus ID&gt;, if possible. | Admin (99) |
| `@homshuffle` | — | `@homshuffle` | Recalculates the homunculus stats, as if the homunculus was leveled again from level 1. | Admin (99) |
| `@homstats` | — | `@homstats` | Displays homunculus stats. | Super Player (1) |
| `@homtalk` | — | `@homtalk <message>` | Let the player's homunculus say the text &lt;message&gt;. | Admin (99) |
| `@identify` | — | `@identify` | Opens the identification window if any unidentified items are in your inventory. | Admin (99) |
| `@identifyall` | — | `@identifyall` | Any unidentified items in your inventory will automatically be identified. | Admin (99) |
| `@idsearch` | — | `@idsearch <part_of_item_name>` | Search all items that name have part_of_item_name | Admin (99) |
| `@int` | — | `@int <amount>` | Raises INT by given amount. | Admin (99) |
| `@invite` | — | `@invite` | Invites a player to a duel. | Admin (99) |
| `@item` | — | `@item <item name or ID> <quantity>` | Gives you the desired item. | Event Manager (4) |
| `@item2` | — | `@item2 <item name or ID> <quantity> <identified_flag> <refine> <broken_flag> <Card1> <Card2> <Card3> <Card4>` | Gives you the desired item. | Admin (99) |
| `@itembound` | — | `@itembound <item name or ID> <quantity> <bound type>` | Creates an item bounded to the character. The items cannot be dropped, sold, vended, auctioned, or mailed, and in some cases cannot be traded or stored. Available bound types: 1: Account… | Admin (99) |
| `@itembound2` | — | `@itembound2 <item name or ID> <quantity> <identified_flag> <refine> <broken_flag> <Card1> <Card2> <Card3> <Card4> <bound type>` | Creates an item bounded to the character. The items cannot be dropped, sold, vended, auctioned, or mailed, and in some cases cannot be traded or stored. Available bound types: 1: Account… | Admin (99) |
| `@iteminfo` | `ii` | `@iteminfo <item name\|ID>` | Shows item info (type, price etc). | Super Player (1) |
| `@itemlist` | `inventorylist` | `@itemlist` | Displays a list of items in the inventory. | Law Enforcement (10) |
| `@itemreset` | `clearinventory` | `@itemreset` | Remove all your items. | Admin (99) |
| `@jail` | — | `@jail <char name>` | Sends specified character in jails. | Law Enforcement (10) |
| `@jailfor` | — | `@jailfor <time> <char name>` | Sends specified character in jails for the given &lt;time&gt;. | Law Enforcement (10) |
| `@jailtime` | — | `@jailtime` | Displays remaining jail time. | Super Player (1) |
| `@jobchange` | `job` | `@jobchange <job name\|ID>` | Changes your job. ----- Novice / 1st Class ----- 0 Novice 1 Swordman 2 Magician 3 Archer 4 Acolyte 5 Merchant 6 Thief ----- 2nd Class ----- 7 Knight 8 Priest 9 Wizard 10 Blacksmith 11… | Admin (99) |
| `@joblevelup` | `jlevel`, `jlvl`, `joblevel`, `joblvl`, `joblvlup`, `joblvup` | `@joblevelup <number of levels>` | Raises your job level the desired number of levels. | Admin (99) |
| `@join` | — | `@join <#channel_name> {<password>}` | Joins the specified channel &lt;#channel_name&gt;, if necessary by using the supplied &lt;password&gt;. | Admin (99) |
| `@jump` | — | `@jump [<x> [<y>]]` | Randomly warps you like a flywing. | Admin (99) |
| `@jumpto` | `goto`, `warpto` | `@jumpto <char name>` | Warps you to selected character. | Support (2) |
| `@kami` | — | `@kami <message>` | Broadcasts a message without your name (in yellow). | Admin (99) |
| `@kamib` | — | `@kamib <message>` | Broadcasts a message without your name (in blue). | Admin (99) |
| `@kamic` | — | `@kamic <color> <message> - color is a hexadecimal value` | Broadcasts a message without your name in the color &lt;color&gt;. | Admin (99) |
| `@kick` | — | `@kick <char name>` | Kicks specified character off the server | Law Enforcement (10) |
| `@kickall` | — | `@kickall` | Kick all characters off the server | Admin (99) |
| `@kill` | `die` | `@kill` | Kills player. | Law Enforcement (10) |
| `@killable` | — | `@killable` | Allows other players to attack you outside of PvP. | Admin (99) |
| `@killer` | — | `@killer` | Allows you to attack other players outside of PvP. | Admin (99) |
| `@killmonster` | — | `@killmonster <map>` | Kill all monsters of the map (they drop) | Admin (99) |
| `@killmonster2` | — | `@killmonster2` | Kills all monsters of your map (without drops). | Event Manager (4) |
| `@ksprotection` | `noks` | `@ksprotection None` / `@ksprotection self` / `@ksprotection party` / `@ksprotection guilds` | Disables kill stealing protection or displays a help message. Enables kill stealing protection against any other players. Enables kill stealing protection against any other players not… | Admin (99) |
| `@langtype` | — | `@langtype <language>` | Changes your language setting. | Super Player (1) |
| `@leave` | — | `@leave` | Leaves a duel. | Admin (99) |
| `@leaves` | — | `@leaves` | Makes all maps to have the leaves weather effect. | Admin (99) |
| `@limitedsale` | — | `@limitedsale` | Opens the limited sale window. | Admin (99) |
| `@lkami` | — | `@lkami <message>` | Broadcasts a message without your name on the current map (in yellow). | Admin (99) |
| `@load` | `return` | `@load` | Warps you to your save point. | Admin (99) |
| `@loadnpc` | — | `@loadnpc <path to script>` | Load the specified script file path. | Script Manager (3) |
| `@localbroadcast` | — | `@localbroadcast <message>` | Broadcasts a message with your name (in yellow) only on your map. | Support (2) |
| `@lostskill` | — | `@lostskill <#>` | Takes away the specified quest skill from you Novice = 142: First Aid, 143: Act Dead Archer = 147: Create Arrow, 148: Charge Arrow Swordman = 144: Moving HP Recovery, 145: Attack Weak… | Admin (99) |
| `@luk` | — | `@luk <amount>` | Raises LUK by given amount. | Admin (99) |
| `@macrochecker` | — | `@macrochecker <mapname>` | Trigger a macro detection on all players of the given map. | Admin (99) |
| `@mail` | — | `@mail` | Open mail box. | Admin (99) |
| `@makeegg` | — | `@makeegg <pet_id>` | Gives pet egg for monster number in pet DB | Admin (99) |
| `@makehomun` | — | `@makehomun <homunculus ID>` | Creates a homunculus with the given &lt;homunculus ID&gt;. | Admin (99) |
| `@mapexit` | — | `@mapexit` | Kick all players and shut down map-server. | Admin (99) |
| `@mapflag` | — | `@mapflag None - Shows mapflags that are active on the current map` / `@mapflag "available" - Shows a list of possible mapflags` / `@mapflag <name> - Activates mapflag <name> on the current map` | — | Admin (99) |
| `@mapinfo` | — | `@mapinfo [<0-3> [map]]` | Give information about a map (general info +: 0: no more, 1: players, 2: NPC, 3: chatrooms). | Admin (99) |
| `@mapmove` | `rura`, `warp` | `@mapmove <mapname> [<x> <y>]` | Warps you to the selected map and position. | Law Enforcement (10) |
| `@marry` | — | `@marry <player name>` | Marry another player. | Event Manager (4) |
| `@me` | — | `@me <message>` | Displays normal text as a message in this format: *name message* (like /me in mIRC). | Event Manager (4) |
| `@memo` | — | `@memo [memo position]` | Set/change a memo location (no position: display memo points). | Admin (99) |
| `@misceffect` | — | `@misceffect <effect ID>` | Does some visual effect on the character. Available effect IDs: 0 = base level up 1 = job level up 2 = refine failure 3 = refine success 4 = game over 5 = pharmacy success 6 = pharmacy… | Admin (99) |
| `@mobinfo` | `monsterinfo`, `mi` | `@mobinfo <monster name\|ID>` | Shows monster info (stats, exp, drops etc). | Super Player (1) |
| `@mobsearch` | — | `@mobsearch <monster name\|ID>` | Shows the location of a certain mob on the current map. | Admin (99) |
| `@model` | — | `@model <hair ID: 0-17> <hair color: 0-8> <clothes color: 0-4> - Changes your characters appearence` | — | Admin (99) |
| `@monster` | `spawn` | `@monster <monster name\|ID> [<number to spawn> [<desired_monster_name> [<x coord> [<y coord>]]]]` | @monster2 &lt;desired_monster_name&gt; &lt;monster name\|ID&gt; [&lt;number to spawn&gt; [&lt;x coord&gt; [&lt;y coord&gt;]]] @spawn/@monster/@summon/@monster2 "desired monster name"… | Event Manager (4) |
| `@monsterbig` | — | `@monsterbig <monster name\|ID>` | Spawns a larger version of a monster. | Event Manager (4) |
| `@monsterignore` | `battleignore` | `@monsterignore` | Makes the player unattackable by monsters, other players, etc. | Admin (99) |
| `@monstersmall` | — | `@monstersmall <monster name\|ID>` | Spawns a smaller version of a monster. | Event Manager (4) |
| `@mount2` | — | `@mount2` | Give/remove a cash mount. | Admin (99) |
| `@mount_peco` | `mount`, `mountpeco` | `@mount_peco` | Give/remove a job-based mount (class is required, but not the skill). | Admin (99) |
| `@mute` | — | `@mute <char name>` | Mutes the player &lt;char name&gt; (prevents talking, usage of skills, and commands). | Law Enforcement (10) |
| `@mutearea` | `stfu` | `@mutearea <time> amount of minutes to mute the players` | Mutes every player on screen for the specified time (prevents talking, usage of skills, and commands). | Admin (99) |
| `@night` | — | `@night` | Enables night mode on all maps, all characters are affected. | Event Manager (4) |
| `@noask` | — | `@noask` | Auto rejects deals/invites. | Super Player (1) |
| `@npcmove` | — | `@npcmove <x coord> <y coord> <NPC name>` | Move a NPC. | Script Manager (3) |
| `@npctalk` | `npctalkc` | `@npctalk <NPC name> <message>` | Forces a NPC to display a message in normal chat. | Admin (99) |
| `@nuke` | — | `@nuke <char name>` | Blow somebody up, including those surrounding them. | Admin (99) |
| `@option` | — | `@option <param1> <param2>(stackable) <param3>(stackable)` | Adds different visual effects on or around your character. &lt;param1&gt; &lt;param2&gt; &lt;param3&gt; 01: Stone 01: Sight 01: Sight 512: Cart Lv. 4 02: Frozen 02: Curse 02: Hiding… | Law Enforcement (10) |
| `@party` | — | `@party <party_name>` | Create a party. | Admin (99) |
| `@partyoption` | — | `@partyoption <item sharing> <item distribution> - yes/no` | Changes party options for item sharing and item distribution. | Admin (99) |
| `@partyrecall` | — | `@partyrecall <party name\|ID>` | Warps all online characters of a party to you. | Admin (99) |
| `@partysharelvl` | — | `@partysharelvl <level difference>` | Temporarily adjusts the party share level range to &lt;level difference&gt;. | Admin (99) |
| `@partyspy` | — | `@partyspy` | @partyspy &lt;party name\|id&gt; - You will receive all messages of the party channel (Chat logging must be enabled) | Admin (99) |
| `@petfriendly` | — | `@petfriendly <#>` | Set pet friendly amount (0-1000) 1000 = Max | Admin (99) |
| `@pethungry` | — | `@pethungry <#>` | Set pet hungry amount (0-100) 100 = Max | Admin (99) |
| `@petrename` | — | `@petrename` | Re-enable pet rename | Admin (99) |
| `@pettalk` | — | `@pettalk <message>` | Makes your pet say a message. | Admin (99) |
| `@points` | — | `@points <amount> - Gives you the specified amount of Kafra Points` | — | Admin (99) |
| `@pow` | — | `@pow <amount>` | Raises POW by given amount. | Admin (99) |
| `@produce` | — | `@produce <equip name or equip ID> <element> <# of very's>` | Element: 0=None 1=Ice 2=Earth 3=Fire 4=Wind You can add up to 3 Star Crumbs and 1 element | Admin (99) |
| `@pvpoff` | — | `@pvpoff` | Disables PvP on the current map | Event Manager (4) |
| `@pvpon` | — | `@pvpon` | Enables PvP on the current map | Event Manager (4) |
| `@questskill` | — | `@questskill <#>` | Gives you the specified quest skill Novice = 142: First Aid, 143: Act Dead Archer = 147: Create Arrow, 148: Charge Arrow Swordman = 144: Moving HP Recovery, 145: Attack Weak Point, 146:… | Admin (99) |
| `@raise` | `revive` | `@raise <char name>` | Revives target character. | Event Manager (4) |
| `@raisemap` | — | `@raisemap` | Resurrects all characters on the map. | Event Manager (4) |
| `@rates` | — | `@rates` | Displays the server's current rates. | Super Player (1) |
| `@recall` | — | `@recall <char name>` | Warps target character to you. | Law Enforcement (10) |
| `@recallall` | — | `@recallall` | Warps every character online to you. | Admin (99) |
| `@refine` | — | `@refine <equip position> <+/- amount>` | — | Admin (99) |
| `@refineui` | — | `@refineui` | Opens the refine UI. | Admin (99) |
| `@refresh` | — | `@refresh` | Synchronizes the position and state between client and server. | Super Player (1) |
| `@refreshall` | — | `@refreshall` | Synchronizes the position and state of all players between client and server. | Event Manager (4) |
| `@reject` | — | `@reject` | Automatically reject duel invitations. | Admin (99) |
| `@reload` | — | `@reload <type>` | Reload a database or a configuration file. itemdb mobdb skilldb atcommand battleconf statusdb pcdb motd script questdb msgconf packetdb cashdb logconf | Admin (99) |
| `@reloadachievementdb` | — | `@reloadachievementdb` | Reload achievement database. | Admin (99) |
| `@reloadatcommand` | — | `@reloadatcommand` | Reload atcommand settings. | Admin (99) |
| `@reloadattendancedb` | — | `@reloadattendancedb` | Reload attendance database. | Admin (99) |
| `@reloadbarterdb` | — | `@reloadbarterdb` | Reload the barter database. | Admin (99) |
| `@reloadbattleconf` | — | `@reloadbattleconf` | Reload battle settings. | Admin (99) |
| `@reloadcashdb` | `reloadcashshop` | `@reloadcashdb` | Reload cash shop database. | Admin (99) |
| `@reloadinstancedb` | — | `@reloadinstancedb` | Reload instance database. | Admin (99) |
| `@reloaditemdb` | — | `@reloaditemdb` | Reload item database. | Admin (99) |
| `@reloadlogconf` | — | `@reloadlogconf` | Reload the log settings. | Admin (99) |
| `@reloadmobdb` | — | `@reloadmobdb` | Reload monster database. | Admin (99) |
| `@reloadmotd` | — | `@reloadmotd` | Reload Message of the Day. | Admin (99) |
| `@reloadmsgconf` | — | `@reloadmsgconf` | Reload message configuration. | Admin (99) |
| `@reloadnpcfile` | `reloadnpc` | `@reloadnpcfile <path> - path to script` | Unloads and loads a script file from &lt;path&gt;. | Admin (99) |
| `@reloadpcdb` | — | `@reloadpcdb` | Reload player settings. | Admin (99) |
| `@reloadquestdb` | — | `@reloadquestdb` | Reload quest database. | Admin (99) |
| `@reloadscript` | — | `@reloadscript` | Reload all scripts. | Admin (99) |
| `@reloadskilldb` | — | `@reloadskilldb` | Reload skills definition database. | Admin (99) |
| `@reloadstatusdb` | — | `@reloadstatusdb` | Reload status settings. | Admin (99) |
| `@repairall` | — | `@repairall` | Repair all items of your inventory | Admin (99) |
| `@request` | — | `@request <message>` | Sends a message to all connected GMs (via the gm whisper system) | Super Player (1) |
| `@reset` | — | `@reset` | Resets the player's status and skill points. | Admin (99) |
| `@resetcooltime` | `resetcooldown` | `@resetcooltime` | Resets the cooldown of all skills of the player and if active also of the homunculus or the mercenary. | Admin (99) |
| `@resetskill` | `skreset` | `@resetskill` | Resets the player's skill points. | Admin (99) |
| `@resetstat` | `streset` | `@resetstat` | Resets the player's status points. | Admin (99) |
| `@resurrect` | — | `@resurrect` | Resurrects a player, if the necessary conditions (items in inventory or status changes) are fulfilled. | Player (0) |
| `@rmvperm` | — | `@rmvperm <permission_name>` | Temporarily remove a permission from a player. | Admin (99) |
| `@roulette` | — | `@roulette` | Opens the roulette UI. | Admin (99) |
| `@sakura` | — | `@sakura` | Makes all maps to have the sakura weather effect. | Admin (99) |
| `@save` | — | `@save` | Sets respawn point to current spot. | Admin (99) |
| `@send` | — | `@send <Hex Number> [<value>]` | For debugging (packet variety) | Admin (99) |
| `@servertime` | `date`, `serverdate`, `time` | `@servertime` | Shows the date and time of the server. | Super Player (1) |
| `@set` | — | `@set <variable name> {<value>}` | Shows the value of the variable &lt;variable name&gt;. If a &lt;value&gt; is provided, it changes the variable &lt;variable name&gt; to the given value. | Admin (99) |
| `@setbattleflag` | — | `@setbattleflag <battle config name> <value> {<reload>}` | Changes &lt;battle config name&gt; to &lt;value&gt; without rebooting the server. If &lt;reload&gt; is specified, the monster database will also be reloaded. | Admin (99) |
| `@setcard` | — | `@setcard` | Adds a card or enchant to the specific slot of the equipment. | Admin (99) |
| `@setquest` | — | `@setquest <quest ID>` | Activates the quest with quest ID &lt;quest ID&gt;. | Admin (99) |
| `@showdelay` | — | `@showdelay` | Shows/hides the "There is a delay after this skill" message. | Super Player (1) |
| `@showexp` | — | `@showexp` | Displays/hides experience gained. | Super Player (1) |
| `@showmobs` | — | `@showmobs <monster ID>` / `@showmobs <monster name>` | Locates and displays the position of a certain mob on your mini-map. This shows up as a small white cross (+). | Admin (99) |
| `@shownpc` | — | `@shownpc <NPC name>` | Enable a NPC. | Script Manager (3) |
| `@showrate` | — | `@showrate` | Enable or disable to show the rate information on every mapchange. | Admin (99) |
| `@showzeny` | — | `@showzeny` | Displays/hides Zeny gained. | Super Player (1) |
| `@size` | — | `@size <0-2> Changes your size (0-Normal 1-Small 2-Large)` | — | Event Manager (4) |
| `@sizeall` | — | `@sizeall` | Changes the size of all players. | Admin (99) |
| `@sizeguild` | — | `@sizeguild` | Changes the size of all online characters of a guild. | Admin (99) |
| `@skillid` | — | `@skillid <name>` | Look up a skill by name | Admin (99) |
| `@skilloff` | — | `@skilloff` | Turn skills off for a map. | Event Manager (4) |
| `@skillon` | — | `@skillon` | Turn skills on for a map. | Event Manager (4) |
| `@skillpoint` | `skpoint` | `@skillpoint <number of points> - Gives you the desired number of skill points` | — | Admin (99) |
| `@skilltree` | — | `@skilltree <skillnum> <charname>` | Prints the skill tree needed to get a skill for the target player. | Admin (99) |
| `@slaveclone` | — | `@slaveclone <charname>` | Spawns a supportive clone of the given player that follows the creator around. | Admin (99) |
| `@snow` | — | `@snow` | Makes all maps to have the snow weather effect. | Admin (99) |
| `@soulball` | — | `@soulball <amount> - value between 0-20` | Summons the specified &lt;amount&gt; of soul spheres around you. | Admin (99) |
| `@sound` | — | `@sound <path to file in data folder or GRF file>` | Plays a sound from the data folder or GRF file located on the client. | Admin (99) |
| `@speed` | — | `@speed <1-1000>` | Changes you walking speed. 1 being the fastest and 1000 the slowest. Default is 150. | Law Enforcement (10) |
| `@spiritball` | — | `@spiritball <1-100>` | Gives you "spirit spheres" like from the skill "Call Spirits". | Admin (99) |
| `@spl` | — | `@spl <amount>` | Raises SPL by given amount. | Admin (99) |
| `@sta` | — | `@sta <amount>` | Raises STA by given amount. | Admin (99) |
| `@stat_all` | `allstat`, `allstats`, `statall`, `statsall` | `@stat_all <value>` | Adds value in all stats (maximum if no value). | Admin (99) |
| `@stats` | — | `@stats` | Displays the stats of the player in your chat. | Law Enforcement (10) |
| `@statuspoint` | `stpoint` | `@statuspoint <number of points> - Gives you the desired number of stat points` | — | Admin (99) |
| `@stockall` | — | `@stockall [<item type>]` | Transfer items from cart to your inventory. No type specified will transfer all items. | Admin (99) |
| `@storage` | — | `@storage` | Opens storage. | Admin (99) |
| `@storagelist` | — | `@storagelist` | Displays a list of items in the storage. | Law Enforcement (10) |
| `@storeall` | — | `@storeall` | Puts all your possessions in storage. | Admin (99) |
| `@str` | — | `@str <amount>` | Raises STR by given amount. | Admin (99) |
| `@stylist` | — | `@stylist` | Opens the stylist user interface. | Admin (99) |
| `@summon` | — | `@summon <monster name/ID> {<duration>}` | Spawns the monster with &lt;monster name/ID&gt; and let it treat you as their master. If a duration is specified, it will stay with you until the duration has ended. | Admin (99) |
| `@tonpc` | — | `@tonpc <NPC name>` | Warps to the specified NPC. | Script Manager (3) |
| `@trade` | — | `@trade <char name> - Open a trade window with a another player` | — | Admin (99) |
| `@trait_all` | `alltrait`, `alltraits`, `traitall`, `traitsall` | `@trait_all <value>` | Adds value in all traits (maximum if no value). | Admin (99) |
| `@traitpoint` | `trpoint` | `@traitpoint <number of points> - Gives you the desired number of trait stat points` | — | Admin (99) |
| `@unban` | `unbanish` | `@unban <name> - Unban an account` | — | Admin (99) |
| `@undisguise` | — | `@undisguise` | Restore your normal appearance. | Event Manager (4) |
| `@undisguiseall` | — | `@undisguiseall` | Restore the normal appearance of all connected players. | Admin (99) |
| `@undisguiseguild` | — | `@undisguiseguild` | Restore the normal appearance of all characters of a guild. | Admin (99) |
| `@unjail` | `discharge` | `@unjail <char name>` | Discharges specified character/prisoner | Admin (99) |
| `@unloadnpc` | — | `@unloadnpc <NPC name>` | Unload the specified NPC according to name. | Script Manager (3) |
| `@unloadnpcfile` | — | `@unloadnpcfile <path>` | Unload the specified script file path. | Admin (99) |
| `@unmute` | — | `@unmute <char name>` | Unmutes the player &lt;char name&gt;. | Admin (99) |
| `@uptime` | — | `@uptime` | Displays how long the server has been online. | Super Player (1) |
| `@users` | — | `@users` | Displays the distribution of players on the server per map. | Support (2) |
| `@useskill` | — | `@useskill <skillid> <skillv> <target>` | Use a skill on target | Admin (99) |
| `@version` | — | `@version` | Displays SVN version of the server. | Support (2) |
| `@vip` | — | `@vip <+/- time> <char name>` | Set a player in VIP mode for a limited time. Time elements: y/a, m, d/j, h, mn, s | Admin (99) |
| `@vit` | — | `@vit <amount>` | Raises VIT by given amount. | Admin (99) |
| `@where` | — | `@where <char name>` | Tells you the location of a character. | Support (2) |
| `@whereis` | — | `@whereis <monster name/ID>` | Displays the maps in which monster &lt;monster name/ID&gt; normally spawns. | Super Player (1) |
| `@who` | `whois` | `@who [<name>]` | Shows a list of online players and their party and guild. | VIP (5) |
| `@who2` | — | `@who2 [<name>]` | Shows a list of online players and their job. | Support (2) |
| `@who3` | — | `@who3 [<name>]` | Shows a list of online players and their location. | Support (2) |
| `@whodrops` | — | `@whodrops <item name\|ID>` | Shows who drops an item (monster with highest drop rates). | Super Player (1) |
| `@whogm` | — | `@whogm [match_text] - Like @who+@who2+who3, but only for GM` | — | Admin (99) |
| `@whomap` | — | `@whomap <mapname>` | Like @who but only for specified map &lt;mapname&gt;. | Support (2) |
| `@whomap2` | — | `@whomap2 <mapname>` | Like @who2 but only for specified map &lt;mapname&gt;. | Support (2) |
| `@whomap3` | — | `@whomap3 <mapname>` | Like @who3 but only for specified map &lt;mapname&gt;. | Support (2) |
| `@wis` | — | `@wis <amount>` | Raises WIS by given amount. | Admin (99) |
| `@zeny` | — | `@zeny <amount> - Gives you desired amount of Zeny` | — | Event Manager (4) |

---

## 4. `#` — char commands

Same commands, aimed at another character:

```
#<command> <char name> <parameters>
```

Only groups that list `CharCommands` may use them, and only for the commands
listed there — the `@` list does not carry over.

- **Event Manager (4)** — `#disguise`, `#item`, `#size`, `#undisguise`, `#zeny`
- **Admin (99)** — all of them, via `all_commands`.
- Every other group — none.

`@charcommands` lists them at runtime, the way `@commands` does for `@`.

---

## 5. Things that will bite

- **`@commands` output will not line up.** The server pads names into
  10-character columns with spaces (`src/map/atcommand.cpp:10058`), assuming a
  fixed-width font. Ours is Arial Unicode (`internal/engine/ui2d/font.go:90`),
  so the list renders ragged. Cosmetic, and not a defect to report.
- **Command replies arrive as `ZC_NOTIFY_PLAYERCHAT` `0x008E`**
  (`clif.cpp:6697`), which the original paints **green**, not yellow — it maps
  to `PUBLIC|SELF` (roBrowser `Main.js:65`). Our client currently paints these
  yellow.
- **A handful of commands answer on `ZC_NPC_CHAT` `0x02C1` instead**
  (`@rates`, `@mobinfo`, `@iteminfo` — 7 call sites in `atcommand.cpp` against
  1035 for the plain path). That packet carries its colour as **BGR**, already
  swapped at startup (`clif.cpp:25863`). Reading it as RGB turns rAthena's
  light green `0xB5FFB5` into a light pink.
- **`ZC_BROADCAST` `0x009A` encodes colour as literal text at the front of the
  message** — `"blue"` for blue, `"ssss"` for the WoE style, anything else
  yellow (`clif.cpp:6722-6735`). Strip it, or `@kamib hi` renders as "bluehi".
- **A denied `@` command is broadcast**, as described in §1. Worth remembering
  before testing GM commands on a public server.

---

## How to regenerate

The `@` tables come from the server tree, which `make server-up` clones at the
pinned SHA. After a `make server-rebuild` onto a newer rAthena, re-run the
extractor to pick up new commands and changed permissions:

```bash
python3 tools/chatcmds/gen.py > /tmp/at-table.md   # then replace §3c below
python3 tools/chatcmds/build_page.py               # rewrites chat-commands.html
```

Both read the same rAthena tree, so they cannot disagree with each other. If the
counts in the header change, update them here too — and a warning that
`atcommands.yml` and `atcommand.cpp` disagree is worth investigating rather than
papering over, since it means the documentation and the server have diverged.

| File | What it is |
|------|------------|
| `tools/chatcmds/gen.py` | Reads the server tree; emits the markdown table below. |
| `tools/chatcmds/build_page.py` | Same data → `chat-commands.html`. |
| `tools/chatcmds/page.template.html` | Layout and filter UI for that page. Deliberately carries no `<!doctype>`/`<html>`/`<head>`/`<body>` so it also serves as the published artifact's source; `build_page.py` adds them for the standalone file. |
| `tools/chatcmds/slash.json` | The `/` commands. **Hand-maintained** — no single file on either side holds them, so this is the one part that can drift. |
