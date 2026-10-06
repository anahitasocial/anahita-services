-- nodes.pinned_at: when a post was pinned to the profile it is on.
--
-- nodes.pinned, the flag, has been there all along. This is beside it for
-- one thing: the order pinned posts are shown in, most recently pinned
-- first. It is NULL on a post that is not pinned.
--
-- The index is what a profile's lists read with now: its pinned posts first.
--
-- GUARDED, for the same reason as the migrations before it.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_add_nodes_pinned_at()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes' AND COLUMN_NAME = 'pinned_at'
    ) THEN
        ALTER TABLE nodes ADD COLUMN `pinned_at` datetime DEFAULT NULL AFTER `pinned`;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.STATISTICS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes' AND INDEX_NAME = 'owner_pinned'
    ) THEN
        ALTER TABLE nodes ADD INDEX `owner_pinned` (`owner_id`, `pinned`);
    END IF;
END;
-- +goose StatementEnd
CALL anahita_add_nodes_pinned_at();
DROP PROCEDURE anahita_add_nodes_pinned_at;

-- A post pinned before there was a date is dated by when it was last
-- changed, which is the nearest thing known.
UPDATE nodes SET pinned_at = COALESCE(updated_at, created_at) WHERE pinned = 1 AND pinned_at IS NULL;

-- +goose Down
-- +goose StatementBegin
CREATE PROCEDURE anahita_drop_nodes_pinned_at()
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.STATISTICS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes' AND INDEX_NAME = 'owner_pinned'
    ) THEN
        ALTER TABLE nodes DROP INDEX `owner_pinned`;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes' AND COLUMN_NAME = 'pinned_at'
    ) THEN
        ALTER TABLE nodes DROP COLUMN `pinned_at`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_drop_nodes_pinned_at();
DROP PROCEDURE anahita_drop_nodes_pinned_at;
