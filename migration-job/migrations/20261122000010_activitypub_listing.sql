-- Whether somebody on the fediverse is listed in other servers' directories
-- and found by their search: their own two choices, on until they say
-- otherwise (the user, 2026-11-01).

-- +goose Up
ALTER TABLE activitypub_accounts
    ADD COLUMN `discoverable` tinyint(1) NOT NULL DEFAULT 1 AFTER `enabled_at`,
    ADD COLUMN `indexable` tinyint(1) NOT NULL DEFAULT 1 AFTER `discoverable`;

-- +goose Down
ALTER TABLE activitypub_accounts
    DROP COLUMN `indexable`,
    DROP COLUMN `discoverable`;
