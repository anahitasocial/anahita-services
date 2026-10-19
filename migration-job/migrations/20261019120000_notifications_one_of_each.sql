-- One notification for one thing (roadmap row 96, part 3): a repeated event
-- makes no second row. The key reads a missing subject, object, target or
-- reply as 0, since MySQL lets NULLs repeat in a unique key.
--
-- The notifications there are go first: none is worth keeping, and finding
-- the duplicates among hundreds of thousands takes many minutes (the user,
-- 2026-10-19).
--
-- With it: an index for the unread count and one for the list, comment_id
-- as wide as every other id, and meta, never read or written, gone.

-- +goose Up
TRUNCATE TABLE notifications;

ALTER TABLE notifications
    MODIFY `comment_id` bigint unsigned DEFAULT NULL,
    DROP COLUMN `meta`,
    DROP INDEX `recipient_id`,
    ADD KEY `recipient_unread` (`recipient_id`, `is_read`),
    ADD KEY `recipient_created` (`recipient_id`, `created_at`),
    ADD UNIQUE KEY `one_of_each` (`recipient_id`, `type`,
        (IFNULL(`subject_id`, 0)), (IFNULL(`object_id`, 0)),
        (IFNULL(`target_id`, 0)), (IFNULL(`comment_id`, 0)));

-- +goose Down
ALTER TABLE notifications
    DROP INDEX `one_of_each`,
    DROP INDEX `recipient_created`,
    DROP INDEX `recipient_unread`,
    ADD KEY `recipient_id` (`recipient_id`),
    ADD COLUMN `meta` mediumtext NOT NULL DEFAULT ('') AFTER `updated_at`,
    MODIFY `comment_id` int(11) unsigned DEFAULT NULL;
