-- Quote posts: a note that carries another post under its own words.
--
--   quote_id            on the quoting note: the post it quotes.
--   quote_detached_at   on the quoting note: when the author of the quoted
--                       post took their post out of it. The note stays.
--   quote_policy        who may quote: 'anyone', 'followers' or 'nobody'.
--                       On a post it is that post's. On a person it is what
--                       their posts have when they say nothing. NULL is "not
--                       said", which in the end means anyone.
--
-- quote_count, how many notes quote a post, has been on nodes all along with
-- nothing writing it.
--
-- GUARDED, for the same reason as the migrations before it.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_add_nodes_quotes()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes' AND COLUMN_NAME = 'quote_id'
    ) THEN
        ALTER TABLE nodes ADD COLUMN `quote_id` bigint(11) unsigned DEFAULT NULL AFTER `root_id`;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes' AND COLUMN_NAME = 'quote_detached_at'
    ) THEN
        ALTER TABLE nodes ADD COLUMN `quote_detached_at` datetime DEFAULT NULL AFTER `quote_id`;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes' AND COLUMN_NAME = 'quote_policy'
    ) THEN
        ALTER TABLE nodes ADD COLUMN `quote_policy` varchar(16) DEFAULT NULL AFTER `reply_access`;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.STATISTICS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes' AND INDEX_NAME = 'quote_id'
    ) THEN
        ALTER TABLE nodes ADD INDEX `quote_id` (`quote_id`);
    END IF;
END;
-- +goose StatementEnd
CALL anahita_add_nodes_quotes();
DROP PROCEDURE anahita_add_nodes_quotes;

-- +goose Down
-- +goose StatementBegin
CREATE PROCEDURE anahita_drop_nodes_quotes()
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.STATISTICS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes' AND INDEX_NAME = 'quote_id'
    ) THEN
        ALTER TABLE nodes DROP INDEX `quote_id`;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes' AND COLUMN_NAME = 'quote_policy'
    ) THEN
        ALTER TABLE nodes DROP COLUMN `quote_policy`;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes' AND COLUMN_NAME = 'quote_detached_at'
    ) THEN
        ALTER TABLE nodes DROP COLUMN `quote_detached_at`;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes' AND COLUMN_NAME = 'quote_id'
    ) THEN
        ALTER TABLE nodes DROP COLUMN `quote_id`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_drop_nodes_quotes();
DROP PROCEDURE anahita_drop_nodes_quotes;
