-- MVP test account + pre-created Novice character.
-- Filename `zzz_*.sql` so MariaDB auto-init runs this AFTER upstream rAthena
-- schema files (main.sql, logs.sql) in alphabetical order.
-- See RFC #49 (https://github.com/avatar29A/midgard-ro/issues/49) and
-- docs/research/rathena-setup.md.

USE ragnarok;

-- Account: midgard-test / midgard-test
-- account_id starts at 2000000 per rAthena's main.sql AUTO_INCREMENT.
INSERT INTO `login` (
    `account_id`, `userid`, `user_pass`, `sex`, `email`,
    `group_id`, `state`, `character_slots`
) VALUES (
    2000000, 'midgard-test', 'midgard-test', 'M',
    'midgard-test@example.com', 0, 0, 9
)
ON DUPLICATE KEY UPDATE `userid` = VALUES(`userid`);

-- Pre-created Novice character on slot 0, spawned in Prontera.
-- Class 0 = Novice. Stats are all 1 (no points spent). HP/SP at level-1 baseline.
-- last_map / save_map = 'prontera'; coords are the standard town spawn.
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
    150000, 2000000, 0, 'MidgardTest', 0,
    1, 1,
    1, 1, 1, 1, 1, 1,
    40, 40, 11, 11,
    1, 1,
    'prontera', 156, 191,
    'prontera', 156, 191,
    'M'
)
ON DUPLICATE KEY UPDATE `name` = VALUES(`name`);
