-- The version of the site's rules a sign-up request agreed to, each ticked
-- on the form (roadmap row 102, part 2b): recorded on the account when an
-- administrator approves it, with where the request came from.
--
-- Numbered after 20261122000001, the last folded migration, as every
-- migration is from here (migrations/README.md).

-- +goose Up
ALTER TABLE signup_requests
    ADD COLUMN `rules_accepted_version` varchar(20) NOT NULL DEFAULT '' AFTER `created_user_agent`;

-- +goose Down
ALTER TABLE signup_requests
    DROP COLUMN `rules_accepted_version`;
