-- An article or a topic can be a draft (the user, 2026-10-16): being
-- written, and seen by nobody but who is writing it. It is in no list, its
-- author's own included but for their drafts, and nothing announces it.
-- Published, it is dated then and announced as a post made then.
--
--   nodes.draft  on an article or a topic: whether it is still a draft.
--
-- Off for everything there is.
--
-- GUARDED, for the same reason as the migrations before it.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_add_draft()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes' AND COLUMN_NAME = 'draft'
    ) THEN
        ALTER TABLE nodes ADD COLUMN `draft` tinyint(1) NOT NULL DEFAULT 0 AFTER `library_only`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_add_draft();
DROP PROCEDURE anahita_add_draft;

-- +goose Down
-- +goose StatementBegin
CREATE PROCEDURE anahita_drop_draft()
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes' AND COLUMN_NAME = 'draft'
    ) THEN
        ALTER TABLE nodes DROP COLUMN `draft`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_drop_draft();
DROP PROCEDURE anahita_drop_draft;
