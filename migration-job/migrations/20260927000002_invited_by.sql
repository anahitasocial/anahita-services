-- Records who invited a person, on the account that invitation created.
--
-- Onboarding shows the inviter first among the accounts it offers to follow.
-- The invitation token has created_by, but tokens are not a durable record:
-- they are consumed, swept and expire, and nothing promises one survives the
-- account it made.
--
-- Written at invite-accept from the token, and at signup-request approval from
-- signup_requests.invited_by, for applications a member vouched for.
-- Self-serve signups leave it NULL — nobody invited them, and 0 would point at
-- an actor that does not exist.
--
-- EXISTING ACCOUNTS ARE LEFT NULL. The tokens that could say who invited them
-- are long gone, and guessing is worse than not knowing.
--
-- No foreign key, like every other actor reference in this schema: an inviter
-- who deletes their account leaves a tombstone node behind, and the reader
-- treats one that no longer resolves as no inviter at all.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_invited_by()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'auth_accounts'
          AND COLUMN_NAME = 'invited_by'
    ) THEN
        ALTER TABLE auth_accounts
            ADD COLUMN `invited_by` bigint unsigned DEFAULT NULL;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_invited_by();
DROP PROCEDURE anahita_invited_by;

-- +goose Down
-- +goose StatementBegin
CREATE PROCEDURE anahita_invited_by_down()
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'auth_accounts'
          AND COLUMN_NAME = 'invited_by'
    ) THEN
        ALTER TABLE auth_accounts DROP COLUMN `invited_by`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_invited_by_down();
DROP PROCEDURE anahita_invited_by_down;
