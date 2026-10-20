-- The rule a report says was broken (roadmap row 102, part 6): its title,
-- as the reporter read it, and whose rule it is, 0 for the site's own or
-- the group or event the reported thing belongs to. A group's reports are
-- looked up by the second, for its administrators.
--
-- Numbered after 20261122000001, the last folded migration, as every
-- migration is from here (migrations/README.md).

-- +goose Up
ALTER TABLE abuse_reports
    ADD COLUMN `rule_title` varchar(100) NOT NULL DEFAULT '' AFTER `details`,
    ADD COLUMN `rule_profile_id` bigint unsigned NOT NULL DEFAULT 0 AFTER `rule_title`,
    ADD KEY `idx_abuse_reports_rule_profile` (`rule_profile_id`, `case_id`);

-- +goose Down
ALTER TABLE abuse_reports
    DROP KEY `idx_abuse_reports_rule_profile`,
    DROP COLUMN `rule_profile_id`,
    DROP COLUMN `rule_title`;
