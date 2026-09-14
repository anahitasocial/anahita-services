-- Brings an existing database into line with the rewritten schema/init.sql.
--
-- schema/init.sql had drifted from what the code actually reads. It declared
-- `time_zone`, `enabled_optional_features`, `permissions` and `actor_gender`,
-- and did not declare `features` or `timezone` at all — while the graph read
-- path selects `node.features` and `node.timezone`
-- (graph-grpc-service/data/repositories/helpers.go). A fresh install from that
-- file failed every node read with "Unknown column 'node.features'". The live
-- schema, which came from legacy/upgrade.sql, was the correct one.
--
-- This migration carries the remaining, deliberate differences: column widths,
-- character set, and two indexes. A database whose columns are already
-- correct — anything built by legacy/upgrade.sql — needs only these.
--
-- Ordering inside the Up is load-bearing. See the type-column note below.
--
-- GUARDED, for the same reason as the migrations before it: entrypoint.sh runs
-- schema/init.sql on an empty database and stamps the versions afterwards, so a
-- migration must survive being run against a schema that already has its
-- change.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_schema_normalisation()
BEGIN
    -- 1. Zero dates, FIRST — before anything that rebuilds a table.
    --
    -- Both the MODIFY below and the CONVERT after it rebuild the table, and a
    -- rebuild is a write: under the strict sql_mode every supported MySQL runs
    -- by default, '0000-00-00 00:00:00' is rejected on the way back in and the
    -- ALTER fails with "Incorrect datetime value", leaving the database half
    -- migrated. The legacy data carries these — 123 follow edges on the
    -- reference database, between real people, plus two last_visit_dates.
    --
    -- This has to run before BOTH, which is why it is step one rather than
    -- sitting next to the conversion it obviously belongs with.
    --
    -- NULL rather than a fabricated timestamp: the column is nullable, and
    -- "unknown" is what these actually mean. Inventing an epoch date would make
    -- 123 relationships look like they were created in 1970.
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

    -- 2. The type columns, before the charset conversion.
    --
    -- These hold machine-generated ASCII identifiers from a fixed grammar —
    -- `node.<family>.<service>.<entity>.v1` — and the longest in the reference
    -- database is 42 characters, so varchar(255) was never needed.
    --
    -- It is also unusable at four bytes a character: uc_edge spans all three
    -- edges type columns, and 3*255*4 + 16 = 3076 bytes is four over InnoDB's
    -- 3072 limit. Converting the charset first fails with "Specified key was
    -- too long" and leaves the database half migrated.
    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes'
          AND COLUMN_NAME = 'type' AND CHARACTER_MAXIMUM_LENGTH > 128
    ) THEN
        ALTER TABLE nodes MODIFY `type` varchar(128) NOT NULL;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'edges'
          AND COLUMN_NAME = 'type' AND CHARACTER_MAXIMUM_LENGTH > 128
    ) THEN
        ALTER TABLE edges MODIFY `type` varchar(128) NOT NULL;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'edges'
          AND COLUMN_NAME = 'node_a_type' AND CHARACTER_MAXIMUM_LENGTH > 128
    ) THEN
        ALTER TABLE edges MODIFY `node_a_type` varchar(128) NOT NULL;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'edges'
          AND COLUMN_NAME = 'node_b_type' AND CHARACTER_MAXIMUM_LENGTH > 128
    ) THEN
        ALTER TABLE edges MODIFY `node_b_type` varchar(128) NOT NULL;
    END IF;

    -- 3. utf8mb4 everywhere.
    --
    -- utf8 in MySQL is utf8mb3, which cannot represent an emoji or any
    -- supplementary-plane character. A value containing one is truncated at the
    -- first byte that will not fit rather than rejected, which is how
    -- people_people.given_name came to hold "Jos" for "José" and "Sellstr" for
    -- "Sellström" while nodes.name — already utf8mb4 — kept both intact.
    IF EXISTS (
        SELECT 1 FROM information_schema.TABLES
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes'
          AND TABLE_COLLATION NOT LIKE 'utf8mb4%'
    ) THEN
        ALTER TABLE nodes CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.TABLES
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'edges'
          AND TABLE_COLLATION NOT LIKE 'utf8mb4%'
    ) THEN
        ALTER TABLE edges CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.TABLES
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'people_people'
          AND TABLE_COLLATION NOT LIKE 'utf8mb4%'
    ) THEN
        ALTER TABLE people_people CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
    END IF;

    -- The remaining utf8mb3 tables. These were created by this repository
    -- rather than carried over, and were simply declared with the wrong
    -- charset; nothing about them needs the ordering the three above did.
    IF EXISTS (
        SELECT 1 FROM information_schema.TABLES
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'notifications'
          AND TABLE_COLLATION NOT LIKE 'utf8mb4%'
    ) THEN
        ALTER TABLE notifications CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.TABLES
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'auth_logs'
          AND TABLE_COLLATION NOT LIKE 'utf8mb4%'
    ) THEN
        ALTER TABLE auth_logs CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.TABLES
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'node_deletion_logs'
          AND TABLE_COLLATION NOT LIKE 'utf8mb4%'
    ) THEN
        ALTER TABLE node_deletion_logs CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
    END IF;

    -- 4. The index on nodes.service is still called `component` on any database
    -- that came through legacy/upgrade.sql: the column was renamed and MySQL
    -- carried the index across under its old name.
    IF EXISTS (
        SELECT 1 FROM information_schema.STATISTICS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes'
          AND INDEX_NAME = 'component'
    ) THEN
        ALTER TABLE nodes RENAME INDEX `component` TO `service`;
    END IF;

    -- 5. Redundant unique indexes left by SERIAL.
    --
    -- SERIAL expands to BIGINT UNSIGNED AUTO_INCREMENT UNIQUE, so every table
    -- declared that way carries a unique index beside its primary key, on the
    -- same single column, maintained on every insert for no benefit. Ten tables
    -- had one. init.sql no longer produces any.
    --
    -- Driven off information_schema rather than a hand-written list, so it also
    -- covers installs whose table set differs from the reference. The
    -- conditions are deliberately narrow: unique, not the primary key, exactly
    -- one column, and that column is the table's primary key — which is what
    -- makes it redundant rather than merely similar. goose_db_version is
    -- excluded because goose owns it.
    BEGIN
        DECLARE done INT DEFAULT 0;
        DECLARE t, k VARCHAR(64);
        DECLARE cur CURSOR FOR
            SELECT s.TABLE_NAME, s.INDEX_NAME
            FROM information_schema.STATISTICS s
            JOIN information_schema.STATISTICS pk
              ON  pk.TABLE_SCHEMA = s.TABLE_SCHEMA
              AND pk.TABLE_NAME   = s.TABLE_NAME
              AND pk.INDEX_NAME   = 'PRIMARY'
              AND pk.COLUMN_NAME  = s.COLUMN_NAME
            WHERE s.TABLE_SCHEMA = DATABASE()
              AND s.NON_UNIQUE   = 0
              AND s.INDEX_NAME  <> 'PRIMARY'
              AND s.TABLE_NAME  <> 'goose_db_version'
              AND s.SEQ_IN_INDEX = 1
              AND (SELECT COUNT(*) FROM information_schema.STATISTICS c
                   WHERE c.TABLE_SCHEMA = s.TABLE_SCHEMA
                     AND c.TABLE_NAME   = s.TABLE_NAME
                     AND c.INDEX_NAME   = s.INDEX_NAME) = 1
              AND (SELECT COUNT(*) FROM information_schema.STATISTICS p2
                   WHERE p2.TABLE_SCHEMA = s.TABLE_SCHEMA
                     AND p2.TABLE_NAME   = s.TABLE_NAME
                     AND p2.INDEX_NAME   = 'PRIMARY') = 1
            GROUP BY s.TABLE_NAME, s.INDEX_NAME;
        DECLARE CONTINUE HANDLER FOR NOT FOUND SET done = 1;

        OPEN cur;
        drop_loop: LOOP
            FETCH cur INTO t, k;
            IF done = 1 THEN
                LEAVE drop_loop;
            END IF;
            SET @drop_redundant = CONCAT('ALTER TABLE `', t, '` DROP INDEX `', k, '`');
            PREPARE stmt FROM @drop_redundant;
            EXECUTE stmt;
            DEALLOCATE PREPARE stmt;
        END LOOP;
        CLOSE cur;
    END;
END;
-- +goose StatementEnd
CALL anahita_schema_normalisation();
DROP PROCEDURE anahita_schema_normalisation;

-- +goose Down
-- +goose StatementBegin
CREATE PROCEDURE anahita_schema_normalisation_down()
BEGIN
    -- The index rename reverses cleanly. The column widths and the charset do
    -- not: widening back to varchar(255) under utf8mb4 recreates the key-length
    -- failure uc_edge hit in the first place, and converting back to utf8mb3
    -- would silently truncate anything outside the basic plane that has been
    -- written since. Neither is a safe automatic reversal, so this Down does
    -- what can be undone and leaves what cannot.
    IF EXISTS (
        SELECT 1 FROM information_schema.STATISTICS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'nodes'
          AND INDEX_NAME = 'service'
    ) THEN
        ALTER TABLE nodes RENAME INDEX `service` TO `component`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_schema_normalisation_down();
DROP PROCEDURE anahita_schema_normalisation_down;
