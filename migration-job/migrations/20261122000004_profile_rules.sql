-- A group's and an event's rules (roadmap row 102, part 5): ticked one by one
-- before following it or answering it, and again when they change. Written by
-- its administrators, at most ten; the version goes up with every change, and
-- what each person agreed to is kept beside it. The site's own rules are in
-- site.json, and agreed to in auth_accounts.rules_accepted_*.
--
-- Numbered after 20261122000001, the last folded migration, as every
-- migration is from here (migrations/README.md).

-- +goose Up
CREATE TABLE profile_rules (
    `profile_id` bigint unsigned NOT NULL,
    `position` tinyint unsigned NOT NULL,
    `title` varchar(100) NOT NULL,
    `body` varchar(300) NOT NULL DEFAULT '',
    PRIMARY KEY (`profile_id`, `position`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE profile_rule_versions (
    `profile_id` bigint unsigned NOT NULL,
    `version` int unsigned NOT NULL DEFAULT 0,
    `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`profile_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE profile_rule_agreements (
    `person_id` bigint unsigned NOT NULL,
    `profile_id` bigint unsigned NOT NULL,
    `version` int unsigned NOT NULL,
    `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`person_id`, `profile_id`),
    KEY `profile_id` (`profile_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE profile_rule_agreements;
DROP TABLE profile_rule_versions;
DROP TABLE profile_rules;
