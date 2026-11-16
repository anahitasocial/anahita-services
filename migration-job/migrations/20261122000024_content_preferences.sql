-- +goose Up
-- What each reader chooses for each kind of content warning (roadmap row
-- 122, phase 2): to see it at once, behind its warning, or not at all.
CREATE TABLE IF NOT EXISTS content_preferences (
    `person_id` bigint unsigned NOT NULL,
    -- As JSON by kind, "show" or "hide": only what is not Warn, which every
    -- kind starts at (anahita-libs entities.WarningChoices).
    `choices` varchar(255) NOT NULL DEFAULT '',
    `updated_at` datetime NOT NULL,
    PRIMARY KEY (`person_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE IF EXISTS content_preferences;
