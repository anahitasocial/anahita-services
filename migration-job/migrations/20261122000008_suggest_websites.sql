-- Websites among the suggestions (anahita-web roadmap row 111): whether
-- somebody is suggested websites, as they choose for people, groups and
-- events. Yes until they say otherwise.

-- +goose Up
ALTER TABLE recommendation_preferences
    ADD COLUMN `suggest_websites` tinyint(1) NOT NULL DEFAULT 1 AFTER `suggest_events`;

-- +goose Down
ALTER TABLE recommendation_preferences
    DROP COLUMN `suggest_websites`;
