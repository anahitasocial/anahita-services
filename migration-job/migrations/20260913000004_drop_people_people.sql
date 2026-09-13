-- Drops `people_people`. This is the destructive half of the refactor.
--
-- 20260913000003 moved everything worth keeping onto `nodes` and into
-- `auth_accounts` but left the old table in place, so the services could be
-- switched over one at a time with a working stack after each step. That is
-- done: no Go code reads or writes `people_people` any more.
--
-- The backfill runs again here, as INSERT IGNORE and a guarded UPDATE, because
-- the two migrations are separated by however long a deployment took to roll
-- out. Any account created in that window has a `people_people` row written by
-- the old auth-service and no `auth_accounts` row; without this second pass it
-- would be dropped along with the table and the person locked out.
--
-- What is NOT carried over, and is gone for good once this runs:
--   given_name, family_name  -- nodes.name holds the correct value; these are
--                               truncated at the first non-ASCII byte
--   username                 -- identical to nodes.alias on every row
--   time_zone, language      -- identical to nodes.timezone / nodes.language
--   gender                   -- replaced by nodes.person_pronouns, which is
--                               free text; "male"/"female" is not a pronoun and
--                               converting it would be guessing on someone's
--                               behalf
--   network_presence         -- 0 on every row
--   activation_code          -- no Go reader
--
-- Take a dump first if any of that matters to you. `gender` in particular is
-- set on a third of accounts and there is no way back afterwards.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_drop_people_people()
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.TABLES
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'people_people'
    ) THEN
        -- Catch up anything created since 20260913000003.
        --
        -- The UPDATE is narrowed to rows that have not been carried over yet,
        -- so it cannot undo an edit made through the new path: a person who
        -- changed their pronouns or was promoted after the previous migration
        -- has a person_type on `nodes` that `people_people.usertype` no longer
        -- knows about, and a blanket UPDATE would write the stale value back.
        UPDATE nodes n JOIN people_people p ON p.node_id = n.id
        SET n.person_type     = p.usertype,
            n.last_visit_date = p.last_visit_date
        WHERE n.person_type IS NULL OR n.person_type = '';

        -- IGNORE rather than REPLACE, for the same reason: an account whose
        -- password or TOTP state changed after the previous migration must
        -- keep the newer `auth_accounts` row, not have the old one written
        -- back over it. Only accounts with no row at all are inserted.
        INSERT IGNORE INTO auth_accounts
            (node_id, email, password, totp_secret, totp_enabled,
             totp_recovery_codes, totp_last_verified_at)
        SELECT node_id, email, password, totp_secret, totp_enabled,
               totp_recovery_codes, totp_last_verified_at
        FROM people_people;

        DROP TABLE people_people;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_drop_people_people();
DROP PROCEDURE anahita_drop_people_people;

-- +goose Down
-- +goose StatementBegin
-- There is no down. The table is gone and its contents with it.
--
-- Recreating it empty would be worse than refusing: the old auth-service reads
-- credentials from it, so a rollback onto an empty table is a deployment where
-- every sign-in fails and every password reset silently does nothing — a
-- failure that looks like a broken database rather than a bad rollback.
--
-- Restore from a dump instead, then re-run the migrations from
-- 20260913000003. This raises rather than returning quietly, so a rollback
-- stops here with a message instead of reporting success and leaving the
-- operator to discover the state later.
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT =
    'people_people cannot be rebuilt: restore it from a dump taken before 20260913000004';
-- +goose StatementEnd
