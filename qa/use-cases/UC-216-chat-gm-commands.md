# UC-216: Chat — `@` GM commands and `#` char commands

## Description
The test account is a GM (group 99), and `@` commands typed into the chat box
reach the server and answer into the box. `@` commands travel as ordinary chat
text — no dedicated packet — so this also proves the account change on its own.

## Preconditions
- Local rAthena stack running (`make server-up`)
- `make server-gm` applied (`login.group_id = 99` for `midgard-test`)
- Character `MidgardTest` on `prontera 156,191`

## Test Steps

### The account really is a GM
1. `docker exec midgard-rathena-db mariadb -uragnarok -pragnarok ragnarok \
   -e "SELECT group_id FROM login WHERE userid='midgard-test';"`
2. Verify it reports `99`
3. Verify `make server-player` sets it back to `0`, and `make server-gm` back to `99`
4. Verify neither needs a DB wipe or a server restart to take effect on the next login

### The command list comes back
1. Launch: `./build/midgard --autologin --trace=cmd,hud --say "@commands" --screenshot-after 20s`
2. Verify the chat box shows `Commands available:` followed by the list
3. Verify the list is long (the server has 318 commands) and the box scrolls
4. Verify `cmd.server` appears in the trace with the line that was sent
5. **Known limitation:** the server pads names into 10-character columns
   assuming a fixed-width font, and ours is proportional — verify the list is
   ragged rather than aligned, and that this is not reported as a defect

### A command that changes the world
1. `--say "@go 1"`
2. Verify the loading screen appears and Morocc loads (`@go 1` is Morocc;
   Geffen is `@go 2` — the numbering is in `atcommand.cpp` ACMD_FUNC(go))
3. Verify the character is in Morocc and can walk
4. `--say "@go 0"` and verify the return to Prontera

### A command that answers with text
1. `--say "@where MidgardTest"` and verify the map name and coordinates appear
   in the box. The name is required: a bare `@where` answers with the usage
   line and `@where failed.`, which is the server refusing it, not a client bug
2. `--say "@rates"` and verify the experience and drop rates appear
3. Verify the `@rates` lines arrive on `0x02C1` (`--trace=net`) and are coloured
   from the packet rather than from the line kind

### The whisper field must not swallow a command
1. Put a name in the chat's name field
2. `--say "@where"` with that field filled
3. Verify the command still runs — it is sent as public chat, not as a whisper
4. Verify no whisper acknowledgement arrives

### `#` char commands
1. `--say "#zeny MidgardTest 100"`
2. Verify Zeny in the Basic Info panel increases by 100
3. Verify the server's confirmation line appears in the box

### A non-GM's command is broadcast
1. `make server-player`, relaunch, `--say "@mapmove prontera 100 100"`
2. Verify the command does **not** run
3. Verify the literal text `@mapmove prontera 100 100` appears in the chat box
   as an ordinary spoken line — this is the original's behaviour and is
   deliberate, not a defect
4. Verify an info-level log notes that the command was probably broadcast
5. `make server-gm` again and verify the same line now moves the character

## Expected Results
- `make server-gm` / `server-player` flip the account without a DB wipe
- `@commands`, `@go`, `@where`, `@rates` all work as a GM
- `#` commands work and act on the named character
- A filled whisper field never turns a command into a private message
- As a non-GM, a denied command is broadcast rather than silently swallowed
