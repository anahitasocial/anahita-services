-- When a notification was read, in place of whether (roadmap row 97, part
-- 1): NULL while it is unread. A read one is kept for so many days after it
-- was read (NOTIFICATIONS_KEEP_DAYS), which needs the time. One read before
-- this takes the time it was last changed, which is when it was marked.

-- +goose Up
ALTER TABLE notifications
    ADD COLUMN `read_at` datetime DEFAULT NULL AFTER `updated_at`;

UPDATE notifications SET read_at = updated_at WHERE is_read = 1;

ALTER TABLE notifications
    DROP INDEX `recipient_unread`,
    DROP COLUMN `is_read`,
    ADD KEY `recipient_unread` (`recipient_id`, `read_at`),
    ADD KEY `read_at` (`read_at`);

-- +goose Down
ALTER TABLE notifications
    ADD COLUMN `is_read` tinyint(1) NOT NULL DEFAULT 0 AFTER `updated_at`;

UPDATE notifications SET is_read = 1 WHERE read_at IS NOT NULL;

ALTER TABLE notifications
    DROP INDEX `read_at`,
    DROP INDEX `recipient_unread`,
    DROP COLUMN `read_at`,
    ADD KEY `recipient_unread` (`recipient_id`, `is_read`);
