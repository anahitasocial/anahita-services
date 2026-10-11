-- Suggestions of events, and each person's own weights.
--
-- Two more things somebody chooses about suggestions: whether to be
-- suggested events, and how much each reason counts for when choosing for
-- them. Weights are kept as JSON by the reason's name, and empty until they
-- are set: the service's own are then used.
--
-- GUARDED, like the migrations before it: schema/init.sql has the columns
-- already on a new database.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_recommendation_weights()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'recommendation_preferences'
          AND COLUMN_NAME = 'suggest_events'
    ) THEN
        ALTER TABLE recommendation_preferences
            ADD COLUMN `suggest_events` tinyint(1) NOT NULL DEFAULT 1 AFTER `suggest_groups`;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'recommendation_preferences'
          AND COLUMN_NAME = 'weights'
    ) THEN
        ALTER TABLE recommendation_preferences
            ADD COLUMN `weights` varchar(1000) NOT NULL DEFAULT '' AFTER `discoverable`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_recommendation_weights();
DROP PROCEDURE anahita_recommendation_weights;

-- +goose Down
ALTER TABLE recommendation_preferences DROP COLUMN `weights`;
ALTER TABLE recommendation_preferences DROP COLUMN `suggest_events`;
