-- Adds what a one-time code needs to the tokens table:
--
--   code_hash      an HMAC of the six digits, never the digits. A plain hash
--                  of a six-digit value can be reversed by trying all million.
--   code_attempts  wrong submissions so far, counted in the database so that
--                  guesses arriving together cannot share one attempt.
--   channel        how the code was sent: 'email', and later 'sms'.
--
-- All three are unused by every other token type.
--
-- GUARDED, like the migrations before it: entrypoint.sh runs schema/init.sql
-- on an empty database and stamps the versions afterwards, so a migration must
-- survive being run against a schema that already has its change.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_add_tokens_column(IN col VARCHAR(64), IN definition VARCHAR(255))
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'tokens'
          AND COLUMN_NAME = col
    ) THEN
        SET @anahita_sql = CONCAT('ALTER TABLE tokens ADD COLUMN `', col, '` ', definition);
        PREPARE anahita_stmt FROM @anahita_sql;
        EXECUTE anahita_stmt;
        DEALLOCATE PREPARE anahita_stmt;
    END IF;
END;
-- +goose StatementEnd

CALL anahita_add_tokens_column('code_hash', 'varchar(64) DEFAULT NULL AFTER `body`');
CALL anahita_add_tokens_column('code_attempts', 'int(11) NOT NULL DEFAULT 0 AFTER `code_hash`');
CALL anahita_add_tokens_column('channel', 'varchar(16) DEFAULT NULL AFTER `code_attempts`');

DROP PROCEDURE anahita_add_tokens_column;

-- +goose Down
-- +goose StatementBegin
CREATE PROCEDURE anahita_drop_tokens_column(IN col VARCHAR(64))
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'tokens'
          AND COLUMN_NAME = col
    ) THEN
        SET @anahita_sql = CONCAT('ALTER TABLE tokens DROP COLUMN `', col, '`');
        PREPARE anahita_stmt FROM @anahita_sql;
        EXECUTE anahita_stmt;
        DEALLOCATE PREPARE anahita_stmt;
    END IF;
END;
-- +goose StatementEnd

CALL anahita_drop_tokens_column('channel');
CALL anahita_drop_tokens_column('code_attempts');
CALL anahita_drop_tokens_column('code_hash');

DROP PROCEDURE anahita_drop_tokens_column;
