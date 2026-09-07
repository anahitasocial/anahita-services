-- Adds tokens.new_email: the destination address for an emailchange
-- token. NULL on every other token type.
--
-- Without it the email-change flow cannot complete at all. The handler
-- writes Token.NewEmail on the way in and compares the person's re-typed
-- address against it on the way back; with nowhere to store it the value
-- was lost at insert and the comparison ran against an empty string.
--
-- GUARDED, and not out of superstition. entrypoint.sh runs schema/init.sql
-- on an empty database and exits WITHOUT recording anything in
-- goose_db_version, so the next start of that same database finds tables,
-- finds no legacy tables, and runs every migration here from zero against
-- a schema that already has them. A plain ALTER would fail on the second
-- container start of every freshly initialized deployment. The procedure
-- is created and dropped in the same migration so nothing is left behind.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_add_tokens_new_email()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'tokens'
          AND COLUMN_NAME = 'new_email'
    ) THEN
        ALTER TABLE tokens
            ADD COLUMN `new_email` varchar(255) DEFAULT NULL AFTER `recipient_email`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_add_tokens_new_email();
DROP PROCEDURE anahita_add_tokens_new_email;

-- +goose Down
-- +goose StatementBegin
CREATE PROCEDURE anahita_drop_tokens_new_email()
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'tokens'
          AND COLUMN_NAME = 'new_email'
    ) THEN
        ALTER TABLE tokens DROP COLUMN `new_email`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_drop_tokens_new_email();
DROP PROCEDURE anahita_drop_tokens_new_email;
