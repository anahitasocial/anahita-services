-- +goose Up
-- Whether search engines may index a profile and its posts (roadmap row
-- 174). Every profile there is keeps being found; a person who signs up
-- from now on is made with it off (actor-service), their consent first.
ALTER TABLE nodes
    ADD COLUMN `search_engines` tinyint(1) NOT NULL DEFAULT 1 AFTER `graph_access`;

-- +goose Down
ALTER TABLE nodes
    DROP COLUMN `search_engines`;
