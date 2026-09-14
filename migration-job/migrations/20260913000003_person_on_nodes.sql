-- Moves person identity onto `nodes`, and credentials into `auth_accounts`.
--
-- Additive only. `people_people` is untouched and still holds everything it
-- did, so the running code keeps working; a later migration drops it once
-- nothing reads it. Landing the two halves separately is what keeps every step
-- of this refactor deployable on its own.
--
-- What lands here:
--   nodes.person_type      <- people_people.usertype
--   nodes.person_pronouns  <- new, replacing `gender`
--   nodes.last_visit_date  <- people_people.last_visit_date
--   nodes.unique_alias     <- new, the actor handle namespace
--   auth_accounts          <- the credential and consent half
--
-- Not carried over: given_name and family_name (nodes.name already holds the
-- correct value and these are truncated at the first non-ASCII byte), username
-- (identical to nodes.alias on every row), time_zone and language (identical to
-- nodes.timezone and nodes.language), network_presence and activation_code
-- (dead), and gender (dropped rather than guessed at).
--
-- GUARDED, for the same reason as the migrations before it: entrypoint.sh runs
-- schema/init.sql on an empty database and stamps the versions afterwards, so a
-- migration must survive being run against a schema that already has its
-- change.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_person_on_nodes()
BEGIN
    -- 1. The person columns.
    --
    -- `person_type` is named for the node, not for auth: the graph owns the
    -- person, and "user" is reserved for protocol surface an external
    -- specification names that way. Unlike owner_type and parent_type it holds
    -- an authority level, not a node type string.
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = 'person_type'
    ) THEN
        ALTER TABLE nodes ADD COLUMN `person_type` varchar(50) DEFAULT NULL;
        ALTER TABLE nodes ADD INDEX `person_type` (`person_type`);
    END IF;

    -- Free text, e.g. "she/her", "iel". Never an enum: an enum hard-codes
    -- English grammar and is meaningless in languages with no gendered
    -- third-person pronoun. Never used for logic, and empty renders nothing.
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = 'person_pronouns'
    ) THEN
        ALTER TABLE nodes ADD COLUMN `person_pronouns` varchar(50) DEFAULT NULL;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = 'last_visit_date'
    ) THEN
        ALTER TABLE nodes ADD COLUMN `last_visit_date` datetime DEFAULT NULL;
    END IF;

    -- 2. Backfill from people_people, before the unique index exists.
    --
    -- name, alias, timezone and language are deliberately absent: they are
    -- already correct on nodes, verified identical on every row.
    IF EXISTS (
        SELECT 1 FROM information_schema.TABLES
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'people_people'
    ) THEN
        UPDATE nodes n JOIN people_people p ON p.node_id = n.id
        SET n.person_type     = p.usertype,
            n.last_visit_date = p.last_visit_date;
    END IF;

    -- 3. The actor handle namespace.
    --
    -- nodes.alias has no index at all, so handle uniqueness is currently
    -- enforced only by people_people.username — which this refactor removes.
    -- Without this the guarantee would disappear silently.
    --
    -- It has to be actor-scoped. A global unique index on alias is impossible:
    -- article revisions share their article's slug, and a location, a photo-set
    -- and its photos share a place slug. MySQL has no partial indexes, hence a
    -- generated column that is NULL for everything else — a unique index
    -- permits any number of NULLs.
    --
    -- All actor types share one namespace because WebFinger resolves
    -- acct:name@domain to exactly one actor and has no type dimension: a person
    -- and a group with the same alias would make both unaddressable the moment
    -- this instance federates.
    --
    -- The `actor:` prefix is redundant with a single branch. It is there so a
    -- second namespace — hashtags, which are addressed by alias and need
    -- uniqueness of their own — is one CASE line rather than a new column and a
    -- new index.
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = 'unique_alias'
    ) THEN
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
    END IF;

    -- 4. The auth half.
    --
    -- Named auth_accounts rather than auth_credentials: it holds legal consent
    -- as well as secrets, and "credentials" is the wrong label for a record of
    -- what somebody agreed to. One table rather than two — it is 1:1 on
    -- node_id, so splitting would buy a join and nothing else.
    CREATE TABLE IF NOT EXISTS auth_accounts (
        `node_id`               bigint unsigned NOT NULL,
        `email`                 varchar(255) DEFAULT NULL,
        `password`              varchar(255) DEFAULT NULL,
        `totp_secret`           varchar(255) DEFAULT '',
        `totp_enabled`          tinyint(1) NOT NULL DEFAULT 0,
        `totp_recovery_codes`   mediumtext DEFAULT NULL,
        `totp_last_verified_at` datetime DEFAULT NULL,

        -- Terms of Service acceptance. The version is recorded alongside the
        -- timestamp so a revision can re-prompt only those on an older version,
        -- and the ip/user_agent/os trio is what makes the record evidential
        -- rather than merely a boolean.
        --
        -- *_accepted_at defaults to NULL, not NOW(): a row that never accepted
        -- anything must not carry a timestamp, or (accepted=0, accepted_at=<a
        -- date>) is ambiguous and the audit trail becomes unreadable.
        `tos_accepted`            tinyint(1) NOT NULL DEFAULT 0,
        `tos_accepted_at`         datetime     DEFAULT NULL,
        `tos_accepted_version`    varchar(20)  DEFAULT NULL,
        `tos_accepted_ip`         varchar(45)  DEFAULT NULL,
        `tos_accepted_user_agent` varchar(255) DEFAULT NULL,
        `tos_accepted_os`         varchar(100) DEFAULT NULL,

        -- Privacy Policy acceptance, same shape.
        `pp_accepted`             tinyint(1) NOT NULL DEFAULT 0,
        `pp_accepted_at`          datetime     DEFAULT NULL,
        `pp_accepted_version`     varchar(20)  DEFAULT NULL,
        `pp_accepted_ip`          varchar(45)  DEFAULT NULL,
        `pp_accepted_user_agent`  varchar(255) DEFAULT NULL,
        `pp_accepted_os`          varchar(100) DEFAULT NULL,

        PRIMARY KEY (`node_id`),
        UNIQUE KEY `email` (`email`),
        -- Revising the terms means finding everyone still on an older version.
        KEY `tos_accepted_version` (`tos_accepted_version`),
        KEY `pp_accepted_version` (`pp_accepted_version`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

    -- 5. Backfill the credentials.
    --
    -- INSERT IGNORE so a re-run against a partly migrated database does not
    -- fail on the primary key. Consent columns keep their defaults: nobody has
    -- accepted anything yet, and inventing an acceptance would be a lie in a
    -- legal record.
    IF EXISTS (
        SELECT 1 FROM information_schema.TABLES
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'people_people'
    ) THEN
        INSERT IGNORE INTO auth_accounts
            (node_id, email, password, totp_secret, totp_enabled,
             totp_recovery_codes, totp_last_verified_at)
        SELECT node_id, email, password, totp_secret, totp_enabled,
               totp_recovery_codes, totp_last_verified_at
        FROM people_people;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_person_on_nodes();
DROP PROCEDURE anahita_person_on_nodes;

-- +goose Down
-- +goose StatementBegin
CREATE PROCEDURE anahita_person_on_nodes_down()
BEGIN
    DROP TABLE IF EXISTS auth_accounts;

    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = 'unique_alias'
    ) THEN
        ALTER TABLE nodes DROP INDEX `uq_unique_alias`;
        ALTER TABLE nodes DROP COLUMN `unique_alias`;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = 'last_visit_date'
    ) THEN
        ALTER TABLE nodes DROP COLUMN `last_visit_date`;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = 'person_pronouns'
    ) THEN
        ALTER TABLE nodes DROP COLUMN `person_pronouns`;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = 'person_type'
    ) THEN
        ALTER TABLE nodes DROP INDEX `person_type`;
        ALTER TABLE nodes DROP COLUMN `person_type`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_person_on_nodes_down();
DROP PROCEDURE anahita_person_on_nodes_down;
