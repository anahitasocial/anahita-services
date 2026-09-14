-- Records whether an account's email address has been confirmed.
--
-- Nothing verified an address before this. `signupHandler.Confirm` was a stub
-- returning 202 and was not even routed, so an account was fully usable with
-- an address nobody controlled — which is both the cheapest bot filter there
-- is, missing, and a password-reset path pointing at a stranger's inbox.
--
-- EVERY EXISTING ACCOUNT IS MARKED VERIFIED. They predate the check, they have
-- been signing in for years, and the alternative is locking 822 people out of
-- a site they already belong to in order to confirm something nobody ever
-- asked them to confirm. Verification applies from here forward.
--
-- Stamped with the account's creation date rather than NOW(), so the column
-- does not claim that every member of this installation verified their address
-- on the day of the migration. Where no creation date is recoverable it falls
-- back to now, which is the only honest alternative.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_email_verification()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'auth_accounts'
          AND COLUMN_NAME = 'email_verified_at'
    ) THEN
        ALTER TABLE auth_accounts
            ADD COLUMN `email_verified_at` datetime DEFAULT NULL;

        -- Everyone who already has an account.
        UPDATE auth_accounts a
        LEFT JOIN nodes n ON n.id = a.node_id
        SET a.email_verified_at = COALESCE(n.created_on, NOW());
    END IF;
END;
-- +goose StatementEnd
CALL anahita_email_verification();
DROP PROCEDURE anahita_email_verification;

-- +goose Down
-- +goose StatementBegin
CREATE PROCEDURE anahita_email_verification_down()
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'auth_accounts'
          AND COLUMN_NAME = 'email_verified_at'
    ) THEN
        ALTER TABLE auth_accounts DROP COLUMN `email_verified_at`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_email_verification_down();
DROP PROCEDURE anahita_email_verification_down;
