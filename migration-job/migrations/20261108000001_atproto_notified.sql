-- AT Protocol: how far each connected account has been read for what
-- people on Bluesky did to what was written from here.
--
--   atproto_accounts.notified_at   the time of the last such thing dealt
--                                  with. Set to now when an account is
--                                  connected: what happened before is
--                                  not gone back for.
--
-- GUARDED, for the same reason as the migrations before it.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_add_atproto_notified()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'atproto_accounts' AND COLUMN_NAME = 'notified_at'
    ) THEN
        ALTER TABLE atproto_accounts ADD COLUMN `notified_at` datetime(3) DEFAULT NULL AFTER `broken_at`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_add_atproto_notified();
DROP PROCEDURE anahita_add_atproto_notified;

-- +goose Down
-- +goose StatementBegin
CREATE PROCEDURE anahita_drop_atproto_notified()
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'atproto_accounts' AND COLUMN_NAME = 'notified_at'
    ) THEN
        ALTER TABLE atproto_accounts DROP COLUMN `notified_at`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_drop_atproto_notified();
DROP PROCEDURE anahita_drop_atproto_notified;
