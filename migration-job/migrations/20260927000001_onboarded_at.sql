-- Records whether a person has been through onboarding.
--
-- The web app sends a signed-in person with an incomplete profile — no avatar,
-- no bio — through a walk-through after the agreements. Without a record that
-- they have already been through it, somebody who skipped every step would be
-- sent back on every sign-in, which turns a nudge into a wall.
--
-- Stamped when the person reaches the dashboard through the flow, whether they
-- completed the steps or skipped them. NULL means "has not been through it".
--
-- EXISTING ACCOUNTS ARE LEFT NULL, deliberately. The gate is "incomplete
-- profile AND never onboarded", so a member with an avatar and a bio is not
-- sent anywhere, and a member without them gets the same walk-through as
-- somebody who joined a minute ago — which is the point: that is where most
-- existing members are.
--
-- On the account rather than the node: it is web-app state about a person's
-- own session, never shown to anybody else, and has no business travelling on
-- graphpb.Node to sixteen callers.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_onboarded_at()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'auth_accounts'
          AND COLUMN_NAME = 'onboarded_at'
    ) THEN
        ALTER TABLE auth_accounts
            ADD COLUMN `onboarded_at` datetime DEFAULT NULL;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_onboarded_at();
DROP PROCEDURE anahita_onboarded_at;

-- +goose Down
-- +goose StatementBegin
CREATE PROCEDURE anahita_onboarded_at_down()
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'auth_accounts'
          AND COLUMN_NAME = 'onboarded_at'
    ) THEN
        ALTER TABLE auth_accounts DROP COLUMN `onboarded_at`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_onboarded_at_down();
DROP PROCEDURE anahita_onboarded_at_down;
