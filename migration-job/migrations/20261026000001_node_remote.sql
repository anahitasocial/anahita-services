-- What came from another server.
--
-- A person followed on Mastodon or Bluesky, and what they post, are kept
-- here as nodes like any other, so that feeds, follows and blocks need to
-- know nothing new. Two columns say that such a node is not ours:
--
--   nodes.uri            what the node is called where it lives: the
--                        address of an ActivityPub object, or an at:// URI.
--                        Unique: one node for each thing elsewhere, however
--                        often it is heard of. NULL on everything made here,
--                        which is what "made here" means.
--   nodes.origin_domain  the server it came from, in lower case. For
--                        refusing a server, and for saying where a post is
--                        from.
--
-- GUARDED, for the same reason as the migrations before it.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_add_node_remote()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes' AND COLUMN_NAME = 'uri'
    ) THEN
        ALTER TABLE nodes
            ADD COLUMN `uri` varchar(512) CHARACTER SET ascii COLLATE ascii_bin DEFAULT NULL AFTER `uuid`,
            ADD COLUMN `origin_domain` varchar(255) DEFAULT NULL AFTER `uri`,
            ADD UNIQUE KEY `uq_uri` (`uri`),
            ADD KEY `origin_domain` (`origin_domain`);
    END IF;
END;
-- +goose StatementEnd
CALL anahita_add_node_remote();
DROP PROCEDURE anahita_add_node_remote;

-- +goose Down
-- +goose StatementBegin
CREATE PROCEDURE anahita_drop_node_remote()
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes' AND COLUMN_NAME = 'uri'
    ) THEN
        ALTER TABLE nodes DROP COLUMN `origin_domain`, DROP COLUMN `uri`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_drop_node_remote();
DROP PROCEDURE anahita_drop_node_remote;
