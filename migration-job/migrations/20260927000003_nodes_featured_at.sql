-- Adds nodes.featured_at: the actors the installation features.
--
-- Onboarding offers them to a new member to follow, so their first feed is not
-- empty. A flag on the actor rather than a list in configuration or a file,
-- because:
--
--   - changing it is a click on the profile, not a redeploy;
--   - it cannot go stale — a renamed, disabled or deleted account drops out
--     through the same query that reads the flag;
--   - it is reusable beyond onboarding, an Explore section for instance.
--
-- A timestamp rather than a boolean: it is the flag and the default order at
-- once, most recently featured first. People and groups can both be featured.
--
-- Indexed, because the featured list is read by filtering and ordering on it.
--
-- GUARDED, for the same reason as the migrations before it: entrypoint.sh runs
-- schema/init.sql on an empty database and stamps the versions afterwards, so a
-- migration must survive being run against a schema that already has its
-- change.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_add_nodes_featured_at()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = 'featured_at'
    ) THEN
        ALTER TABLE nodes ADD COLUMN `featured_at` datetime DEFAULT NULL;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.STATISTICS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND INDEX_NAME = 'featured_at'
    ) THEN
        ALTER TABLE nodes ADD INDEX `featured_at` (`featured_at`);
    END IF;
END;
-- +goose StatementEnd
CALL anahita_add_nodes_featured_at();
DROP PROCEDURE anahita_add_nodes_featured_at;

-- +goose Down
-- +goose StatementBegin
CREATE PROCEDURE anahita_drop_nodes_featured_at()
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.STATISTICS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND INDEX_NAME = 'featured_at'
    ) THEN
        ALTER TABLE nodes DROP INDEX `featured_at`;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = 'featured_at'
    ) THEN
        ALTER TABLE nodes DROP COLUMN `featured_at`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_drop_nodes_featured_at();
DROP PROCEDURE anahita_drop_nodes_featured_at;
