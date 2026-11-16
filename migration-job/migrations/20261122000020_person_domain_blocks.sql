-- +goose Up
-- The servers a person blocked for themselves (roadmap row 152): nothing
-- from them, or from a server under them, is shown to that person, and no
-- follow crosses between them.
CREATE TABLE IF NOT EXISTS person_domain_blocks (
    `person_id` bigint unsigned NOT NULL,
    -- The server's domain, lowercase, without a scheme or a port.
    `domain` varchar(255) NOT NULL,
    `created_at` datetime NOT NULL,
    PRIMARY KEY (`person_id`, `domain`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE IF EXISTS person_domain_blocks;
