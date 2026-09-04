# Test Accounts

Accounts seeded into the local rAthena stack (`make server-up`). All three are
created by the SQL files in [`docker/rathena/seed/`](../docker/rathena/seed/)
and exist only in your own Docker volume — nothing here is a real credential.

Server: `127.0.0.1:6900` (login) — see [QUICKSTART.md](QUICKSTART.md).

## Accounts

| Login | Password | Job | Character | Sex | Level | Spawn |
|---|---|---|---|---|---|---|
| `midgard-test` | `midgard-test` | Novice (class 0), GM | `MidgardTest` | M | 1 / 1 | prontera (156, 191) |
| `midgard-sword` | `midgard-sword` | Swordman (class 1) | `MidgardSword` | M | 10 / 10 | prontera (152, 191) |
| `midgard-mage` | `midgard-mage` | Mage (class 2), GM | `MidgardMage` | F | 10 / 10 | prontera (160, 191) |

Each account has one pre-created character on slot 0 and 9 character slots
free for manual creation. "Spawn" is the seeded save point; once you play a
character the server moves its save point with it, and re-applying the seed
does not reset that (see [When the seed applies](#when-the-seed-applies)).

`midgard-test` is seeded as a GM (group 99, all `@` commands). `midgard-sword`
is an ordinary player (group 0). `midgard-mage` is seeded as a player but was
made a GM on the shared stack for screenshots and skill testing, and
`MidgardMage` has been levelled by hand:

| | seeded | on the shared stack (2026-09-04) |
|---|---|---|
| group | 0 | 99 (GM) |
| base / job level | 10 / 10 | 70 / 50 |
| AGI / VIT / INT / DEX | 6 / 5 / 14 / 8 | 36 / 45 / 74 / 58 |
| skills | none | every Mage skill at max (`@allskill`) |

That is the character to use when a screenshot or a test needs a working
caster: Frost Diver (skill 15) and Thunderstorm (21) both show their effects.
To make it again on a fresh DB, log in as `midgard-mage` after
`MIDGARD_ACCOUNT=midgard-mage make server-gm` and say `@blvl 60`, `@jlvl 40`,
`@allskill`, `@int 60`, `@dex 50`, `@vit 40`, `@agi 30` — or pass the same
lines as `--say` flags (see below).

The seeded HP/SP values are cosmetic — rAthena recalculates both from job,
level and VIT/INT when the character logs in, so `MidgardSword` shows 136 HP
in game rather than the 200 in the SQL.

## Why three

- **`midgard-test`** is the default in `config.example.yaml` — the happy path
  the QUICKSTART walks through, and the only one the MVP scope strictly needs.
- **`midgard-sword`** and **`midgard-mage`** exist for two reasons:
  - **Sprite variety.** Different `class` values, and a female character, force
    the client down different sprite-lookup paths than the male Novice.
  - **Two clients at once.** One account holds one session, so seeing another
    player render on your screen means logging a second account in from a
    second client. The three spawn points are a few cells apart in the Prontera
    plaza so the characters don't stack on one tile.

## Switching accounts

Pass `--username` / `--password` on the command line. They override whatever
`config.yaml` holds, for that run only, and are never written back to disk:

```bash
go run ./cmd/client --username midgard-sword --password midgard-sword
```

The login window pre-fills from them, so `--autologin` gets you straight into
Prontera as that character:

```bash
go run ./cmd/client --username midgard-mage --password midgard-mage --autologin
```

> A password in `argv` is visible to anything that can read `ps`. These flags
> are for the throwaway accounts on this page, not for real ones.

To run two clients side by side, build once and launch each with its own
account — no config juggling:

```bash
go build -o midgard ./cmd/client
./midgard --username midgard-sword --password midgard-sword --autologin --windowed &
./midgard --username midgard-mage  --password midgard-mage  --autologin --windowed &
```

Both characters spawn a few cells apart in the Prontera plaza, so each client
renders the other's player.

The persistent alternative is still the `network` block in `config.yaml`
(gitignored; created by `make config`), which sets the default account:

```yaml
network:
  login_server: "127.0.0.1:6900"
  username: "midgard-test"
  password: "midgard-test"
```

## When the seed applies

MariaDB runs the seed files **only on first initialisation of the DB volume**.
If your stack predates a seed change, the new accounts won't be there. Two ways
to get them:

```bash
# A. Wipe the DB and re-seed (loses any characters you created by hand)
make server-reset && make server-up
```

```bash
# B. Apply just the new seed against the running DB (keeps everything else)
docker exec -i midgard-rathena-db mariadb -uragnarok -pragnarok ragnarok \
    < docker/rathena/seed/zzz_test_accounts.sql
```

Both are safe to repeat — every statement in the seed is an
`INSERT ... ON DUPLICATE KEY UPDATE`.

Verify with:

```bash
make server-shell-db
> SELECT account_id, userid, sex FROM login;
> SELECT char_id, account_id, name, class, base_level FROM `char`;
```

## Adding more

Add the `login` + `char` pair to a `zzz_*.sql` file in
[`docker/rathena/seed/`](../docker/rathena/seed/), keeping `account_id` and
`char_id` above the table `AUTO_INCREMENT` starts (2000000 and 150000
respectively) and unique against what's already seeded. Then apply it with
option B above and add a row to the table here.
