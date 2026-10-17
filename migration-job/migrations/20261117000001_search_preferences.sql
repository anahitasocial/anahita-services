-- Each person's own weights for search's Most relevant, as they have
-- their own for recommendations (the user, 2026-10-16).

-- +goose Up
CREATE TABLE IF NOT EXISTS search_preferences (
    `person_id` bigint unsigned NOT NULL,
    -- How much each sign of use counts in their Most relevant, as JSON by
    -- name: a step that multiplies the installation's own weight (graph
    -- ranking.Adjustable, ranking.Steps). Only what they have set.
    `weights` varchar(1000) NOT NULL DEFAULT '',
    `updated_at` datetime NOT NULL,
    PRIMARY KEY (`person_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE IF EXISTS search_preferences;
