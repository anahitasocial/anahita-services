-- +goose Up
-- A person moving their account, out and in (anahita-web roadmap row 124,
-- part 3): the old accounts this one also is (its alsoKnownAs), and the
-- account it moved to, and when. Owned by activitypub-service; auth-service
-- reads moved_at to limit a moved account's tokens.
ALTER TABLE activitypub_accounts
    ADD COLUMN `also_known_as` text DEFAULT NULL AFTER `indexable`,
    ADD COLUMN `moved_to` varchar(512) DEFAULT NULL AFTER `also_known_as`,
    ADD COLUMN `moved_at` datetime DEFAULT NULL AFTER `moved_to`;

-- +goose Down
ALTER TABLE activitypub_accounts
    DROP COLUMN `moved_at`,
    DROP COLUMN `moved_to`,
    DROP COLUMN `also_known_as`;
