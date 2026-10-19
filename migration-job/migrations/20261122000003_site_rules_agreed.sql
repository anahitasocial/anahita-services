-- The site's rules, agreed to by ticking each one (roadmap row 102, part
-- 2): recorded as the Terms of Service and the Privacy Policy are, with the
-- version agreed to and where from. The rules themselves are the operator's,
-- in site.json (configuration.md, Rules and promises).
--
-- Numbered after 20261122000001, the last folded migration, as every
-- migration is from here (migrations/README.md).

-- +goose Up
ALTER TABLE auth_accounts
    ADD COLUMN `rules_accepted` tinyint(1) NOT NULL DEFAULT 0 AFTER `pp_accepted_os`,
    ADD COLUMN `rules_accepted_at` datetime DEFAULT NULL AFTER `rules_accepted`,
    ADD COLUMN `rules_accepted_version` varchar(20) NOT NULL DEFAULT '' AFTER `rules_accepted_at`,
    ADD COLUMN `rules_accepted_ip` varchar(45) NOT NULL DEFAULT '' AFTER `rules_accepted_version`,
    ADD COLUMN `rules_accepted_user_agent` varchar(255) NOT NULL DEFAULT '' AFTER `rules_accepted_ip`,
    ADD COLUMN `rules_accepted_os` varchar(100) NOT NULL DEFAULT '' AFTER `rules_accepted_user_agent`,
    ADD KEY `rules_accepted_version` (`rules_accepted_version`);

-- +goose Down
ALTER TABLE auth_accounts
    DROP KEY `rules_accepted_version`,
    DROP COLUMN `rules_accepted_os`,
    DROP COLUMN `rules_accepted_user_agent`,
    DROP COLUMN `rules_accepted_ip`,
    DROP COLUMN `rules_accepted_version`,
    DROP COLUMN `rules_accepted_at`,
    DROP COLUMN `rules_accepted`;
