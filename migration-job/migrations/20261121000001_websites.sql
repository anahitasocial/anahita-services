-- Websites: websites' feeds, as profiles to follow (docs/plans/website.md).
--
--   websites        one for each website: its feed, the site, the domain
--                 proved for it, and how its feed is being read
--   website_items   every item of its feed seen, with the article it
--                 became, so that nothing is posted twice and a changed
--                 item changes its post
--
-- Belongs to syndication-service.

-- +goose Up
CREATE TABLE IF NOT EXISTS websites (
    -- The website's node: node.actor.actor-service.website.v1.
    node_id bigint unsigned NOT NULL,
    -- Where its feed is read, one website to a feed, and the site it is of.
    feed_url varchar(2000) NOT NULL,
    feed_hash char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    site_url varchar(2000) NOT NULL,
    -- The domain proved for it.
    domain_id bigint unsigned NOT NULL,
    -- active, or paused: its proof gone for a week, or its administrators
    -- stopped it. A paused website's feed is not read.
    state varchar(16) NOT NULL DEFAULT 'active',
    -- What the feed's server said the last time, sent back so that an
    -- unchanged feed is not sent again.
    etag varchar(255) DEFAULT NULL,
    last_modified varchar(64) DEFAULT NULL,
    -- The WebSub hub the feed names, and until when it sends to us. NULL
    -- while there is none.
    hub_url varchar(2000) DEFAULT NULL,
    hub_until datetime DEFAULT NULL,
    -- When the feed is next read, when it last was, and how often in a row
    -- it could not be: a feed that fails is asked less often.
    next_read_at datetime NOT NULL,
    read_at datetime DEFAULT NULL,
    failures int unsigned NOT NULL DEFAULT 0,
    last_error varchar(500) DEFAULT NULL,
    -- When a worker took it to read: see Claim.
    claimed_at datetime DEFAULT NULL,
    -- How many posts it made this hour, and when that hour began.
    hour_posts int unsigned NOT NULL DEFAULT 0,
    hour_began_at datetime DEFAULT NULL,
    created_at datetime NOT NULL,
    updated_at datetime NOT NULL,
    PRIMARY KEY (node_id),
    UNIQUE KEY uq_feed_hash (feed_hash),
    KEY state_next_read_at (state, next_read_at),
    KEY domain_id (domain_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS website_items (
    website_id bigint unsigned NOT NULL,
    -- The item's id, or its address when it has none, hashed.
    item_key char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    -- The article it became. NULL for an item marked seen and not posted:
    -- what the feed held when the website was made, or what waits for the
    -- next hour.
    node_id bigint unsigned DEFAULT NULL,
    -- A hash of its title and its excerpt, so that a changed item changes
    -- its post.
    content_hash char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    -- seen (never to be posted), waiting (to be posted, held back by the
    -- hourly cap) or posted.
    state varchar(16) NOT NULL,
    created_at datetime NOT NULL,
    updated_at datetime NOT NULL,
    PRIMARY KEY (website_id, item_key),
    KEY node_id (node_id),
    KEY website_state (website_id, state)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE IF EXISTS website_items;
DROP TABLE IF EXISTS websites;
