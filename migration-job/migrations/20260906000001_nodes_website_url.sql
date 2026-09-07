-- Adds nodes.website_url, one link on an actor profile.
--
-- Replaces the website/contact_url/phone trio that used to be read out of the
-- legacy `information` map. Nothing in the Go stack ever populated that map, so
-- the fields rendered empty and no request accepted them — the profile form
-- appeared to offer three editable values and wrote none of them.
--
-- One field rather than a list, matching Instagram, X, GitHub and Threads. It
-- maps cleanly onto a single schema.org PropertyValue attachment, which is how
-- Mastodon carries profile fields over ActivityPub, so a labelled-list version
-- later needs no migration of what is stored here.
--
-- On `nodes` rather than `people_people` because a group has a website too.
--
-- No index: nothing looks an actor up by its website, and this is read only as
-- part of a profile that has already been found by id or alias.
--
-- GUARDED, for the same reason as the migrations before it — see
-- 20260903000002 for the full explanation of why a plain ALTER would fail.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_add_nodes_website_url()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = 'website_url'
    ) THEN
        ALTER TABLE nodes
            ADD COLUMN `website_url` varchar(255) CHARACTER SET utf8mb4 DEFAULT NULL;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_add_nodes_website_url();
DROP PROCEDURE anahita_add_nodes_website_url;

-- +goose Down
-- +goose StatementBegin
CREATE PROCEDURE anahita_drop_nodes_website_url()
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = 'website_url'
    ) THEN
        ALTER TABLE nodes DROP COLUMN `website_url`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_drop_nodes_website_url();
DROP PROCEDURE anahita_drop_nodes_website_url;
