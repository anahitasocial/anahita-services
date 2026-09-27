-- Step 1: Rename tables by removing the prefix
-- DATABASE(), not a hardcoded 'anahita': entrypoint.sh connects to
-- $MYSQL_DATABASE, which is configurable, so anyone whose database is called
-- anything else got a NULL prefix here and a RENAME of `_nodes` that cannot
-- resolve.
--
-- The prefix is taken from the nodes table specifically rather than from an
-- arbitrary row, which is also what entrypoint.sh looks for. Splitting on the
-- first underscore of whichever table came back first gets the wrong answer for
-- any prefix that itself contains one.
SET @prefix = (
    SELECT SUBSTRING_INDEX(TABLE_NAME, '_nodes', 1)
    FROM INFORMATION_SCHEMA.TABLES
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME LIKE '%\_nodes'
    LIMIT 1
);

SET @rename_edges = CONCAT('RENAME TABLE ', @prefix, '_edges TO edges;');
SET @rename_nodes = CONCAT('RENAME TABLE ', @prefix, '_nodes TO nodes;');
SET @rename_people_people = CONCAT('RENAME TABLE ', @prefix, '_people_people TO people_people;');
SET @rename_todos_todos = CONCAT('RENAME TABLE ', @prefix, '_todos_todos TO todos_todos;');

PREPARE stmt FROM @rename_edges;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

PREPARE stmt FROM @rename_nodes;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

PREPARE stmt FROM @rename_people_people;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

PREPARE stmt FROM @rename_todos_todos;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- Step 2: Drop all tables except edges, nodes, people_people, and todos_todos
SET SESSION group_concat_max_len = 1000000;

SET @drop_tables = (
    SELECT GROUP_CONCAT(TABLE_NAME SEPARATOR ',')
    FROM INFORMATION_SCHEMA.TABLES
    WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME NOT IN ('edges', 'nodes', 'people_people')
);

-- Debugging output to check the generated drop statements
SELECT @drop_tables AS drop_statements;

-- Execute the drop statements if there are tables to drop
SET @drop_query = CONCAT('DROP TABLE ', @drop_tables);
PREPARE stmt FROM @drop_query;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- Step 3: DELETE rows from nodes table WHERE component isn't in ('com_people', 'com_topics', 'com_groups', 'com_notes', 'com_photos', 'com_articles', 'com_topics', 'com_stories', 'com_hashtags', 'com_locations')
DELETE FROM nodes WHERE component NOT IN ('com_people', 'com_topics', 'com_groups', 'com_notes', 'com_photos', 'com_articles', 'com_topics', 'com_stories', 'com_hashtags', 'com_locations');

-- Step 4: update node and edge types

-- Update node types

-- Actors
UPDATE nodes SET type = 'node.actor.person-service.person.v1' WHERE type LIKE '%com:people.domain.entity.person%';
UPDATE nodes SET type = 'node.actor.group-service.group.v1' WHERE type LIKE '%com:groups.domain.entity.group%';

-- Mediums (Content)
UPDATE nodes SET type = 'node.medium.text-service.note.v1' WHERE type LIKE '%com:notes.domain.entity.note%';
UPDATE nodes SET type = 'node.medium.photo-service.photo.v1' WHERE type LIKE '%com:photos.domain.entity.photo%';
UPDATE nodes SET type = 'node.medium.photo-service.photo-set.v1' WHERE type LIKE '%com:photos.domain.entity.set%';
UPDATE nodes SET type = 'node.medium.text-service.article.v1' WHERE type LIKE '%com:articles.domain.entity.article%';
UPDATE nodes SET type = 'node.base.text-service.article-revision.v1' WHERE type LIKE '%com:articles.domain.entity.revision%';
UPDATE nodes SET type = 'node.medium.text-service.topic.v1' WHERE type LIKE '%com:topics.domain.entity.topic%';

-- Comments
UPDATE nodes SET type = 'node.base.comment-service.comment.v1' WHERE type LIKE '%com:comments.domain.entity.comment%';
UPDATE nodes SET type = 'node.base.comment-service.comment.v1' WHERE type LIKE '%com:topics.domain.entity.comment%';
UPDATE nodes SET type = 'node.base.comment-service.comment.v1' WHERE type LIKE '%com:photos.domain.entity.comment%';
UPDATE nodes SET type = 'node.base.comment-service.comment.v1' WHERE type LIKE '%com:people.domain.entity.comment%';
UPDATE nodes SET type = 'node.base.comment-service.comment.v1' WHERE type LIKE '%com:notes.domain.entity.comment%';
UPDATE nodes SET type = 'node.base.comment-service.comment.v1' WHERE type LIKE '%com:groups.domain.entity.comment%';
UPDATE nodes SET type = 'node.base.comment-service.comment.v1' WHERE type LIKE '%com:articles.domain.entity.comment%';

-- Tags
UPDATE nodes SET type = 'node.tag.hashtag-service.hashtag.v1' WHERE type LIKE '%com:hashtags.domain.entity.hashtag%';
UPDATE nodes SET type = 'node.tag.location-service.location.v1' WHERE type LIKE '%com:locations.domain.entity.location%';

-- Notifications (Using `component` for service context)
UPDATE nodes SET type = 'node.base.notification-service.notification.v1' WHERE type LIKE '%com:notifications.domain.entity.notification%';

-- Stories (Using `component` for service context)
UPDATE nodes SET type = 'node.story.story-service.story.v1' WHERE type LIKE '%com:stories.domain.entity.story%';

-- Actor relationships
UPDATE edges SET type = 'edge.actor.admin.v1' WHERE type LIKE '%com:actors.domain.entity.administrator%';
UPDATE edges SET type = 'edge.actor.block.v1' WHERE type LIKE '%com:actors.domain.entity.block%';
UPDATE edges SET type = 'edge.actor.follow.v1' WHERE type LIKE '%com:actors.domain.entity.follow%';
UPDATE edges SET type = 'edge.actor.follow-request.v1' WHERE type LIKE '%com:actors.domain.entity.request%';

-- Base relationships
UPDATE edges SET type = 'edge.base.subscription.v1' WHERE type LIKE '%com:base.domain.entity.subscription%';
UPDATE edges SET type = 'edge.base.voteup.v1' WHERE type LIKE '%com:base.domain.entity.voteup%';

-- Notification settings
UPDATE edges SET type = 'edge.notification.setting.v1' WHERE type LIKE '%com:notifications.domain.entity.setting%';

-- Tag relationships
UPDATE edges SET type = 'edge.tag.hashtag.v1' WHERE type LIKE '%com:hashtags.domain.entity.tag%';
UPDATE edges SET type = 'edge.tag.location.v1' WHERE type LIKE '%com:locations.domain.entity.tag%';
UPDATE edges SET type = 'edge.tag.mention.v1' WHERE type LIKE '%com:people.domain.entity.mention%';

-- Photo relationships
UPDATE edges SET type = 'edge.photo.photo-set.v1' WHERE type LIKE '%com:photos.domain.entity.edge%';

-- node_a_type
UPDATE edges SET node_a_type = 'node.actor.person-service.person.v1' WHERE node_a_type LIKE '%com:people.domain.entity.person%';
UPDATE edges SET node_a_type = 'node.medium.photo-service.photo.v1' WHERE node_a_type LIKE '%com:photos.domain.entity.photo%';
UPDATE edges SET node_a_type = 'node.tag.hashtag-service.hashtag.v1' WHERE node_a_type LIKE '%com:hashtags.domain.entity.tag%';
UPDATE edges SET node_a_type = 'node.tag.location-service.location.v1' WHERE node_a_type LIKE '%com:locations.domain.entity.tag%';
-- The two above match the EDGE's entity name ("tag"), which is not what the
-- NODE at the other end is called. A hashtag edge is `...entity.tag` while the
-- hashtag node it points at is `...entity.hashtag`, so neither pattern ever
-- matched a single row and every tag edge kept its legacy node_a_type — 8,314
-- hashtag edges and 587 location edges on the reference database.
--
-- That is not cosmetic: node_a_type is a query filter (edges.go, edges_read.go,
-- edges_delete.go) and part of uc_edge, so a caller passing the modern type
-- matched nothing, untagging silently did nothing, and re-tagging inserted a
-- second edge rather than colliding with the first.
UPDATE edges SET node_a_type = 'node.tag.hashtag-service.hashtag.v1' WHERE node_a_type LIKE '%com:hashtags.domain.entity.hashtag%';
UPDATE edges SET node_a_type = 'node.tag.location-service.location.v1' WHERE node_a_type LIKE '%com:locations.domain.entity.location%';

-- node_b_type
UPDATE edges SET node_b_type = 'node.actor.group-service.group.v1' WHERE node_b_type LIKE '%com:groups.domain.entity.group%';
UPDATE edges SET node_b_type = 'node.actor.person-service.person.v1' WHERE node_b_type LIKE '%com:people.domain.entity.person%';

-- Medium (Content) Nodes
UPDATE edges SET node_b_type = 'node.medium.photo-service.photo-set.v1' WHERE node_b_type LIKE '%com:photos.domain.entity.set%';
UPDATE edges SET node_b_type = 'node.medium.photo-service.photo.v1' WHERE node_b_type LIKE '%com:photos.domain.entity.photo%';
UPDATE edges SET node_b_type = 'node.medium.text-service.note.v1' WHERE node_b_type LIKE '%com:notes.domain.entity.note%';
UPDATE edges SET node_b_type = 'node.medium.text-service.article.v1' WHERE node_b_type LIKE '%com:articles.domain.entity.article%';
UPDATE edges SET node_b_type = 'node.medium.text-service.topic.v1' WHERE node_b_type LIKE '%com:topics.domain.entity.topic%';

-- Comment Nodes
--
-- `node.base.comment-service.comment.v1`, matching constants.NodeBaseCommentEntityType
-- and the `type` these rows carry in the nodes table. This wrote
-- `node.comment.comment-service.comment.v1` — a string that appears nowhere in
-- the nodes table, nowhere in anahita-libs/constants, and nowhere else in the
-- workspace.
--
-- Every component that could be commented on needs a line here. Only four had
-- one, so topics, notes, pages, articles and todos kept their legacy value: on
-- the reference database 3,294 of 3,295 comment edges disagreed with the type
-- of the node they point at, the one correct row having been written by the Go
-- stack rather than by this file.
UPDATE edges SET node_b_type = 'node.base.comment-service.comment.v1' WHERE node_b_type LIKE '%com:stories.domain.entity.comment%';
UPDATE edges SET node_b_type = 'node.base.comment-service.comment.v1' WHERE node_b_type LIKE '%com:groups.domain.entity.comment%';
UPDATE edges SET node_b_type = 'node.base.comment-service.comment.v1' WHERE node_b_type LIKE '%com:people.domain.entity.comment%';
UPDATE edges SET node_b_type = 'node.base.comment-service.comment.v1' WHERE node_b_type LIKE '%com:photos.domain.entity.comment%';
UPDATE edges SET node_b_type = 'node.base.comment-service.comment.v1' WHERE node_b_type LIKE '%com:topics.domain.entity.comment%';
UPDATE edges SET node_b_type = 'node.base.comment-service.comment.v1' WHERE node_b_type LIKE '%com:notes.domain.entity.comment%';
UPDATE edges SET node_b_type = 'node.base.comment-service.comment.v1' WHERE node_b_type LIKE '%com:pages.domain.entity.comment%';
UPDATE edges SET node_b_type = 'node.base.comment-service.comment.v1' WHERE node_b_type LIKE '%com:articles.domain.entity.comment%';
UPDATE edges SET node_b_type = 'node.base.comment-service.comment.v1' WHERE node_b_type LIKE '%com:todos.domain.entity.comment%';
-- Anything already carrying the wrong rewrite from a previous run of this file.
UPDATE edges SET node_b_type = 'node.base.comment-service.comment.v1' WHERE node_b_type = 'node.comment.comment-service.comment.v1';

-- Nodes the component filter above should have removed but did not.
--
-- Step 3 deletes by `component NOT IN (...)`, and these rows carry a type
-- string in place of a component, so the filter never saw them. 123 survived on
-- the reference database and were counted by every query that does not filter
-- on type.
DELETE FROM nodes WHERE type LIKE 'ComComponents%';

-- Edges whose endpoints no longer exist.
--
-- Step 3 deletes the nodes of every component that did not survive the port —
-- todos being the large one — but never the edges pointing at them. 3,427 such
-- edges remained on the reference database, roughly a tenth of the table, each
-- one a relationship to a node that cannot be read.
--
-- After the type rewrites above, so that anything salvageable has already been
-- salvaged and only genuine orphans are removed.
DELETE e FROM edges e LEFT JOIN nodes n ON n.id = e.node_a_id WHERE n.id IS NULL;
DELETE e FROM edges e LEFT JOIN nodes n ON n.id = e.node_b_id WHERE n.id IS NULL;

-- Migrate component field
-- Update `component` field to match service names
UPDATE nodes SET component = 'person-service' WHERE component = 'com_people';
UPDATE nodes SET component = 'group-service' WHERE component = 'com_groups';
UPDATE nodes SET component = 'event-service' WHERE component = 'com_events';
UPDATE nodes SET component = 'project-service' WHERE component = 'com_projects';
UPDATE nodes SET component = 'text-service' WHERE component = 'com_topics';
UPDATE nodes SET component = 'photo-service' WHERE component = 'com_photos';
UPDATE nodes SET component = 'text-service' WHERE component = 'com_notes';
UPDATE nodes SET component = 'todo-service' WHERE component = 'com_todos';
UPDATE nodes SET component = 'text-service' WHERE component = 'com_articles';
UPDATE nodes SET component = 'hashtag-service' WHERE component = 'com_hashtags';
UPDATE nodes SET component = 'location-service' WHERE component = 'com_locations';
UPDATE nodes SET component = 'comment-service' WHERE component IN ('com_topics', 'com_photos', 'com_notes', 'com_articles', 'com_stories');
UPDATE nodes SET component = 'notification-service' WHERE component = 'com_notifications';
UPDATE nodes SET component = 'assignment-service' WHERE component = 'com_components';

-- Rename `component` to `service`
ALTER TABLE nodes CHANGE component service VARCHAR(255);

-- Modify `access` column to be VARCHAR(255) NOT NULL
UPDATE nodes SET access = LEFT(access, 255);
UPDATE nodes SET access = '' WHERE access IS NULL;
ALTER TABLE nodes MODIFY COLUMN access VARCHAR(255) NOT NULL;

-- Cleanup nodes.filename data from com_socialengine/ prefix
UPDATE nodes
SET filename = REPLACE(filename, 'com_socialengine/', '')
WHERE filename LIKE 'com_socialengine/%';

UPDATE nodes SET status = '';

-- Add a few additional fields to nodes table
ALTER TABLE nodes ADD COLUMN `features` TEXT AFTER `access`;

-- Drop a few fields from nodes table
ALTER TABLE nodes DROP COLUMN `permissions`;

-- Migrate notifications to `notifications` table

CREATE TABLE IF NOT EXISTS signup_requests (
    `id`       bigint unsigned NOT NULL AUTO_INCREMENT,
    `username` varchar(32)  NOT NULL,
    `email`    varchar(255) NOT NULL,
    `password` varchar(255) NOT NULL,

    -- What an administrator is deciding on. Required in this mode, because
    -- deciding on an email address alone is not deciding.
    `name`        varchar(100) DEFAULT NULL,
    `body`        text         DEFAULT NULL,
    `website_url` varchar(255) DEFAULT NULL,

    -- unverified -> pending -> approved | rejected
    --
    -- A request reaches the queue only once its address is confirmed, so
    -- administrators are not moderating junk that nobody can even receive mail
    -- at. `unverified` is where it waits until then.
    `status` varchar(20) NOT NULL DEFAULT 'unverified',
    `email_verified_at` datetime DEFAULT NULL,

    -- Who vouched, when a member's invitation put them here rather than a
    -- cold request. An administrator's invitation never reaches this
    -- table — it is already a decision.
    `invited_by` bigint unsigned DEFAULT NULL,

    `decided_by`    bigint unsigned DEFAULT NULL,
    `decided_at`    datetime        DEFAULT NULL,
    `decision_note` varchar(255)    DEFAULT NULL,

    -- Where the request came from. The same evidential trio the consent
    -- columns carry, and for the same reason: a queue entry that cannot say
    -- where it came from is harder to moderate, and a spam wave is usually
    -- recognisable by its source long before its content.
    `created_ip`         varchar(45)  DEFAULT NULL,
    `created_user_agent` varchar(255) DEFAULT NULL,

    `created_at` datetime NOT NULL,
    `updated_at` datetime NOT NULL,

    PRIMARY KEY (`id`),
    UNIQUE KEY `email` (`email`),
    KEY `status` (`status`),
    KEY `username` (`username`),
    KEY `created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
-- --------------------------------------------------------

CREATE TABLE notifications (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    `type` varchar(255) NOT NULL,
    `service` varchar(255) NOT NULL,
    `recipient_id` bigint(11) unsigned DEFAULT NULL,
    `subject_id` bigint(11) unsigned DEFAULT NULL,
    `subject_type` varchar(255) DEFAULT NULL,
    `object_id` bigint(11) unsigned DEFAULT NULL,
    `object_type` varchar(255) DEFAULT NULL,
    `target_id` bigint(11) unsigned DEFAULT NULL,
    `target_type` varchar(255) DEFAULT NULL,
    `comment_id` int(11) unsigned DEFAULT NULL,
    `created_on` datetime DEFAULT NULL,
    `modified_on` datetime DEFAULT NULL,
    -- mediumtext, not text. An existing database reaches utf8mb4 through
    -- CONVERT TO CHARACTER SET, which promotes text to preserve its character
    -- capacity; a fresh one is created as utf8mb4 outright and would keep
    -- text. Declaring the wider type is what makes the two paths agree.
    `meta` mediumtext DEFAULT NULL,
    `is_read` tinyint(1) NOT NULL DEFAULT 0,
    PRIMARY KEY (`id`),
    KEY `recipient_id` (`recipient_id`),
    KEY `subject_id` (`subject_id`),
    KEY `subject_type` (`subject_type`),
    KEY `object_id` (`object_id`),
    KEY `object_type` (`object_type`),
    KEY `target_id` (`target_id`),
    KEY `target_type` (`target_type`),
    KEY `comment_id` (`comment_id`),
    KEY `created_on` (`created_on`),
    KEY `modified_on` (`modified_on`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Migrate data from nodes to notifications
DELIMITER $$

CREATE PROCEDURE migrate_notifications()
BEGIN
  DECLARE done INT DEFAULT 0;
  DECLARE n_id BIGINT;
  DECLARE n_type VARCHAR(255);
  DECLARE n_service VARCHAR(255);
  DECLARE n_name VARCHAR(255);
  DECLARE n_meta TEXT;
  DECLARE n_created_on DATETIME;
  DECLARE n_modified_on DATETIME;
  DECLARE n_subject_id BIGINT;
  DECLARE n_object_type VARCHAR(255);
  DECLARE n_object_id BIGINT;
  DECLARE n_target_id BIGINT;
  DECLARE n_comment_id INT;
  DECLARE n_subscriber_ids TEXT;

  DECLARE cur CURSOR FOR
    SELECT id, service, name, meta, created_on, modified_on,
           story_subject_id, story_object_type, story_object_id,
           story_target_id, story_comment_id, subscriber_ids
    FROM nodes
    WHERE type = 'node.base.notification-service.notification.v1';

  DECLARE CONTINUE HANDLER FOR NOT FOUND SET done = 1;

  OPEN cur;

  read_loop: LOOP
    FETCH cur INTO n_id, n_service, n_name, n_meta, n_created_on, n_modified_on,
                   n_subject_id, n_object_type, n_object_id,
                   n_target_id, n_comment_id, n_subscriber_ids;
    IF done THEN
      LEAVE read_loop;
    END IF;

    -- Split subscriber_ids string and iterate
    WHILE LOCATE(',', n_subscriber_ids) > 0 DO
      SET @subscriber_id = SUBSTRING_INDEX(n_subscriber_ids, ',', 1);
      SET n_subscriber_ids = SUBSTRING(n_subscriber_ids, LOCATE(',', n_subscriber_ids) + 1);

      INSERT INTO notifications (
        type, service, recipient_id, subject_id, object_id, object_type,
        target_id, comment_id, created_on, modified_on,
        meta, is_read
      )
      VALUES (
        n_name, n_service, @subscriber_id, n_subject_id, n_object_id, n_object_type,
        n_target_id, n_comment_id, n_created_on, n_modified_on,
        n_meta, 1
      );
    END WHILE;

    -- Final ID (last one without comma)
    IF LENGTH(TRIM(n_subscriber_ids)) > 0 THEN
      INSERT INTO notifications (
        type, service, recipient_id, subject_id, object_id, object_type,
        target_id, comment_id, created_on, modified_on,
        meta, is_read
      )
      VALUES (
        n_name, n_service, TRIM(n_subscriber_ids), n_subject_id, n_object_id, n_object_type,
        n_target_id, n_comment_id, n_created_on, n_modified_on,
        n_meta, 1
      );
    END IF;

  END LOOP;

  CLOSE cur;
END$$

DELIMITER ;
CALL migrate_notifications();
DELETE FROM nodes WHERE type = 'node.base.notification-service.notification.v1';
ALTER TABLE nodes DROP COLUMN notification_ids;
ALTER TABLE nodes DROP COLUMN new_notification_ids;

-- Add indices

CREATE INDEX idx_edges_node_a_id ON edges (node_a_id);
CREATE INDEX idx_edges_node_b_id ON edges (node_b_id);
CREATE INDEX idx_edges_type     ON edges (type);

-- Composite indices for frequent relationship lookups
CREATE INDEX idx_edges_aid_type_bid ON edges (node_a_id, type, node_b_id);
CREATE INDEX idx_edges_bid_type_aid ON edges (node_b_id, type, node_a_id);

CREATE INDEX idx_nodes_id        ON nodes (id);
CREATE INDEX idx_nodes_owner_id  ON nodes (owner_id);
CREATE INDEX idx_nodes_type      ON nodes (type);
CREATE INDEX idx_nodes_access    ON nodes (access);
CREATE INDEX idx_nodes_created_on ON nodes (created_on);

CREATE INDEX idx_nodes_parent_id ON nodes (parent_id);

CREATE INDEX idx_edges_lookup ON edges (node_a_id, node_b_id, type);
CREATE INDEX idx_nodes_feed   ON nodes (owner_id, type, created_on);

-- Fast lookups by type + direction
CREATE INDEX idx_edges_type_nodea_nodeb  ON edges (type, node_a_id, node_b_id);
CREATE INDEX idx_edges_type_nodeb_nodea  ON edges (type, node_b_id, node_a_id);

-- Droping all CSV fields
ALTER TABLE nodes DROP COLUMN leader_ids;
ALTER TABLE nodes DROP COLUMN follower_ids;
ALTER TABLE nodes DROP COLUMN mutual_ids;
ALTER TABLE nodes DROP COLUMN blocker_ids;
ALTER TABLE nodes DROP COLUMN follow_requester_ids;
ALTER TABLE nodes DROP COLUMN subscriber_ids;
ALTER TABLE nodes DROP COLUMN administrating_ids;
ALTER TABLE nodes DROP COLUMN voter_up_ids;
ALTER TABLE nodes DROP COLUMN voter_down_ids;
ALTER TABLE nodes DROP COLUMN shared_owner_count;
ALTER TABLE nodes DROP COLUMN shared_owner_ids;
ALTER TABLE nodes DROP COLUMN administrator_ids;
ALTER TABLE nodes DROP COLUMN blocked_ids;

-- Removing stories
DELETE FROM nodes WHERE type = 'node.story.story-service.story.v1';
ALTER TABLE nodes DROP COLUMN story_subject_id;
ALTER TABLE nodes DROP COLUMN story_object_id;
ALTER TABLE nodes DROP COLUMN story_object_type;
ALTER TABLE nodes DROP COLUMN story_target_id;
ALTER TABLE nodes DROP COLUMN story_comment_id;

-- Add new counters
ALTER TABLE nodes ADD COLUMN `repost_count` INT(11) UNSIGNED NOT NULL DEFAULT 0 AFTER `comment_count`;
ALTER TABLE nodes ADD COLUMN `quote_count` INT(11) UNSIGNED NOT NULL DEFAULT 0 AFTER `repost_count`;

-- Add new TOTP fields to people_people table
-- Account deletion. entrypoint.sh runs THIS file for a legacy database and
-- then exits, so it never sees schema/init.sql or the goose migrations —
-- anything added there has to be added here too or a migrated install starts
-- with a schema the code cannot read. Every actor read selects
-- nodes.deleted_at, so its absence breaks far more than deletion.
ALTER TABLE nodes ADD COLUMN `deleted_at` datetime DEFAULT NULL;
ALTER TABLE nodes ADD INDEX `deleted_at` (`deleted_at`);

-- nodes.archived_at, the third lifecycle state. Permanent, and unlike deletion
-- it destroys nothing — the content keeps its URLs and stops being pushed at
-- people. Its own column rather than a fourth meaning for `enabled`.
ALTER TABLE nodes ADD COLUMN `archived_at` datetime DEFAULT NULL;
ALTER TABLE nodes ADD INDEX `archived_at` (`archived_at`);

-- One link on an actor profile. A migrated install has the legacy website,
-- contact and phone values inside `meta`; they are deliberately NOT carried
-- across — nothing in the Go stack ever read that map, so they have been
-- invisible since the port and are not worth reviving.
ALTER TABLE nodes ADD COLUMN `website_url` varchar(255) DEFAULT NULL;

CREATE TABLE node_deletion_logs (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    `node_id` bigint(11) unsigned NOT NULL,
    `node_alias` varchar(255) DEFAULT NULL,
    `node_type` varchar(255) DEFAULT NULL,
    `deleted_by` bigint(11) unsigned NOT NULL,
    `action` varchar(20) NOT NULL,
    `created_on` datetime NOT NULL,
    PRIMARY KEY (`id`),
    KEY `idx_node_deletion_logs_node_id` (`node_id`),
    KEY `idx_node_deletion_logs_created_on` (`created_on`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

ALTER TABLE people_people ADD COLUMN `totp_secret` VARCHAR(255) DEFAULT '';
ALTER TABLE people_people ADD COLUMN `totp_enabled` TINYINT(1) NOT NULL DEFAULT 0;
ALTER TABLE people_people ADD COLUMN `totp_recovery_codes` TEXT DEFAULT NULL;
ALTER TABLE people_people ADD COLUMN `totp_last_verified_at` DATETIME DEFAULT NULL;

-- Create auth_logs table
CREATE TABLE auth_logs (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    `created_on` datetime DEFAULT NULL,
    `created_by` BIGINT UNSIGNED NOT NULL,
    `agent_device` varchar(50) DEFAULT 'bot',
    `agent_os` varchar(255) DEFAULT NULL,
    `agent_name` varchar(255) DEFAULT NULL,
    `ip` varchar(255) DEFAULT NULL,
    `city` varchar(255) DEFAULT NULL,
    `country` varchar(255) DEFAULT NULL,
    `oauth_refresh_token_id` bigint(11) unsigned NOT NULL,
    `client_id` varchar(255) NOT NULL,
    PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE INDEX idx_auth_logs_created_by ON auth_logs (created_by);
CREATE INDEX idx_auth_logs_created_on ON auth_logs (created_on);
CREATE INDEX idx_auth_logs_created_by_client_id_created_on ON auth_logs (created_by, client_id, created_on DESC);
CREATE INDEX idx_auth_logs_oauth_refresh_token_id ON auth_logs (oauth_refresh_token_id);

-- --------------------------------------------------------
-- OAuth tables
-- --------------------------------------------------------

CREATE TABLE oauth_authorizations (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    `code_hash` varchar(255) NOT NULL,
    `client_id` varchar(255) NOT NULL,
    `user_id` bigint(11) unsigned NOT NULL,
    `redirect_uri` text NOT NULL,
    `scopes` text DEFAULT NULL,
    `code_challenge` varchar(128) DEFAULT NULL,
    `code_challenge_method` varchar(10) DEFAULT NULL,
    `nonce` varchar(255) DEFAULT NULL,
    `expires_at` datetime NOT NULL,
    -- Defaulted, not supplied by the insert: under MySQL strict mode a
    -- NOT NULL column with no default and no value is error 1364.
    `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_code_hash` (`code_hash`),
    KEY `idx_client_id` (`client_id`),
    KEY `idx_user_id` (`user_id`),
    KEY `idx_expires_at` (`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- --------------------------------------------------------

CREATE TABLE oauth_clients (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    `client_id` varchar(255) NOT NULL,
    `secret_hash` varchar(255) NOT NULL,
    `name` varchar(128) NOT NULL,
    `redirect_uris` JSON NOT NULL,
    `grant_types` JSON NOT NULL,
    `scopes` JSON NOT NULL,
    `token_expiry` int(11) NOT NULL DEFAULT 900,
    `confidential` tinyint(1) NOT NULL DEFAULT 1,
    `skip_consent` tinyint(1) NOT NULL DEFAULT 0,
    -- First-party clients are the apps we ship ourselves. Separate from
    -- skip_consent: a first-party client may still need a consent
    -- screen, and a third-party client is never ours regardless of how
    -- consent is configured.
    `first_party` tinyint(1) NOT NULL DEFAULT 0,
    `active` tinyint(1) NOT NULL DEFAULT 1,
    -- Defaulted because seeded first-party clients have no creating
    -- person — SeedDefaultClients runs at startup, before any request.
    `created_by` bigint(11) unsigned NOT NULL DEFAULT 0,
    -- created_at/updated_at rather than create_time/update_time: the
    -- repository selects created_at, and every other oauth_* table
    -- already uses the _at suffix. Both defaulted — neither Add nor
    -- Edit supplies them.
    `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_client_id` (`client_id`),
    KEY `idx_active` (`active`),
    KEY `idx_created_by` (`created_by`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- --------------------------------------------------------

CREATE TABLE oauth_refresh_tokens (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    `token_hash` varchar(255) NOT NULL,
    `user_id` bigint(11) unsigned NOT NULL,
    `client_id` varchar(255) NOT NULL,
    `scopes` text DEFAULT NULL,
    `revoked` tinyint(1) NOT NULL DEFAULT 0,
    -- Records WHEN the token was revoked, which reuse detection needs in
    -- order to tell a rotation race from a replay: revoked moments ago is
    -- a client refreshing concurrently with itself, revoked long ago is an
    -- attacker replaying a stolen token, and the two want opposite
    -- responses. See ReuseGracePeriod in auth-service.
    --
    -- Nullable on purpose: a revoked row carrying no timestamp reads as
    -- "revoked long ago" and still trips the containment cascade, which is
    -- the safe direction to be wrong in.
    `revoked_at` datetime DEFAULT NULL,
    `expires_at` datetime NOT NULL,
    -- Defaulted: the insert does not name it. See oauth_authorizations.
    `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    -- No index on revoked_at: reuse detection finds the row by token_hash,
    -- which is already UNIQUE, and reads revoked/revoked_at off it.
    UNIQUE KEY `idx_token_hash` (`token_hash`),
    KEY `idx_user_id` (`user_id`),
    KEY `idx_client_id` (`client_id`),
    KEY `idx_revoked_expires` (`revoked`, `expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- --------------------------------------------------------

CREATE TABLE oauth_signing_keys (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    `key_id` varchar(255) NOT NULL,
    `algorithm` varchar(10) NOT NULL DEFAULT 'RS256',
    `public_key` text NOT NULL,
    `private_key` text DEFAULT NULL,
    `active` tinyint(1) NOT NULL DEFAULT 1,
    `expires_at` datetime NOT NULL,
    -- Defaulted: neither Add nor the KMS seeder names it. See
    -- oauth_authorizations.
    `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `backend` varchar(255) NOT NULL DEFAULT 'local',
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_key_id` (`key_id`),
    KEY `idx_active` (`active`),
    KEY `idx_active_expires` (`active`, `expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- --------------------------------------------------------

-- Single-use tokens emailed to a person: password resets and invites.
-- created_by / updated_by are node ids, the same soft reference
-- auth_logs.created_by uses.

CREATE TABLE tokens (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    `recipient_email` varchar(255) NOT NULL,
    -- The requested address, on emailchange tokens only; NULL on every
    -- other type. Separate from recipient_email precisely so the two
    -- cannot be confused — swapping them turns the confirmation flow
    -- from a control into a way to hand the account away.
    --
    -- Declared HERE rather than added by a later ALTER: this file creates the
    -- table fresh, and the ALTER that used to carry this column ran 170 lines
    -- before the CREATE, so every legacy upgrade failed with
    -- "Table 'tokens' doesn't exist".
    `new_email` varchar(255) DEFAULT NULL,
    -- 'passwordreset' or 'invite' (constants.TokenType*). A varchar
    -- rather than an ENUM so adding a token type is a code change.
    `type` varchar(50) NOT NULL,
    -- 64 random bytes, hex-encoded. Indexed because ReadByBody is the
    -- lookup every reset/invite link performs.
    `body` varchar(255) NOT NULL,
    -- Defaulted: Token.NewToken sets StartTime in Go but tokens.Add
    -- does not include it in the insert column list.
    `start_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    -- Lifetime in seconds from start_time. Expiry is computed in Go
    -- (Token.SetStatus) rather than stored.
    `expired_after` int(11) NOT NULL DEFAULT 0,
    `is_used` tinyint(1) NOT NULL DEFAULT 0,
    `created_at` datetime NOT NULL,
    `created_by` bigint(11) unsigned NOT NULL,
    `updated_at` datetime NOT NULL,
    `updated_by` bigint(11) unsigned NOT NULL,
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_tokens_body` (`body`),
    KEY `idx_tokens_type` (`type`),
    KEY `idx_tokens_recipient_email` (`recipient_email`),
    KEY `idx_tokens_created_by` (`created_by`),
    KEY `idx_tokens_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- --------------------------------------------------------

-- WebAuthn (passkey) credentials. One row per authenticator a person
-- has enrolled. user_id is a node id, a soft reference like the rest of
-- this schema, so person deletion must call DeleteByPersonID
-- explicitly: a recycled node id would otherwise inherit a stranger's
-- passkey.

CREATE TABLE webauthn_credentials (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    `user_id` bigint(11) unsigned NOT NULL,
    -- Raw authenticator-generated identifier, stored as bytes rather
    -- than base64url so the login lookup does not depend on encoding
    -- choices matching across library versions. VARBINARY(255) is the
    -- spec maximum and, unlike BLOB, can carry a UNIQUE index directly.
    `credential_id` varbinary(255) NOT NULL,
    -- COSE-encoded public key. The private key never leaves the device.
    `public_key` blob NOT NULL,
    `attestation_type` varchar(255) NOT NULL DEFAULT '',
    -- Authenticator model identifier. Drives the name/icon shown in the
    -- passkey list, not any security decision.
    `aaguid` varbinary(16) DEFAULT NULL,
    -- Monotonic counter for clone detection. Synced platform
    -- authenticators always report 0, which is expected.
    `sign_count` bigint(20) unsigned NOT NULL DEFAULT 0,
    `clone_warning` tinyint(1) NOT NULL DEFAULT 0,
    -- Transport hints ("internal", "hybrid", "usb", "nfc", "ble"). JSON
    -- because MySQL has no array type; read and written through
    -- entities.StringSlice.
    `transports` JSON NOT NULL,
    -- BE / BS flags: whether this passkey can sync to a cloud keychain,
    -- and whether it currently is.
    `backup_eligible` tinyint(1) NOT NULL DEFAULT 0,
    `backup_state` tinyint(1) NOT NULL DEFAULT 0,
    -- Person-supplied label. Required: three unlabelled passkeys is not
    -- a list anyone can safely revoke from.
    `nickname` varchar(255) NOT NULL,
    `last_used_at` datetime DEFAULT NULL,
    `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_webauthn_credentials_credential_id` (`credential_id`),
    KEY `idx_webauthn_credentials_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- --------------------------------------------------------

-- Bring the three tables carried over from the PHP schema into line with
-- schema/init.sql. The tables created above already have the right shape;
-- nodes, edges and people_people arrive from the legacy database and have to
-- be altered into it.

-- Zero dates before anything that rebuilds a table, which is both the MODIFY
-- below and the CONVERT after it.
--
-- A rebuild is a write, and under the strict sql_mode every supported MySQL
-- runs by default '0000-00-00 00:00:00' is rejected on the way back in: the
-- ALTER dies with "Incorrect datetime value" and leaves the upgrade half
-- finished, every table created but none of them converted. The PHP schema is
-- full of these — MySQL accepted them then.
--
-- NULL, not a fabricated timestamp. The columns are nullable and "unknown" is
-- what these mean; inventing an epoch date would date real relationships to
-- 1970.
UPDATE edges SET created_on  = NULL WHERE CAST(created_on  AS CHAR) LIKE '0000%';
UPDATE edges SET modified_on = NULL WHERE CAST(modified_on AS CHAR) LIKE '0000%';
UPDATE edges SET start_date  = NULL WHERE CAST(start_date  AS CHAR) LIKE '0000%';
UPDATE edges SET end_date    = NULL WHERE CAST(end_date    AS CHAR) LIKE '0000%';

UPDATE nodes SET created_on         = NULL WHERE CAST(created_on         AS CHAR) LIKE '0000%';
UPDATE nodes SET modified_on        = NULL WHERE CAST(modified_on        AS CHAR) LIKE '0000%';
UPDATE nodes SET start_date         = NULL WHERE CAST(start_date         AS CHAR) LIKE '0000%';
UPDATE nodes SET end_date           = NULL WHERE CAST(end_date           AS CHAR) LIKE '0000%';
UPDATE nodes SET last_comment_on    = NULL WHERE CAST(last_comment_on    AS CHAR) LIKE '0000%';
UPDATE nodes SET status_update_time = NULL WHERE CAST(status_update_time AS CHAR) LIKE '0000%';
UPDATE nodes SET deleted_at         = NULL WHERE CAST(deleted_at         AS CHAR) LIKE '0000%';
UPDATE nodes SET archived_at        = NULL WHERE CAST(archived_at        AS CHAR) LIKE '0000%';

UPDATE people_people SET last_visit_date       = NULL WHERE CAST(last_visit_date       AS CHAR) LIKE '0000%';
UPDATE people_people SET totp_last_verified_at = NULL WHERE CAST(totp_last_verified_at AS CHAR) LIKE '0000%';

-- The type columns next, and this order is not optional either.
--
-- These hold machine-generated ASCII identifiers from a fixed grammar, longest
-- 42 characters, so varchar(255) was never needed. It is also unusable: the
-- charset conversion below widens every character from three bytes to four,
-- and uc_edge spans all three of the edges type columns — 3*255*4 + 16 = 3076
-- bytes, four over InnoDB's 3072 limit. Converting first fails with "Specified
-- key was too long" and leaves the database half migrated.
ALTER TABLE nodes MODIFY `type` varchar(128) NOT NULL;
ALTER TABLE edges MODIFY `type` varchar(128) NOT NULL;
ALTER TABLE edges MODIFY `node_a_type` varchar(128) NOT NULL;
ALTER TABLE edges MODIFY `node_b_type` varchar(128) NOT NULL;

-- Now the charset. utf8 in MySQL is utf8mb3, which cannot store an emoji or a
-- supplementary-plane character at all; a name containing one is truncated at
-- the first byte it cannot represent rather than rejected.
ALTER TABLE nodes         CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
ALTER TABLE edges         CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
ALTER TABLE people_people CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;

-- The index on nodes.service is still called `component`: step 5 renamed the
-- column and MySQL carried the index across under its old name, so a legacy
-- database and a fresh install disagreed on what it is called.
SET @rename_component_index = IF(
    EXISTS(SELECT 1 FROM information_schema.STATISTICS
           WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes'
             AND INDEX_NAME = 'component'),
    'ALTER TABLE nodes RENAME INDEX `component` TO `service`',
    'SELECT 1');
PREPARE stmt FROM @rename_component_index;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- people_people.people_person_id carries a redundant UNIQUE index alongside
-- the primary key, the signature of a SERIAL column. Every insert maintained
-- both. init.sql no longer produces one.
SET @drop_pp_serial_index = IF(
    EXISTS(SELECT 1 FROM information_schema.STATISTICS
           WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'people_people'
             AND INDEX_NAME = 'people_person_id'),
    'ALTER TABLE people_people DROP INDEX `people_person_id`',
    'SELECT 1');
PREPARE stmt FROM @drop_pp_serial_index;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- --------------------------------------------------------

-- Person identity onto `nodes`, and credentials into `auth_accounts`, then
-- `people_people` is dropped.
--
-- The drop belongs here rather than being left to goose: entrypoint.sh runs
-- this file INSTEAD of the migrations and then stamps them all as applied, so
-- whatever this file does not do never happens. It has to arrive at the same
-- schema schema/init.sql produces, which no longer has the table.

ALTER TABLE nodes ADD COLUMN `person_type` varchar(50) DEFAULT NULL;
ALTER TABLE nodes ADD COLUMN `person_pronouns` varchar(50) DEFAULT NULL;
ALTER TABLE nodes ADD COLUMN `last_visit_date` datetime DEFAULT NULL;
ALTER TABLE nodes ADD INDEX `person_type` (`person_type`);

-- name, alias, timezone and language are deliberately not copied: they are
-- already correct on nodes. given_name and family_name are not carried at all —
-- they are truncated at the first non-ASCII byte, while nodes.name is intact.
UPDATE nodes n JOIN people_people p ON p.node_id = n.id
SET n.person_type     = p.usertype,
    n.last_visit_date = p.last_visit_date;

-- The actor handle namespace. See schema/init.sql for why this is a generated
-- column rather than a unique index on alias.
ALTER TABLE nodes
    ADD COLUMN `unique_alias` varchar(300)
        GENERATED ALWAYS AS (
            CASE
                WHEN alias IS NULL OR alias = '' THEN NULL
                WHEN type LIKE 'node.actor.%'    THEN CONCAT('actor:', alias)
                ELSE NULL
            END
        ) STORED;
ALTER TABLE nodes ADD UNIQUE KEY `uq_unique_alias` (`unique_alias`);

CREATE TABLE auth_accounts (
    `node_id`               bigint unsigned NOT NULL,
    `email`                 varchar(255) DEFAULT NULL,
    `password`              varchar(255) DEFAULT NULL,
    `totp_secret`           varchar(255) DEFAULT '',
    `totp_enabled`          tinyint(1) NOT NULL DEFAULT 0,
    `totp_recovery_codes`   mediumtext DEFAULT NULL,
    `totp_last_verified_at` datetime DEFAULT NULL,

    -- When the address was confirmed. NULL means it has not been, and an
    -- unverified account cannot sign in.
    `email_verified_at`     datetime DEFAULT NULL,

    -- NULL for everybody: nobody has been through onboarding yet, and a
    -- member with an empty profile should be walked through it.
    `onboarded_at`          datetime DEFAULT NULL,
    `tos_accepted`            tinyint(1) NOT NULL DEFAULT 0,
    `tos_accepted_at`         datetime     DEFAULT NULL,
    `tos_accepted_version`    varchar(20)  DEFAULT NULL,
    `tos_accepted_ip`         varchar(45)  DEFAULT NULL,
    `tos_accepted_user_agent` varchar(255) DEFAULT NULL,
    `tos_accepted_os`         varchar(100) DEFAULT NULL,
    `pp_accepted`             tinyint(1) NOT NULL DEFAULT 0,
    `pp_accepted_at`          datetime     DEFAULT NULL,
    `pp_accepted_version`     varchar(20)  DEFAULT NULL,
    `pp_accepted_ip`          varchar(45)  DEFAULT NULL,
    `pp_accepted_user_agent`  varchar(255) DEFAULT NULL,
    `pp_accepted_os`          varchar(100) DEFAULT NULL,
    PRIMARY KEY (`node_id`),
    UNIQUE KEY `email` (`email`),
    KEY `tos_accepted_version` (`tos_accepted_version`),
    KEY `pp_accepted_version` (`pp_accepted_version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Consent columns keep their defaults: nobody has accepted anything yet, and
-- inventing an acceptance would be a lie in a legal record.
INSERT INTO auth_accounts
    (node_id, email, password, totp_secret, totp_enabled,
     totp_recovery_codes, totp_last_verified_at)
SELECT node_id, email, password, totp_secret, totp_enabled,
       totp_recovery_codes, totp_last_verified_at
FROM people_people;

-- Everything worth keeping is now on `nodes` and in `auth_accounts`.
--
-- Not carried over, and gone once this runs: given_name and family_name
-- (truncated at the first non-ASCII byte, while nodes.name is intact),
-- username (identical to nodes.alias on every row), time_zone and language
-- (identical to nodes.timezone and nodes.language), network_presence and
-- activation_code (dead), and gender — replaced by nodes.person_pronouns,
-- which is free text; "male"/"female" is not a pronoun and converting it
-- would be guessing on somebody's behalf.
--
-- This file only ever runs against a copy of a legacy database, and the
-- original is still wherever it was dumped from, so there is nothing here to
-- lose that is not already held somewhere else.
DROP TABLE people_people;

-- --------------------------------------------------------

-- Some data updates for now, remove it in the final commit
UPDATE nodes SET filename = '8944e519-1d9f-46c4-a16f-3da487ade61f.jpg' WHERE alias = 'rastin';
UPDATE nodes SET filename = '423f925ce9bb8dce0c8f1876778a96a8.jpg' WHERE alias = 'rmundel';
