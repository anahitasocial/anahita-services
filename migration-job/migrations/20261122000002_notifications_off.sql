-- What a person has turned off being told about (roadmap row 97, part 5):
-- one row for each kind of thing turned off, none for what is on, which is
-- everything to begin with. A notification of a kind turned off is not
-- made.
--
-- Numbered after 20261122000001, the last folded migration, as every
-- migration is from here (migrations/README.md).

-- +goose Up
CREATE TABLE notifications_off (
    `recipient_id` bigint unsigned NOT NULL,
    `topic` varchar(32) NOT NULL,
    `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`recipient_id`, `topic`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE notifications_off;
