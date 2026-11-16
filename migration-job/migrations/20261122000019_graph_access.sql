-- +goose Up
-- Who may see a profile's followers and whom it follows (roadmap row 151):
-- one of the access levels, or empty for whoever may see the profile.
ALTER TABLE nodes
    ADD COLUMN `graph_access` varchar(32) NOT NULL DEFAULT '' AFTER `access`;

-- +goose Down
ALTER TABLE nodes
    DROP COLUMN `graph_access`;
