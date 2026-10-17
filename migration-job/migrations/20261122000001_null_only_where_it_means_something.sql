-- The last migration before the schema is folded into schema/init.sql
-- (roadmap row 27). The same change ends legacy/upgrade.sql, and
-- schema/init.sql declares the columns so.
--
-- NULL only where it means something. A reference to another row is NULL
-- when there is none, never 0; a place is NULL when there is none, and 0,0
-- was written for none; a date is NULL when what it dates has not happened
-- or does not apply (an event's start and end, on an event only). Every
-- other column says what it holds: '' for no text, 0 for no count, false
-- for a switch not set. The PHP application left nearly everything
-- nullable, and the code read most of it through COALESCE.

-- +goose Up
-- nodes: the repairs first, while the columns still take what they hold.
UPDATE nodes SET parent_id = NULL WHERE parent_id = 0;
UPDATE nodes SET root_id = NULL WHERE root_id = 0;
UPDATE nodes SET quote_id = NULL WHERE quote_id = 0;
UPDATE nodes SET owner_id = NULL WHERE owner_id = 0;
UPDATE nodes SET created_by = NULL WHERE created_by = 0;
UPDATE nodes SET modified_by = NULL WHERE modified_by = 0;
UPDATE nodes SET geo_latitude = NULL, geo_longitude = NULL
WHERE (geo_latitude = 0 AND geo_longitude = 0) OR geo_latitude IS NULL OR geo_longitude IS NULL;
UPDATE nodes SET starts_at = NULL, ends_at = NULL WHERE type <> 'node.actor.actor-service.event.v1';
UPDATE nodes SET updated_at = created_at WHERE updated_at IS NULL;

-- What is NULL now takes what the column will say instead.
UPDATE nodes SET
    `name` = COALESCE(`name`, ''),
    `alias` = COALESCE(`alias`, ''),
    `body` = COALESCE(`body`, ''),
    `meta` = COALESCE(`meta`, ''),
    `features` = COALESCE(`features`, ''),
    `owner_type` = COALESCE(`owner_type`, ''),
    `parent_type` = COALESCE(`parent_type`, ''),
    `comment_status` = COALESCE(`comment_status`, 0),
    `comment_count` = COALESCE(`comment_count`, 0),
    `follower_count` = COALESCE(`follower_count`, 0),
    `leader_count` = COALESCE(`leader_count`, 0),
    `subscriber_count` = COALESCE(`subscriber_count`, 0),
    `vote_up_count` = COALESCE(`vote_up_count`, 0),
    `allow_follow_request` = COALESCE(`allow_follow_request`, 0),
    `reply_access` = COALESCE(`reply_access`, ''),
    `quote_policy` = COALESCE(`quote_policy`, ''),
    `website_url` = COALESCE(`website_url`, ''),
    `person_type` = COALESCE(`person_type`, ''),
    `person_pronouns` = COALESCE(`person_pronouns`, ''),
    `geo_address` = COALESCE(`geo_address`, ''),
    `geo_city` = COALESCE(`geo_city`, ''),
    `geo_state_province` = COALESCE(`geo_state_province`, ''),
    `geo_country` = COALESCE(`geo_country`, ''),
    `geo_postalcode` = COALESCE(`geo_postalcode`, ''),
    `filename` = COALESCE(`filename`, ''),
    `avatar_filename` = COALESCE(`avatar_filename`, ''),
    `cover_filename` = COALESCE(`cover_filename`, ''),
    `mimetype` = COALESCE(`mimetype`, ''),
    `language` = COALESCE(`language`, '');

ALTER TABLE nodes
    MODIFY `service` varchar(255) NOT NULL,
    MODIFY `name` varchar(255) NOT NULL DEFAULT '',
    MODIFY `alias` varchar(255) NOT NULL DEFAULT '',
    MODIFY `body` mediumtext NOT NULL DEFAULT (''),
    MODIFY `meta` mediumtext NOT NULL DEFAULT (''),
    MODIFY `features` mediumtext NOT NULL DEFAULT (''),
    MODIFY `owner_type` varchar(255) NOT NULL DEFAULT '',
    MODIFY `parent_type` varchar(255) NOT NULL DEFAULT '',
    MODIFY `comment_status` tinyint(1) NOT NULL DEFAULT 0,
    MODIFY `comment_count` int unsigned NOT NULL DEFAULT 0,
    MODIFY `follower_count` int unsigned NOT NULL DEFAULT 0,
    MODIFY `leader_count` int unsigned NOT NULL DEFAULT 0,
    MODIFY `subscriber_count` int unsigned NOT NULL DEFAULT 0,
    MODIFY `vote_up_count` int unsigned NOT NULL DEFAULT 0,
    MODIFY `allow_follow_request` tinyint unsigned NOT NULL DEFAULT 0,
    MODIFY `reply_access` varchar(64) NOT NULL DEFAULT '',
    MODIFY `quote_policy` varchar(16) NOT NULL DEFAULT '',
    MODIFY `website_url` varchar(255) NOT NULL DEFAULT '',
    MODIFY `person_type` varchar(50) NOT NULL DEFAULT '',
    MODIFY `person_pronouns` varchar(50) NOT NULL DEFAULT '',
    MODIFY `geo_address` varchar(255) NOT NULL DEFAULT '',
    MODIFY `geo_city` varchar(50) NOT NULL DEFAULT '',
    MODIFY `geo_state_province` varchar(50) NOT NULL DEFAULT '',
    MODIFY `geo_country` varchar(30) NOT NULL DEFAULT '',
    MODIFY `geo_postalcode` varchar(15) NOT NULL DEFAULT '',
    MODIFY `filename` varchar(255) NOT NULL DEFAULT '',
    MODIFY `avatar_filename` varchar(255) NOT NULL DEFAULT '',
    MODIFY `cover_filename` varchar(255) NOT NULL DEFAULT '',
    MODIFY `mimetype` varchar(100) NOT NULL DEFAULT '',
    MODIFY `language` varchar(50) NOT NULL DEFAULT '',
    MODIFY `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    MODIFY `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP;

-- edges
UPDATE edges SET created_by = NULL WHERE created_by = 0;
UPDATE edges SET modified_by = NULL WHERE modified_by = 0;
UPDATE edges SET created_at = COALESCE(updated_at, created_at) WHERE created_at IS NULL;
UPDATE edges AS edge
JOIN nodes AS a ON a.id = edge.node_a_id
JOIN nodes AS b ON b.id = edge.node_b_id
SET edge.created_at = GREATEST(a.created_at, b.created_at)
WHERE edge.created_at IS NULL;
UPDATE edges SET created_at = NOW() WHERE created_at IS NULL;
UPDATE edges SET updated_at = created_at WHERE updated_at IS NULL;

-- What is NULL now takes what the column will say instead.
UPDATE edges SET
    `meta` = COALESCE(`meta`, ''),
    `ordering` = COALESCE(`ordering`, 0);

ALTER TABLE edges
    MODIFY `meta` mediumtext NOT NULL DEFAULT (''),
    MODIFY `ordering` int unsigned NOT NULL DEFAULT 0,
    MODIFY `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    MODIFY `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP;

-- notifications
UPDATE notifications SET updated_at = created_at WHERE updated_at IS NULL;
UPDATE notifications SET subject_id = NULL WHERE subject_id = 0;
UPDATE notifications SET object_id = NULL WHERE object_id = 0;
UPDATE notifications SET target_id = NULL WHERE target_id = 0;
UPDATE notifications SET comment_id = NULL WHERE comment_id = 0;

-- What is NULL now takes what the column will say instead.
UPDATE notifications SET
    `subject_type` = COALESCE(`subject_type`, ''),
    `object_type` = COALESCE(`object_type`, ''),
    `target_type` = COALESCE(`target_type`, ''),
    `meta` = COALESCE(`meta`, '');

ALTER TABLE notifications
    MODIFY `recipient_id` bigint unsigned NOT NULL,
    MODIFY `subject_type` varchar(255) NOT NULL DEFAULT '',
    MODIFY `object_type` varchar(255) NOT NULL DEFAULT '',
    MODIFY `target_type` varchar(255) NOT NULL DEFAULT '',
    MODIFY `meta` mediumtext NOT NULL DEFAULT (''),
    MODIFY `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    MODIFY `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP;

-- auth_logs and node_deletion_logs: what a sign-in or a deletion was is
-- always written, the empty string where nothing was known.
-- What is NULL now takes what the column will say instead.
UPDATE auth_logs SET
    `agent_device` = COALESCE(`agent_device`, 'bot'),
    `agent_name` = COALESCE(`agent_name`, ''),
    `agent_os` = COALESCE(`agent_os`, ''),
    `ip` = COALESCE(`ip`, ''),
    `city` = COALESCE(`city`, ''),
    `country` = COALESCE(`country`, '');

ALTER TABLE auth_logs
    MODIFY `agent_device` varchar(50) NOT NULL DEFAULT 'bot',
    MODIFY `agent_name` varchar(255) NOT NULL DEFAULT '',
    MODIFY `agent_os` varchar(255) NOT NULL DEFAULT '',
    MODIFY `ip` varchar(255) NOT NULL DEFAULT '',
    MODIFY `city` varchar(255) NOT NULL DEFAULT '',
    MODIFY `country` varchar(255) NOT NULL DEFAULT '',
    MODIFY `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP;

-- What is NULL now takes what the column will say instead.
UPDATE node_deletion_logs SET
    `node_alias` = COALESCE(`node_alias`, ''),
    `node_type` = COALESCE(`node_type`, '');

ALTER TABLE node_deletion_logs
    MODIFY `node_alias` varchar(255) NOT NULL DEFAULT '',
    MODIFY `node_type` varchar(255) NOT NULL DEFAULT '';

-- +goose Down
-- Nothing: the NULLs it took out meant nothing, and putting them back would
-- bring back the bugs they hid.
