-- The approval queue: people who have asked to join and are waiting on a
-- decision.
--
-- NOT accounts. An application has no node and no auth_accounts row until it
-- is approved, and that is the design rather than an implementation detail:
--
--   * a disabled actor per application fills the graph with people who were
--     never admitted, and publishes them once this instance federates;
--   * it consumes usernames, so a rejected applicant holds a name forever;
--   * and "is this a member" stops being answerable by looking at `nodes`.
--
-- The password hash is stored here so approval does not have to ask for one
-- again. It is bcrypt, the same as auth_accounts, and it moves across
-- untouched when the account is created.
--
-- `username` is deliberately NOT unique. Two people may apply for the same
-- name; the first to be approved gets it, and the second is told at that
-- point. Enforcing it here would refuse the second application on behalf of
-- one that may itself be rejected.
--
-- `email` IS unique — one application per address, so a queue cannot be filled
-- by resubmitting.

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS registration_applications (
    `id`       bigint unsigned NOT NULL AUTO_INCREMENT,
    `username` varchar(32)  NOT NULL,
    `email`    varchar(255) NOT NULL,
    `password` varchar(255) NOT NULL,

    -- What an administrator is deciding on. Required in this mode, because
    -- deciding on an email address alone is not deciding.
    `name`        varchar(100) DEFAULT NULL,
    `body`        text         DEFAULT NULL,
    `website_url` varchar(255) DEFAULT NULL,

    -- unverified -> pending -> approved | rejected
    --
    -- An application reaches the queue only once its address is confirmed, so
    -- administrators are not moderating junk that nobody can even receive mail
    -- at. `unverified` is where it waits until then.
    `status` varchar(20) NOT NULL DEFAULT 'unverified',
    `email_verified_at` datetime DEFAULT NULL,

    -- Who vouched, when a member's invitation put them here rather than a
    -- cold application. An administrator's invitation never reaches this
    -- table — it is already a decision.
    `invited_by` bigint unsigned DEFAULT NULL,

    `decided_by`    bigint unsigned DEFAULT NULL,
    `decided_at`    datetime        DEFAULT NULL,
    `decision_note` varchar(255)    DEFAULT NULL,

    -- Where the application came from. The same evidential trio the consent
    -- columns carry, and for the same reason: a queue entry that cannot say
    -- where it came from is harder to moderate, and a spam wave is usually
    -- recognisable by its source long before its content.
    `created_ip`         varchar(45)  DEFAULT NULL,
    `created_user_agent` varchar(255) DEFAULT NULL,

    `created_at` datetime NOT NULL,
    `updated_at` datetime NOT NULL,

    PRIMARY KEY (`id`),
    UNIQUE KEY `email` (`email`),
    KEY `status` (`status`),
    KEY `username` (`username`),
    KEY `created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS registration_applications;
-- +goose StatementEnd
