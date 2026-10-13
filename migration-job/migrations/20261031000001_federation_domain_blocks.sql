-- Federation: the servers this installation will have nothing to do with.
--
--   federation_domain_blocks   one row for each server that is refused.
--                              Nothing is taken from it, nothing is sent to
--                              it, and nobody on it can be looked up. A
--                              server's name also covers every name under
--                              it: example.net refuses social.example.net.
--
-- Written by activitypub-service, for administrators; to be read by the
-- Bluesky service too when there is one.

-- +goose Up
CREATE TABLE IF NOT EXISTS federation_domain_blocks (
    `domain` varchar(255) NOT NULL,
    -- Why, for the administrators who come after. Never shown to anybody
    -- else, and never sent to the server refused.
    `reason` varchar(500) NOT NULL DEFAULT '',
    `created_by` bigint unsigned NOT NULL,
    `created_at` datetime NOT NULL,
    PRIMARY KEY (`domain`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE IF EXISTS federation_domain_blocks;
