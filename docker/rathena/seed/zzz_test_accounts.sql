-- Two extra test accounts beyond the MVP Novice, for exercising job/sex
-- sprite variety and for running two clients side by side (each account can
-- only hold one session, so seeing another player render needs a second one).
--
-- Runs after `zzz_mvp_novice.sql` — MariaDB auto-init sorts alphabetically and
-- `zzz_mvp_*` < `zzz_test_*`.
--
-- Credentials are documented in docs/TEST_ACCOUNTS.md.

USE ragnarok;

-- Account 2: midgard-sword / midgard-sword — male Swordman.
INSERT INTO `login` (
    `account_id`, `userid`, `user_pass`, `sex`, `email`,
    `group_id`, `state`, `character_slots`
) VALUES (
    2000001, 'midgard-sword', 'midgard-sword', 'M',
    'midgard-sword@example.com', 0, 0, 9
)
ON DUPLICATE KEY UPDATE `userid` = VALUES(`userid`);

-- Class 1 = Swordman. Spawned a few cells west of the Novice so two clients
-- logged in at once don't stack on the same tile.
INSERT INTO `char` (
    `char_id`, `account_id`, `char_num`, `name`, `class`,
    `base_level`, `job_level`,
    `str`, `agi`, `vit`, `int`, `dex`, `luk`,
    `max_hp`, `hp`, `max_sp`, `sp`,
    `hair`, `hair_color`,
    `last_map`, `last_x`, `last_y`,
    `save_map`, `save_x`, `save_y`,
    `sex`
) VALUES (
    150001, 2000001, 0, 'MidgardSword', 1,
    10, 10,
    12, 6, 10, 1, 6, 1,
    200, 200, 20, 20,
    2, 3,
    'prontera', 152, 191,
    'prontera', 152, 191,
    'M'
)
ON DUPLICATE KEY UPDATE `name` = VALUES(`name`);

-- Account 3: midgard-mage / midgard-mage — female Mage.
INSERT INTO `login` (
    `account_id`, `userid`, `user_pass`, `sex`, `email`,
    `group_id`, `state`, `character_slots`
) VALUES (
    2000002, 'midgard-mage', 'midgard-mage', 'F',
    'midgard-mage@example.com', 0, 0, 9
)
ON DUPLICATE KEY UPDATE `userid` = VALUES(`userid`);

-- Class 2 = Mage. Female, so the client has to pick the `_f` sprite variant.
INSERT INTO `char` (
    `char_id`, `account_id`, `char_num`, `name`, `class`,
    `base_level`, `job_level`,
    `str`, `agi`, `vit`, `int`, `dex`, `luk`,
    `max_hp`, `hp`, `max_sp`, `sp`,
    `hair`, `hair_color`,
    `last_map`, `last_x`, `last_y`,
    `save_map`, `save_x`, `save_y`,
    `sex`
) VALUES (
    150002, 2000002, 0, 'MidgardMage', 2,
    10, 10,
    1, 6, 5, 14, 8, 1,
    80, 80, 90, 90,
    4, 6,
    'prontera', 160, 191,
    'prontera', 160, 191,
    'F'
)
ON DUPLICATE KEY UPDATE `name` = VALUES(`name`);
