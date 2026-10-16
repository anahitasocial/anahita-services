-- An article's excerpt, dropped (the user, 2026-10-16). Nobody wrote one
-- any more and nothing showed it: where a short form of a post is wanted,
-- a page's preview, a card, it is made from the opening of its text, as
-- WordPress makes one when none is written. The excerpts there were, the
-- old site's included, are dropped with the column, at the user's word.
--
-- GUARDED, for the same reason as the migrations before it.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_drop_excerpt()
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes' AND COLUMN_NAME = 'excerpt'
    ) THEN
        ALTER TABLE nodes DROP COLUMN `excerpt`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_drop_excerpt();
DROP PROCEDURE anahita_drop_excerpt;

-- +goose Down
-- The column comes back empty: what was in it is gone.
-- +goose StatementBegin
CREATE PROCEDURE anahita_add_excerpt()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes' AND COLUMN_NAME = 'excerpt'
    ) THEN
        ALTER TABLE nodes ADD COLUMN `excerpt` text AFTER `vote_up_count`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_add_excerpt();
DROP PROCEDURE anahita_add_excerpt;
