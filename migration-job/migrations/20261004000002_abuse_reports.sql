-- Abuse reports: what people have reported, and what was decided about it.
--
-- Two tables. A CASE is one reported node and its decision; a REPORT is one
-- person's complaint inside a case. A post that twenty people report is one
-- case for an administrator to handle, not twenty.
--
-- Owned by abuse-report-service. No foreign keys, like the rest of this
-- schema: a case must outlive the node it is about, which is the point of
-- keeping the target's type and owner on the row.
--
-- CREATE TABLE IF NOT EXISTS, for the same reason the migrations before it
-- are guarded: entrypoint.sh runs schema/init.sql on an empty database and
-- stamps the versions afterwards, so a migration must survive being run
-- against a schema that already has its change.

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS abuse_cases (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    -- What was reported: any node. A person, a group, a post, a comment, a
    -- hashtag or a place.
    `target_id` bigint unsigned NOT NULL,
    -- Its type when it was first reported, kept so the case still says what
    -- it was about after the node is gone.
    `target_type` varchar(255) NOT NULL,
    -- Who was responsible for it then: the author of a post or comment, the
    -- actor itself for a person or group. NULL for a hashtag or a place.
    `target_owner_id` bigint unsigned DEFAULT NULL,
    -- 'open', 'actioned' or 'dismissed'.
    `status` varchar(16) NOT NULL DEFAULT 'open',
    -- The target, while the case is open, and NULL once it is closed. Unique,
    -- which is what makes "one open case per node" a rule of the database
    -- rather than of whichever request got there first: two people reporting
    -- the same post at the same moment cannot open two cases. A closed case
    -- stores NULL, and any number of NULLs may share a unique index, so a
    -- node can be reported again later and its history is kept.
    `open_target_id` bigint unsigned GENERATED ALWAYS AS (IF(`status` = 'open', `target_id`, NULL)) STORED,
    -- How many people have reported it in this case.
    `report_count` int unsigned NOT NULL DEFAULT 0,
    `first_reported_at` datetime NOT NULL,
    `last_reported_at` datetime NOT NULL,
    -- The decision, once somebody has made one.
    `decided_by` bigint unsigned DEFAULT NULL,
    `decided_at` datetime DEFAULT NULL,
    -- What was done about it: 'none', 'deleted', 'disabled', 'archived',
    -- 'blocked' or 'other'.
    `decision_action` varchar(16) DEFAULT NULL,
    `decision_note` text,
    -- When the target was deleted or purged, if it was.
    `target_gone_at` datetime DEFAULT NULL,
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_abuse_cases_open_target` (`open_target_id`),
    KEY `idx_abuse_cases_target` (`target_id`),
    KEY `idx_abuse_cases_status_reported` (`status`, `last_reported_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS abuse_reports (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    `case_id` bigint unsigned NOT NULL,
    -- Who reported it. Shown to administrators only, never to whoever was
    -- reported.
    `reporter_id` bigint unsigned NOT NULL,
    -- A key from the installation's reasons file. Not checked against it
    -- here: a reason removed from the file must not orphan the reports that
    -- used it.
    `reason_key` varchar(64) NOT NULL,
    `details` varchar(1000) NOT NULL DEFAULT '',
    `created_at` datetime NOT NULL,
    `updated_at` datetime NOT NULL,
    `created_ip` varchar(45) NOT NULL DEFAULT '',
    PRIMARY KEY (`id`),
    -- One report per person per case. Reporting again changes the reason and
    -- the details of the one they have.
    UNIQUE KEY `idx_abuse_reports_case_reporter` (`case_id`, `reporter_id`),
    KEY `idx_abuse_reports_reporter` (`reporter_id`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS abuse_reports;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS abuse_cases;
-- +goose StatementEnd
