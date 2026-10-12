-- Recommendations: what each person has chosen about suggestions, and the
-- suggestions they turned down.
--
-- GUARDED by IF NOT EXISTS: entrypoint.sh runs schema/init.sql on an empty
-- database and stamps the versions afterwards, so a migration must survive
-- being run against a schema that already has its change.

-- +goose Up
-- What somebody has chosen about being suggested profiles to follow, and
-- about being suggested to others. One row a person, and only once they have
-- changed something: no row is every choice at its default, which is yes.
--
-- Owned by recommendation-service. No foreign keys, like the rest of this
-- schema.
CREATE TABLE IF NOT EXISTS recommendation_preferences (
    `person_id` bigint unsigned NOT NULL,
    -- Whether to be suggested people, and groups.
    `suggest_people` tinyint(1) NOT NULL DEFAULT 1,
    `suggest_groups` tinyint(1) NOT NULL DEFAULT 1,
    -- Whether what they liked and wrote may be used to choose for them.
    `use_activity` tinyint(1) NOT NULL DEFAULT 1,
    -- Whether they may be suggested to other people.
    `discoverable` tinyint(1) NOT NULL DEFAULT 1,
    `updated_at` datetime NOT NULL,
    PRIMARY KEY (`person_id`),
    KEY `idx_recommendation_preferences_discoverable` (`discoverable`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- A suggestion somebody said they were not interested in: that profile is
-- not suggested to them again. Theirs alone to see, and to take back.
CREATE TABLE IF NOT EXISTS recommendation_dismissals (
    `person_id` bigint unsigned NOT NULL,
    `actor_id` bigint unsigned NOT NULL,
    `created_at` datetime NOT NULL,
    PRIMARY KEY (`person_id`, `actor_id`),
    KEY `idx_recommendation_dismissals_actor` (`actor_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE IF EXISTS recommendation_dismissals;
DROP TABLE IF EXISTS recommendation_preferences;
