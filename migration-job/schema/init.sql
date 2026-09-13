-- --------------------------------------------------------

CREATE TABLE nodes (
  -- Not SERIAL. SERIAL expands to BIGINT UNSIGNED AUTO_INCREMENT UNIQUE, so
  -- every row carried a redundant unique index on `id` alongside the primary
  -- key, and every insert maintained both.
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  -- 128 for the same reason as edges: an ASCII identifier from a fixed
  -- grammar, longest 42 characters. It also carries five composite indexes,
  -- so the width is paid for five times over.
  `type` varchar(128) NOT NULL,
  `service` varchar(255) NOT NULL,
  `name` varchar(255),
  `alias` varchar(255),
  `body` mediumtext,
  `start_date` datetime DEFAULT NULL,
  `end_date` datetime DEFAULT NULL,
  `owner_type` varchar(255) DEFAULT NULL,
  `owner_id` bigint(11) unsigned DEFAULT NULL,
  `comment_status` tinyint(1) DEFAULT NULL,
  `comment_count` int(11) unsigned DEFAULT NULL,
  `repost_count` int(11) unsigned NOT NULL DEFAULT 0,
  `quote_count` int(11) unsigned NOT NULL DEFAULT 0,
  `last_comment_id` bigint(11) unsigned DEFAULT NULL,
  `last_comment_by` bigint(11) unsigned DEFAULT NULL,
  `last_comment_on` datetime DEFAULT NULL,
  `ordering` int(11) DEFAULT 0,
  `enabled` tinyint(1) NOT NULL DEFAULT 0,
  `verified` tinyint(1) NOT NULL DEFAULT 0,
  `pinned` tinyint(1) NOT NULL DEFAULT 0,
  `is_default` tinyint(1) NOT NULL DEFAULT 0,
  -- mediumtext, not text: CONVERT TO CHARACTER SET utf8mb4 promotes a text
  -- column to preserve its character capacity, so a database migrated by
  -- legacy/upgrade.sql ends up with mediumtext. Declared here so the two
  -- paths produce the same schema.
  `meta` mediumtext DEFAULT NULL,
  -- One link on an actor profile. Replaces the website/contact/phone trio that
  -- used to be read out of `meta`, none of which any Go request ever accepted.
  `website_url` varchar(255) DEFAULT NULL,
  `hits` int(11) unsigned DEFAULT NULL,
  `parent_id` bigint(11) unsigned DEFAULT NULL,
  `parent_type` varchar(255) DEFAULT NULL,
  `geo_latitude` float(10,6) DEFAULT NULL,
  `geo_longitude` float(10,6) DEFAULT NULL,
  `geo_address` VARCHAR(255) DEFAULT NULL,
  `geo_city` VARCHAR(50) DEFAULT NULL,
  `geo_state_province` VARCHAR(50) DEFAULT NULL,
  `geo_country` VARCHAR(30) DEFAULT NULL,
  `geo_postalcode` VARCHAR(15) DEFAULT NULL,
  `created_on` datetime DEFAULT NULL,
  `created_by` bigint(11) unsigned DEFAULT NULL,
  `modified_on` datetime DEFAULT NULL,
  `modified_by` bigint(11) unsigned DEFAULT NULL,
  `filename` varchar(255) DEFAULT NULL,
  `filesize` int(11) DEFAULT NULL,
  `cover_filename` varchar(255) DEFAULT NULL,
  `cover_filesize` int(11) DEFAULT NULL,
  `cover_mimetype` varchar(100) DEFAULT NULL,
  `status` mediumtext,
  `status_update_time` datetime DEFAULT NULL,
  `follower_count` int(11) unsigned DEFAULT NULL,
  `leader_count` int(11) unsigned DEFAULT NULL,
  `mutual_count` int(11) unsigned DEFAULT NULL,
  `allow_follow_request` tinyint(1) unsigned DEFAULT NULL,
  `subscriber_count` int(11) unsigned DEFAULT NULL,
  `vote_up_count` int(11) unsigned DEFAULT NULL,
  `vote_down_count` int(11) unsigned DEFAULT NULL,
  `excerpt` text,
  `mimetype` varchar(100) DEFAULT NULL,
  -- `timezone`, not `time_zone`: the Go read path selects `node.timezone`
  -- (graph-grpc-service/data/repositories/helpers.go), and this file spelled it
  -- with an underscore while the column the code needs was never created here
  -- at all. A fresh install therefore failed every node read.
  `timezone` int(11) DEFAULT NULL,
  `language` varchar(50) DEFAULT NULL,
  `access` varchar(255) NOT NULL,
  -- The per-node-type optional feature set, as JSON. Named `features` because
  -- that is what the code selects; this file called it
  -- `enabled_optional_features`, which nothing has ever read.
  `features` mediumtext,
  -- When an actor was scheduled for deletion. NULL for everything that is not
  -- deleted, which is almost every row.
  --
  -- Deletion sets this and clears `enabled`; it does not remove anything. The
  -- account stops resolving at every login path immediately, and the sweeper
  -- purges the content after the grace period. Until then the person can sign
  -- in and restore, which the previous hard delete made impossible — there was
  -- no soft delete anywhere and no way back from a misclick.
  --
  -- Only actor nodes carry it. Content owned by a deleted actor stays as it is
  -- until the sweeper runs, so a restore brings the profile back whole.
  `deleted_at` datetime DEFAULT NULL,
  -- Permanent. The content stays readable at its URLs; what stops is being
  -- pushed at people who did not ask for it. Separate from `enabled` and
  -- `deleted_at` because each of the three maps to a different answer, and a
  -- single flag cannot give three.
  `archived_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  -- The sweeper's only query is "actors deleted before X", and it runs against
  -- a column that is NULL for practically every row.
  KEY `deleted_at` (`deleted_at`),
  KEY `archived_at` (`archived_at`),
  KEY `last_comment_by` (`last_comment_by`),
  KEY `created_by` (`created_by`),
  KEY `modified_by` (`modified_by`),
  KEY `type` (`type`),
  KEY `service` (`service`),
  KEY `owner_id` (`owner_id`),
  KEY `parent_id` (`parent_id`),
  KEY `start_date` (`start_date`),
  KEY `end_date` (`end_date`),
  KEY `type_enabled` (`type`,`enabled`),
  KEY `type_modifed_on` (`type`,`modified_on`),
  KEY `type_created_on` (`type`,`created_on`),
  KEY `type_status_update_time` (`type`,`status_update_time`),
  KEY `type_default` (`type`,`is_default`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
-- --------------------------------------------------------

CREATE TABLE edges (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  -- 128, not 255. These are machine-generated ASCII identifiers from a fixed
  -- grammar — `edge.<family>.<name>.v1`, `node.<family>.<service>.<entity>.v1`
  -- — and the longest in the database is 42 characters.
  --
  -- The size is load-bearing, not cosmetic: uc_edge spans all three type
  -- columns, and at varchar(255) under utf8mb4 that key is 3*255*4 + 16 =
  -- 3076 bytes, four over InnoDB's 3072 limit. The table simply will not
  -- create. It fitted only while the schema was utf8 at three bytes a
  -- character.
  `type` varchar(128) NOT NULL,
  `node_a_id` bigint(11) unsigned NOT NULL,
  `node_a_type` varchar(128) NOT NULL,
  `node_b_id` bigint(11) unsigned NOT NULL,
  `node_b_type` varchar(128) NOT NULL,
  `meta` mediumtext,
  `created_on` datetime DEFAULT NULL,
  `created_by` bigint(11) unsigned DEFAULT NULL,
  `modified_on` datetime DEFAULT NULL,
  `modified_by` bigint(11) unsigned DEFAULT NULL,
  `ordering` int(11) unsigned DEFAULT NULL,
  `start_date` datetime DEFAULT NULL,
  `end_date` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `type` (`type`),
  KEY `node_a_id` (`node_a_id`),
  KEY `node_b_id` (`node_b_id`),
  KEY `start_date` (`start_date`),
  KEY `end_date` (`end_date`),
  CONSTRAINT uc_edge UNIQUE(`type`,`node_a_id`,`node_a_type`,`node_b_id`,`node_b_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
-- --------------------------------------------------------

CREATE TABLE people_people (
    `people_person_id` bigint unsigned NOT NULL AUTO_INCREMENT,
    `node_id` BIGINT UNSIGNED NOT NULL,
    `email` varchar(255) DEFAULT NULL,
    `username` varchar(255) DEFAULT NULL,
    `password` varchar(255) DEFAULT NULL,
    `usertype` varchar(50) DEFAULT NULL,
    `gender` varchar(50) DEFAULT NULL,
    `given_name` varchar(255) DEFAULT NULL,
    `family_name` varchar(255) DEFAULT NULL,
    `network_presence` tinyint(3) NOT NULL DEFAULT 0,
    `last_visit_date` datetime DEFAULT NULL,
    `time_zone` int(11) DEFAULT NULL,
    `language` varchar(100) DEFAULT NULL,
    `activation_code` varchar(255) DEFAULT NULL,
    `totp_secret` varchar(255) DEFAULT '',
    `totp_enabled` tinyint(1) NOT NULL DEFAULT 0,
    `totp_recovery_codes` mediumtext DEFAULT NULL,
    `totp_last_verified_at` datetime DEFAULT NULL,
    PRIMARY KEY (`people_person_id`),
    KEY `usertype` (`usertype`),
    UNIQUE KEY `username` (`username`),
    UNIQUE KEY `email` (`email`),
    UNIQUE KEY `node_id` (`node_id`),
    KEY `last_visit_date` (`last_visit_date`)
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
-- --------------------------------------------------------

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

-- Deletion audit, for any node.
--
-- Deletion previously left no record at all: person-service published an event
-- nothing consumed, group-service published nothing, and the only trace was a
-- line in a container's stdout — gone with the next restart.
--
-- Keyed on node_id rather than on an actor, because nothing here is specific to
-- actors: a deleted photo, article or comment wants the same record for the
-- same reason. Actors are simply the first thing to write to it. `node_type`
-- carries the full type string, so one query answers "what happened to this
-- node" and another answers "what has been deleted in this service".
--
-- Deliberately OUTLIVES the node. After the sweeper runs, node_id points at a
-- tombstone rather than a profile — which is the point. An audit trail erased
-- along with its subject is not an audit trail. It holds no personal data
-- beyond the two ids and the alias.
CREATE TABLE node_deletion_logs (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    -- The node that was deleted.
    `node_id` bigint(11) unsigned NOT NULL,
    -- The alias at the time, kept because a tombstone can be scrubbed by a
    -- later erasure request and this is then the only record of what went.
    `node_alias` varchar(255) DEFAULT NULL,
    -- Full type string, e.g. node.actor.group-service.group.v1. Doubles as the
    -- service filter, so no separate column is needed for it.
    `node_type` varchar(255) DEFAULT NULL,
    -- Who asked. Equal to node_id for a self-service account deletion, and for
    -- the sweeper, which acts on a deadline rather than for a person.
    `deleted_by` bigint(11) unsigned NOT NULL,
    -- 'scheduled' when the grace period starts, 'purged' when the sweeper
    -- erases it, 'restored' when it is taken back. Up to three rows for one
    -- node is the normal, complete story. Kept a varchar rather than an ENUM so
    -- adding an action is a code change, not an ALTER on an audit table.
    `action` varchar(20) NOT NULL,
    `created_on` datetime NOT NULL,
    PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE INDEX idx_node_deletion_logs_node_id ON node_deletion_logs (node_id);
CREATE INDEX idx_node_deletion_logs_created_on ON node_deletion_logs (created_on);

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
    -- Defaulted, not supplied by the insert. Under MySQL strict mode a
    -- NOT NULL column with no default and no value is error 1364, not a
    -- silent zero — every one of these rows is written without naming
    -- created_at.
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
    -- screen, and a third-party client must never be treated as ours
    -- regardless of how consent is configured.
    `first_party` tinyint(1) NOT NULL DEFAULT 0,
    `active` tinyint(1) NOT NULL DEFAULT 1,
    -- Defaulted because seeded first-party clients have no creating
    -- person — SeedDefaultClients runs at startup, before any request.
    `created_by` bigint(11) unsigned NOT NULL DEFAULT 0,
    -- created_at/updated_at rather than create_time/update_time: the
    -- repository selects created_at, and every other oauth_* table here
    -- already uses the _at suffix. Both defaulted — neither Add nor
    -- Edit supplies them, and under MySQL strict mode a NOT NULL column
    -- with no default is error 1364 rather than a silent zero.
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
--
-- created_by / updated_by are node ids (nodes.id), the same soft
-- reference auth_logs.created_by uses. No foreign key, matching the
-- rest of this schema.
CREATE TABLE tokens (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    -- Where the token is SENT. For an emailchange token that is the
    -- person's current address, not the one they are moving to: the
    -- current inbox is what has to approve the move, so a stolen session
    -- alone cannot repoint the account.
    `recipient_email` varchar(255) NOT NULL,
    -- The requested address, on emailchange tokens only; NULL on every
    -- other type. Separate from recipient_email precisely so the two
    -- cannot be confused — swapping them turns the confirmation flow
    -- from a control into a way to hand the account away.
    `new_email` varchar(255) DEFAULT NULL,
    -- 'passwordreset' or 'invite' (constants.TokenType*). A varchar
    -- rather than an ENUM so adding a token type is a code change, not
    -- an ALTER TABLE on a table that gets written on every reset.
    `type` varchar(50) NOT NULL,
    -- The token itself: 64 random bytes, hex-encoded. Indexed because
    -- ReadByBody is the lookup every reset/invite link performs.
    `body` varchar(255) NOT NULL,
    -- Defaulted: Token.NewToken sets StartTime in Go but tokens.Add
    -- does not include it in the insert column list.
    `start_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    -- Lifetime in seconds from start_time. Expiry is computed in Go
    -- (Token.SetStatus) rather than stored, so changing the default
    -- does not require rewriting existing rows.
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
-- has enrolled — laptop Touch ID, phone, hardware key.
--
-- user_id is a node id (nodes.id), a soft reference like the rest of
-- this schema. Nothing cleans these rows up automatically, so person
-- deletion must call DeleteByPersonID explicitly: a recycled node id
-- would otherwise inherit a stranger's passkey.
CREATE TABLE webauthn_credentials (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    `user_id` bigint(11) unsigned NOT NULL,
    -- Raw authenticator-generated identifier, stored as bytes rather
    -- than base64url so the login lookup does not depend on encoding
    -- choices matching across library versions. VARBINARY(255) is the
    -- longest a credential ID can be per the WebAuthn spec, and unlike
    -- BLOB it can carry a UNIQUE index directly.
    `credential_id` varbinary(255) NOT NULL,
    -- COSE-encoded public key. The private key never leaves the device.
    `public_key` blob NOT NULL,
    `attestation_type` varchar(255) NOT NULL DEFAULT '',
    -- Authenticator model identifier. Not used for any security
    -- decision — it drives the name/icon shown in the passkey list.
    `aaguid` varbinary(16) DEFAULT NULL,
    -- Monotonic counter for clone detection. Synced platform
    -- authenticators (iCloud Keychain, Google Password Manager) always
    -- report 0, which is expected and not a fault.
    `sign_count` bigint(20) unsigned NOT NULL DEFAULT 0,
    `clone_warning` tinyint(1) NOT NULL DEFAULT 0,
    -- Transport hints ("internal", "hybrid", "usb", "nfc", "ble").
    -- JSON because MySQL has no array type; the Postgres original used
    -- TEXT[]. Read and written through entities.StringSlice.
    `transports` JSON NOT NULL,
    -- BE / BS flags: whether this passkey can sync to a cloud keychain,
    -- and whether it currently is. Together they tell the person
    -- whether the passkey survives losing the device.
    `backup_eligible` tinyint(1) NOT NULL DEFAULT 0,
    `backup_state` tinyint(1) NOT NULL DEFAULT 0,
    -- Person-supplied label. Required: three unlabelled passkeys is not
    -- a list anyone can safely revoke from.
    `nickname` varchar(255) NOT NULL,
    `last_used_at` datetime DEFAULT NULL,
    `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    -- A credential ID identifies exactly one enrolment, globally. This
    -- is the login lookup.
    UNIQUE KEY `idx_webauthn_credentials_credential_id` (`credential_id`),
    KEY `idx_webauthn_credentials_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
