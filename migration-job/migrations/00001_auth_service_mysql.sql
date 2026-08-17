-- +goose Up

-- Brings an existing database in line with the auth-service repository
-- layer after the port from the upstream Postgres schema to MySQL.
--
-- Every statement is guarded against already having been applied. The
-- entrypoint runs schema/init.sql on an empty database and exits
-- without stamping a goose version, so a database created that way can
-- later reach this migration with the changes already in place. The
-- guards use prepared statements over information_schema, the same
-- pattern legacy/upgrade.sql already uses.

-- oauth_clients.first_party — read and written by every client query.
-- Distinct from skip_consent: a first-party client may still show a
-- consent screen, and a third-party client is never ours regardless of
-- how consent is configured.
SET @col_exists = (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'oauth_clients'
      AND COLUMN_NAME = 'first_party'
);
SET @stmt = IF(@col_exists = 0,
    'ALTER TABLE oauth_clients ADD COLUMN `first_party` tinyint(1) NOT NULL DEFAULT 0 AFTER `skip_consent`',
    'SELECT 1');
PREPARE s FROM @stmt;
EXECUTE s;
DEALLOCATE PREPARE s;

-- create_time -> created_at. The repository selects created_at, and
-- every other oauth_* table already uses the _at suffix.
SET @col_exists = (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'oauth_clients'
      AND COLUMN_NAME = 'create_time'
);
SET @stmt = IF(@col_exists = 1,
    'ALTER TABLE oauth_clients CHANGE `create_time` `created_at` datetime NOT NULL',
    'SELECT 1');
PREPARE s FROM @stmt;
EXECUTE s;
DEALLOCATE PREPARE s;

SET @col_exists = (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'oauth_clients'
      AND COLUMN_NAME = 'update_time'
);
SET @stmt = IF(@col_exists = 1,
    'ALTER TABLE oauth_clients CHANGE `update_time` `updated_at` datetime NOT NULL',
    'SELECT 1');
PREPARE s FROM @stmt;
EXECUTE s;
DEALLOCATE PREPARE s;

-- Timestamp defaults. None of the oauth inserts name created_at, and
-- under MySQL strict mode a NOT NULL column with no default and no
-- supplied value is error 1364 rather than a silent zero. Repeating an
-- identical MODIFY is a no-op, so these need no guard.
ALTER TABLE oauth_authorizations
    MODIFY `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP;

ALTER TABLE oauth_refresh_tokens
    MODIFY `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP;

ALTER TABLE oauth_signing_keys
    MODIFY `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP;

-- oauth_clients also writes neither created_by (seeded clients have no
-- creating person) nor updated_at.
ALTER TABLE oauth_clients
    MODIFY `created_by` bigint(11) unsigned NOT NULL DEFAULT 0,
    MODIFY `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    MODIFY `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP;

-- Single-use tokens emailed to a person: password resets and invites.
-- created_by / updated_by are node ids, the same soft reference
-- auth_logs.created_by uses.
CREATE TABLE IF NOT EXISTS tokens (
    `id` SERIAL,
    `recipient_email` varchar(255) NOT NULL,
    `type` varchar(50) NOT NULL,
    `body` varchar(255) NOT NULL,
    `start_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
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
) ENGINE=InnoDB CHARACTER SET=utf8mb4;

-- WebAuthn (passkey) credentials. user_id is a node id — a soft
-- reference, so person deletion must call DeleteByPersonID explicitly.
CREATE TABLE IF NOT EXISTS webauthn_credentials (
    `id` SERIAL,
    `user_id` bigint(11) unsigned NOT NULL,
    `credential_id` varbinary(255) NOT NULL,
    `public_key` blob NOT NULL,
    `attestation_type` varchar(255) NOT NULL DEFAULT '',
    `aaguid` varbinary(16) DEFAULT NULL,
    `sign_count` bigint(20) unsigned NOT NULL DEFAULT 0,
    `clone_warning` tinyint(1) NOT NULL DEFAULT 0,
    `transports` JSON NOT NULL,
    `backup_eligible` tinyint(1) NOT NULL DEFAULT 0,
    `backup_state` tinyint(1) NOT NULL DEFAULT 0,
    `nickname` varchar(255) NOT NULL,
    `last_used_at` datetime DEFAULT NULL,
    `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_webauthn_credentials_credential_id` (`credential_id`),
    KEY `idx_webauthn_credentials_user_id` (`user_id`)
) ENGINE=InnoDB CHARACTER SET=utf8mb4;

-- +goose Down

DROP TABLE IF EXISTS webauthn_credentials;
DROP TABLE IF EXISTS tokens;

ALTER TABLE oauth_clients CHANGE `updated_at` `update_time` datetime NOT NULL;
ALTER TABLE oauth_clients CHANGE `created_at` `create_time` datetime NOT NULL;
ALTER TABLE oauth_clients DROP COLUMN `first_party`;
