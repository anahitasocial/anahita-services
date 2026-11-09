-- +goose Up
-- A video or a sound from another network, copied here the first time
-- somebody plays it (anahita-web roadmap rows 120 and 137,
-- docs/plans/audio-and-video.md). Owned by video-service, whose worker
-- does the copying. No foreign keys, like the rest of the schema.
CREATE TABLE IF NOT EXISTS media_elsewhere (
    -- The post from another network whose video or sound this is.
    `node_id` bigint unsigned NOT NULL,
    -- What the post said of it when it was asked for: video or audio, and
    -- the https address of the file on its own server.
    `kind` varchar(8) NOT NULL,
    `source` varchar(2000) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    -- waiting, working, ready; failed, with its reason (too_large,
    -- too_long, not_a_video, gone, refused, failed), or removed once the
    -- copy is older than REMOTE_VIDEO_DAYS.
    `status` varchar(16) NOT NULL DEFAULT 'waiting',
    `reason` varchar(32) NOT NULL DEFAULT '',
    `attempts` int NOT NULL DEFAULT 0,
    `claimed_at` datetime DEFAULT NULL,
    -- When somebody last pressed play: the copy is removed so long after.
    `played_at` datetime NOT NULL,
    `created_at` datetime NOT NULL,
    `updated_at` datetime NOT NULL,
    PRIMARY KEY (`node_id`),
    KEY `idx_media_elsewhere_status` (`status`, `updated_at`),
    KEY `idx_media_elsewhere_played` (`status`, `played_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE IF EXISTS media_elsewhere;
