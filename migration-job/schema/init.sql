-- --------------------------------------------------------

CREATE TABLE nodes (
  -- Not SERIAL. SERIAL expands to BIGINT UNSIGNED AUTO_INCREMENT UNIQUE, so
  -- every row carried a redundant unique index on `id` alongside the primary
  -- key, and every insert maintained both.
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  -- What the node is called when another server is told of it: random, so
  -- it says nothing of how many there are, and never changed. The database
  -- gives it, so nothing that adds a node can forget to.
  `uuid` char(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT (LOWER(CONCAT(HEX(RANDOM_BYTES(4)),'-',HEX(RANDOM_BYTES(2)),'-4',SUBSTR(HEX(RANDOM_BYTES(2)),2),'-',HEX(CONV(SUBSTR(HEX(RANDOM_BYTES(1)),1,1),16,10) & 3 | 8),SUBSTR(HEX(RANDOM_BYTES(2)),2),'-',HEX(RANDOM_BYTES(6))))),
  -- What the node is called where it lives, when that is another server:
  -- the address of an ActivityPub object, or an at:// URI. NULL on
  -- everything made here, which is what "made here" means.
  `uri` varchar(512) CHARACTER SET ascii COLLATE ascii_bin DEFAULT NULL,
  -- The server it came from, in lower case.
  `origin_domain` varchar(255) DEFAULT NULL,
  -- 128 for the same reason as edges: an ASCII identifier from a fixed
  -- grammar, longest 42 characters. It also carries five composite indexes,
  -- so the width is paid for five times over.
  `type` varchar(128) NOT NULL,
  `service` varchar(255) NOT NULL,
  `name` varchar(255),
  `alias` varchar(255),
  `body` mediumtext,
  `starts_at` datetime DEFAULT NULL,
  `ends_at` datetime DEFAULT NULL,
  `owner_type` varchar(255) DEFAULT NULL,
  `owner_id` bigint(11) unsigned DEFAULT NULL,
  `comment_status` tinyint(1) DEFAULT NULL,
  -- Who may reply, as the post's author sets it. NULL when they have not
  -- said. See anahita-libs/constants/reply_access.go.
  `reply_access` varchar(64) DEFAULT NULL,
  -- Who may quote: on a post, that post; on a person, their posts when
  -- they say nothing. See anahita-libs/constants/quote_policy.go.
  `quote_policy` varchar(16) DEFAULT NULL,
  `comment_count` int(11) unsigned DEFAULT NULL,
  `repost_count` int(11) unsigned NOT NULL DEFAULT 0,
  `quote_count` int(11) unsigned NOT NULL DEFAULT 0,
  `enabled` tinyint(1) NOT NULL DEFAULT 0,
  `verified` tinyint(1) NOT NULL DEFAULT 0,
  `pinned` tinyint(1) NOT NULL DEFAULT 0,
  -- When it was pinned to the profile it is on. NULL when it is not.
  `pinned_at` datetime DEFAULT NULL,
  -- mediumtext, not text: CONVERT TO CHARACTER SET utf8mb4 promotes a text
  -- column to preserve its character capacity, so a database migrated by
  -- legacy/upgrade.sql ends up with mediumtext. Declared here so the two
  -- paths produce the same schema.
  `meta` mediumtext DEFAULT NULL,
  -- One link on an actor profile. Replaces the website/contact/phone trio that
  -- used to be read out of `meta`, none of which any Go request ever accepted.
  `website_url` varchar(255) DEFAULT NULL,
  `view_count` int(11) unsigned NOT NULL DEFAULT 0,
  `parent_id` bigint(11) unsigned DEFAULT NULL,
  `parent_type` varchar(255) DEFAULT NULL,
  -- The post at the top of the thread a reply belongs to. NULL on anything
  -- that is not a reply. Who may see a reply is decided by this post.
  `root_id` bigint(11) unsigned DEFAULT NULL,
  -- On a note that quotes a post: the post, and when that post's author
  -- took it out of the quote.
  `quote_id` bigint(11) unsigned DEFAULT NULL,
  `quote_detached_at` datetime DEFAULT NULL,
  `geo_latitude` float(10,6) DEFAULT NULL,
  `geo_longitude` float(10,6) DEFAULT NULL,
  `geo_address` VARCHAR(255) DEFAULT NULL,
  `geo_city` VARCHAR(50) DEFAULT NULL,
  `geo_state_province` VARCHAR(50) DEFAULT NULL,
  `geo_country` VARCHAR(30) DEFAULT NULL,
  `geo_postalcode` VARCHAR(15) DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `created_by` bigint(11) unsigned DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `modified_by` bigint(11) unsigned DEFAULT NULL,
  -- The file this node is: a photo post's photo. Never an actor's picture.
  `filename` varchar(255) DEFAULT NULL,
  -- The picture that stands for an actor. Only actors have one.
  `avatar_filename` varchar(255) DEFAULT NULL,
  `cover_filename` varchar(255) DEFAULT NULL,
  `follower_count` int(11) unsigned DEFAULT NULL,
  `leader_count` int(11) unsigned DEFAULT NULL,
  `allow_follow_request` tinyint(1) unsigned DEFAULT NULL,
  -- Whether what an actor reposts is also among its profile's posts.
  `show_reposts_on_profile` tinyint(1) NOT NULL DEFAULT 0,
  -- On a post that holds a file, whether whoever can see it may download
  -- it. On a person or a group, what their new posts start with.
  `allow_download` tinyint(1) NOT NULL DEFAULT 0,
  `subscriber_count` int(11) unsigned DEFAULT NULL,
  `vote_up_count` int(11) unsigned DEFAULT NULL,
  `excerpt` text,
  `mimetype` varchar(100) DEFAULT NULL,
  `language` varchar(50) DEFAULT NULL,
  `access` varchar(255) NOT NULL,
  -- The per-node-type optional feature set, as JSON. Named `features` because
  -- that is what the code selects; this file called it
  -- `enabled_optional_features`, which nothing has ever read.
  `features` mediumtext,
  -- When an actor was scheduled for deletion. NULL for everything that is not
  -- deleted, which is almost every row.
  --
  -- Deletion sets this and clears `enabled`; it does not remove anything. The
  -- account stops resolving at every login path immediately, and the sweeper
  -- purges the content after the grace period. Until then the person can sign
  -- in and restore, which the previous hard delete made impossible — there was
  -- no soft delete anywhere and no way back from a misclick.
  --
  -- Only actor nodes carry it. Content owned by a deleted actor stays as it is
  -- until the sweeper runs, so a restore brings the profile back whole.
  `deleted_at` datetime DEFAULT NULL,
  -- Set when the sweeper, or a super administrator, has reduced the actor to
  -- a tombstone. The purge queue is "deleted and not yet purged"; without this
  -- every tombstone would be read again on every run.
  `purged_at` datetime DEFAULT NULL,
  -- Permanent. The content stays readable at its URLs; what stops is being
  -- pushed at people who did not ask for it. Separate from `enabled` and
  -- `deleted_at` because each of the three maps to a different answer, and a
  -- single flag cannot give three.
  `archived_at` datetime DEFAULT NULL,
  -- Set when the installation features this actor, NULL otherwise. The
  -- accounts onboarding offers to follow. A timestamp rather than a flag so it
  -- is the order as well: most recently featured first.
  `featured_at` datetime DEFAULT NULL,
  -- Person identity. guest / registered / administrator / super-administrator.
  -- Named for the node rather than for auth: the graph owns the person, and
  -- unlike owner_type and parent_type this holds an authority level, not a node
  -- type string.
  `person_type` varchar(50) DEFAULT NULL,
  -- Free text, e.g. "she/her", "iel". Never an enum — an enum hard-codes
  -- English grammar and is meaningless in a language with no gendered
  -- third-person pronoun. Never used for logic; empty renders nothing.
  `person_pronouns` varchar(50) DEFAULT NULL,
  `last_visited_at` datetime DEFAULT NULL,
  -- The actor handle namespace.
  --
  -- alias alone cannot carry a unique index: article revisions share their
  -- article's slug, and a location, a photo-set and its photos share a place
  -- slug. MySQL has no partial indexes, so this is NULL for everything that is
  -- not an actor and a unique index permits any number of NULLs.
  --
  -- All actor types share one namespace because WebFinger resolves
  -- acct:name@domain to exactly one actor and has no type dimension — a person
  -- and a group with the same alias would make both unaddressable once this
  -- instance federates.
  --
  -- An event is an actor and is left out. It is not addressed by a handle:
  -- its address carries its id, and two events may well be called the same
  -- thing. Left in, a second "Birthday" could not be made, and an event
  -- named for a town would take that handle from the group that wants it.
  --
  -- The `actor:` prefix is redundant with one branch. It is there so a second
  -- namespace — hashtags, which are addressed by alias — is one CASE line
  -- rather than another column and another index.
  `unique_alias` varchar(300) GENERATED ALWAYS AS (
      CASE
          WHEN alias IS NULL OR alias = '' THEN NULL
          WHEN type = 'node.actor.actor-service.event.v1' THEN NULL
          WHEN type LIKE 'node.actor.%'    THEN CONCAT('actor:', alias)
          ELSE NULL
      END
  ) STORED,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_unique_alias` (`unique_alias`),
  UNIQUE KEY `uq_uuid` (`uuid`),
  UNIQUE KEY `uq_uri` (`uri`),
  KEY `person_type` (`person_type`),
  -- The sweeper's only query is "actors deleted before X", and it runs against
  -- a column that is NULL for practically every row.
  KEY `deleted_at` (`deleted_at`),
  KEY `purged_at` (`purged_at`),
  KEY `archived_at` (`archived_at`),
  KEY `featured_at` (`featured_at`),
  KEY `created_by` (`created_by`),
  KEY `modified_by` (`modified_by`),
  KEY `type` (`type`),
  KEY `service` (`service`),
  KEY `owner_id` (`owner_id`),
  KEY `owner_pinned` (`owner_id`,`pinned`),
  KEY `parent_id` (`parent_id`),
  KEY `root_id` (`root_id`),
  KEY `quote_id` (`quote_id`),
  KEY `starts_at` (`starts_at`),
  KEY `ends_at` (`ends_at`),
  KEY `type_enabled` (`type`,`enabled`),
  KEY `type_updated_at` (`type`,`updated_at`),
  KEY `type_created_at` (`type`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
-- --------------------------------------------------------

CREATE TABLE edges (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  -- 128, not 255. These are machine-generated ASCII identifiers from a fixed
  -- grammar — `edge.<family>.<name>.v1`, `node.<family>.<service>.<entity>.v1`
  -- — and the longest in the database is 42 characters.
  --
  -- The size is load-bearing, not cosmetic: uc_edge spans all three type
  -- columns, and at varchar(255) under utf8mb4 that key is 3*255*4 + 16 =
  -- 3076 bytes, four over InnoDB's 3072 limit. The table simply will not
  -- create. It fitted only while the schema was utf8 at three bytes a
  -- character.
  `type` varchar(128) NOT NULL,
  `node_a_id` bigint(11) unsigned NOT NULL,
  `node_a_type` varchar(128) NOT NULL,
  `node_b_id` bigint(11) unsigned NOT NULL,
  `node_b_type` varchar(128) NOT NULL,
  `meta` mediumtext,
  `created_at` datetime DEFAULT NULL,
  `created_by` bigint(11) unsigned DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `modified_by` bigint(11) unsigned DEFAULT NULL,
  -- Where an edge sits among its siblings, for the ones kept in an order
  -- somebody chose. Written with every edge, so it has to be here.
  `ordering` int(11) unsigned DEFAULT NULL,
  `starts_at` datetime DEFAULT NULL,
  `ends_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `type` (`type`),
  KEY `node_a_id` (`node_a_id`),
  KEY `node_b_id` (`node_b_id`),
  KEY `starts_at` (`starts_at`),
  KEY `ends_at` (`ends_at`),
  CONSTRAINT uc_edge UNIQUE(`type`,`node_a_id`,`node_a_type`,`node_b_id`,`node_b_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
-- --------------------------------------------------------

-- The auth half of a person: credentials, and what they agreed to.
--
-- Named auth_accounts rather than auth_credentials because it holds legal
-- consent as well as secrets, and "credentials" is the wrong label for a record
-- of what somebody agreed to. One table rather than two — it is 1:1 with the
-- node, so splitting would buy a join and nothing else.
--
-- The person half lives on `nodes`. Nothing here is identity: no name, no
-- alias, no person_type. email is here rather than on the node because
-- graphpb.Node is the universal payload returned to sixteen callers, and a
-- login identifier is one missing access check away from public.
CREATE TABLE auth_accounts (
    `node_id`               bigint unsigned NOT NULL,
    `email`                 varchar(255) DEFAULT NULL,
    `password`              varchar(255) DEFAULT NULL,
    `totp_secret`           varchar(255) DEFAULT '',
    `totp_enabled`          tinyint(1) NOT NULL DEFAULT 0,
    `totp_recovery_codes`   mediumtext DEFAULT NULL,
    `totp_last_verified_at` datetime DEFAULT NULL,

    -- When the address was confirmed. NULL means it has not been, and an
    -- unverified account cannot sign in.
    `email_verified_at`     datetime DEFAULT NULL,

    -- When the person went through onboarding — completed or skipped, either
    -- counts. NULL means they have not, and the web app sends a person with
    -- an incomplete profile there after the agreements. Set once: a profile
    -- that later becomes incomplete does not re-open it.
    `onboarded_at`          datetime DEFAULT NULL,

    -- Who invited this person, when an invitation is how they arrived —
    -- directly, or through an application a member vouched for. NULL for a
    -- self-serve signup. Kept here because the invitation token that also
    -- says so is consumed and swept, and onboarding shows the inviter first.
    `invited_by`            bigint unsigned DEFAULT NULL,

    -- Terms of Service acceptance. The version sits alongside the timestamp so
    -- a revision can re-prompt only those on an older one, and the
    -- ip/user_agent/os trio is what makes the record evidential rather than
    -- merely a boolean.
    --
    -- *_accepted_at defaults to NULL, not NOW(). A row that never accepted
    -- anything must not carry a timestamp, or (accepted=0, accepted_at=<a
    -- date>) is ambiguous and the audit trail cannot be read.
    `tos_accepted`            tinyint(1) NOT NULL DEFAULT 0,
    `tos_accepted_at`         datetime     DEFAULT NULL,
    `tos_accepted_version`    varchar(20)  DEFAULT NULL,
    `tos_accepted_ip`         varchar(45)  DEFAULT NULL,
    `tos_accepted_user_agent` varchar(255) DEFAULT NULL,
    `tos_accepted_os`         varchar(100) DEFAULT NULL,

    -- Privacy Policy acceptance, same shape.
    `pp_accepted`             tinyint(1) NOT NULL DEFAULT 0,
    `pp_accepted_at`          datetime     DEFAULT NULL,
    `pp_accepted_version`     varchar(20)  DEFAULT NULL,
    `pp_accepted_ip`          varchar(45)  DEFAULT NULL,
    `pp_accepted_user_agent`  varchar(255) DEFAULT NULL,
    `pp_accepted_os`          varchar(100) DEFAULT NULL,

    PRIMARY KEY (`node_id`),
    UNIQUE KEY `email` (`email`),
    -- Revising the terms means finding everyone still on an older version.
    KEY `tos_accepted_version` (`tos_accepted_version`),
    KEY `pp_accepted_version` (`pp_accepted_version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
-- --------------------------------------------------------

CREATE TABLE IF NOT EXISTS signup_requests (
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
    -- A request reaches the queue only once its address is confirmed, so
    -- administrators are not moderating junk that nobody can even receive mail
    -- at. `unverified` is where it waits until then.
    `status` varchar(20) NOT NULL DEFAULT 'unverified',
    `email_verified_at` datetime DEFAULT NULL,

    -- Who vouched, when a member's invitation put them here rather than a
    -- cold request. An administrator's invitation never reaches this
    -- table — it is already a decision.
    `invited_by` bigint unsigned DEFAULT NULL,

    `decided_by`    bigint unsigned DEFAULT NULL,
    `decided_at`    datetime        DEFAULT NULL,
    `decision_note` varchar(255)    DEFAULT NULL,

    -- Where the request came from. The same evidential trio the consent
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
-- --------------------------------------------------------

CREATE TABLE notifications (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    `type` varchar(255) NOT NULL,
    `service` varchar(255) NOT NULL,
    `recipient_id` bigint(11) unsigned DEFAULT NULL,
    `subject_id` bigint(11) unsigned DEFAULT NULL,
    `subject_type` varchar(255) DEFAULT NULL,
    `object_id` bigint(11) unsigned DEFAULT NULL,
    `object_type` varchar(255) DEFAULT NULL,
    `target_id` bigint(11) unsigned DEFAULT NULL,
    `target_type` varchar(255) DEFAULT NULL,
    `comment_id` int(11) unsigned DEFAULT NULL,
    `created_at` datetime DEFAULT NULL,
    `updated_at` datetime DEFAULT NULL,
    -- mediumtext, not text. An existing database reaches utf8mb4 through
    -- CONVERT TO CHARACTER SET, which promotes text to preserve its character
    -- capacity; a fresh one is created as utf8mb4 outright and would keep
    -- text. Declaring the wider type is what makes the two paths agree.
    `meta` mediumtext DEFAULT NULL,
    `is_read` tinyint(1) NOT NULL DEFAULT 0,
    PRIMARY KEY (`id`),
    KEY `recipient_id` (`recipient_id`),
    KEY `subject_id` (`subject_id`),
    KEY `subject_type` (`subject_type`),
    KEY `object_id` (`object_id`),
    KEY `object_type` (`object_type`),
    KEY `target_id` (`target_id`),
    KEY `target_type` (`target_type`),
    KEY `comment_id` (`comment_id`),
    KEY `created_at` (`created_at`),
    KEY `updated_at` (`updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
-- --------------------------------------------------------

CREATE TABLE auth_logs (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    `created_at` datetime DEFAULT NULL,
    `created_by` BIGINT UNSIGNED NOT NULL,
    `agent_device` varchar(50) DEFAULT 'bot',
    `agent_os` varchar(255) DEFAULT NULL,
    `agent_name` varchar(255) DEFAULT NULL,
    `ip` varchar(255) DEFAULT NULL,
    `city` varchar(255) DEFAULT NULL,
    `country` varchar(255) DEFAULT NULL,
    `oauth_refresh_token_id` bigint(11) unsigned NOT NULL,
    `client_id` varchar(255) NOT NULL,
    PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE INDEX idx_auth_logs_created_by ON auth_logs (created_by);
CREATE INDEX idx_auth_logs_created_at ON auth_logs (created_at);
CREATE INDEX idx_auth_logs_created_by_client_id_created_at ON auth_logs (created_by, client_id, created_at DESC);
CREATE INDEX idx_auth_logs_oauth_refresh_token_id ON auth_logs (oauth_refresh_token_id);

-- Deletion audit, for any node.
--
-- Deletion previously left no record at all: person-service published an event
-- nothing consumed, group-service published nothing, and the only trace was a
-- line in a container's stdout — gone with the next restart.
--
-- Keyed on node_id rather than on an actor, because nothing here is specific to
-- actors: a deleted photo, article or comment wants the same record for the
-- same reason. Actors are simply the first thing to write to it. `node_type`
-- carries the full type string, so one query answers "what happened to this
-- node" and another answers "what has been deleted in this service".
--
-- Deliberately OUTLIVES the node. After the sweeper runs, node_id points at a
-- tombstone rather than a profile — which is the point. An audit trail erased
-- along with its subject is not an audit trail. It holds no personal data
-- beyond the two ids and the alias.
CREATE TABLE node_deletion_logs (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    -- The node that was deleted.
    `node_id` bigint(11) unsigned NOT NULL,
    -- The alias at the time, kept because a tombstone can be scrubbed by a
    -- later erasure request and this is then the only record of what went.
    `node_alias` varchar(255) DEFAULT NULL,
    -- Full type string, e.g. node.actor.group-service.group.v1. Doubles as the
    -- service filter, so no separate column is needed for it.
    `node_type` varchar(255) DEFAULT NULL,
    -- Who asked. Equal to node_id for a self-service account deletion, and for
    -- the sweeper, which acts on a deadline rather than for a person.
    `deleted_by` bigint(11) unsigned NOT NULL,
    -- 'scheduled' when the grace period starts, 'purged' when the sweeper
    -- erases it, 'restored' when it is taken back. Up to three rows for one
    -- node is the normal, complete story. Kept a varchar rather than an ENUM so
    -- adding an action is a code change, not an ALTER on an audit table.
    `action` varchar(20) NOT NULL,
    `created_at` datetime NOT NULL,
    PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE INDEX idx_node_deletion_logs_node_id ON node_deletion_logs (node_id);
CREATE INDEX idx_node_deletion_logs_created_at ON node_deletion_logs (created_at);

-- What is left to do for a purge in progress.
--
-- A purge removes an account's content in batches, each in its own short
-- transaction, so a large account cannot run out of time and an interrupted
-- purge carries on from what remains. Rows are removed as the work is done: an
-- actor with none left is finished.
--
-- Owned by graph-grpc-service.
CREATE TABLE IF NOT EXISTS purge_work (
    -- The actor being purged.
    `actor_id` bigint unsigned NOT NULL,
    -- 'delete' for a node to remove, or the counter to count again on a node
    -- that stays: 'count_follows', 'count_votes', 'count_subscribers',
    -- 'count_comments', 'count_reposts'.
    `task` varchar(24) NOT NULL,
    -- The node the task applies to.
    `node_id` bigint unsigned NOT NULL,
    PRIMARY KEY (`actor_id`, `task`, `node_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- The queue of accounts a super administrator has asked to be removed now.
--
-- Asking does not purge. It hides the account at once and adds a row here; the
-- purge itself is done by whoever holds the sweeper's lock, one account at a
-- time, so a request for five hundred accounts returns immediately and cannot
-- time out. The rows are also the progress of the request, and afterwards the
-- record of who asked for what.
--
-- Owned by graph-grpc-service.
CREATE TABLE IF NOT EXISTS purge_requests (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    -- The request that asked for it. One request names many actors, and its
    -- progress is the state of these rows.
    `batch_id` char(36) NOT NULL,
    `actor_id` bigint unsigned NOT NULL,
    -- Kept here so the progress view can still name an account after the
    -- purge has taken its name.
    `actor_alias` varchar(255) NOT NULL DEFAULT '',
    `actor_type` varchar(255) NOT NULL DEFAULT '',
    -- 'queued', 'done' or 'failed'.
    `status` varchar(16) NOT NULL DEFAULT 'queued',
    -- How many times the purge has been tried. It is given up as 'failed'
    -- after a few, with the last error kept.
    `attempts` int unsigned NOT NULL DEFAULT 0,
    `error` varchar(255) NOT NULL DEFAULT '',
    -- The super administrator who asked.
    `requested_by` bigint unsigned NOT NULL,
    `requested_at` datetime NOT NULL,
    `finished_at` datetime DEFAULT NULL,
    PRIMARY KEY (`id`),
    -- An actor is purged once. A failed request is replaced by asking again.
    UNIQUE KEY `idx_purge_requests_actor` (`actor_id`),
    KEY `idx_purge_requests_batch` (`batch_id`),
    KEY `idx_purge_requests_status` (`status`, `requested_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- A record of the things a super administrator does to the whole
-- installation at once, which no single profile's history would show.
--
-- The first is converting everything public to registered. It changes
-- thousands of rows in one step and cannot be undone, so who did it and what
-- it came to is written down here, once per action.
--
-- Owned by graph-grpc-service.
CREATE TABLE IF NOT EXISTS site_actions (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    -- What was done, as a short fixed word: 'made-public-registered'.
    `action` varchar(64) NOT NULL,
    -- The super administrator who did it.
    `performed_by` bigint unsigned NOT NULL,
    -- What it came to, as JSON: how many rows of each kind it changed.
    `detail` text,
    `created_at` datetime NOT NULL,
    PRIMARY KEY (`id`),
    KEY `idx_site_actions_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- The images of a photo post, in order. A post holds up to four.
--
-- One post with several images, not an album of posts: one caption, one set
-- of likes and replies, one audience. It is also how the fediverse sends
-- them, as one post with attachments.
--
-- nodes.filename and nodes.mimetype stay, as a copy of the first image, so
-- everything that shows one image for a photo (feeds, search, notifications)
-- goes on working without knowing about this table.
--
-- photo_uploads holds images that have been uploaded and are not in a post
-- yet. Each image is uploaded in a request of its own, which keeps every
-- request small, and the post is then made from them. An upload nobody uses
-- within a day is removed, with its stored images, by the sweeper.
--
-- Both owned by graph-grpc-service. No foreign keys, like the rest of this
-- schema.
CREATE TABLE IF NOT EXISTS photo_files (
    -- The photo post.
    `node_id` bigint unsigned NOT NULL,
    -- Where it comes in the post, from 0. The first is the cover.
    `position` tinyint unsigned NOT NULL,
    -- The stored name, as nodes.filename holds it for a single image. Each
    -- size is stored under this name with the size added.
    `filename` varchar(255) NOT NULL,
    `mimetype` varchar(100) NOT NULL DEFAULT '',
    -- Of the largest copy kept, turned the right way up. 0 for an image
    -- stored before this table existed.
    `width` int unsigned NOT NULL DEFAULT 0,
    `height` int unsigned NOT NULL DEFAULT 0,
    -- What the image shows, for people who cannot see it.
    `alt_text` varchar(1500) NOT NULL DEFAULT '',
    PRIMARY KEY (`node_id`, `position`),
    UNIQUE KEY `uq_photo_files_filename` (`filename`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS photo_uploads (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    -- The profile it was uploaded for. Part of where the image is stored.
    `owner_id` bigint unsigned NOT NULL,
    -- Who uploaded it. Nobody else can put it in a post.
    `created_by` bigint unsigned NOT NULL,
    `filename` varchar(255) NOT NULL,
    `mimetype` varchar(100) NOT NULL DEFAULT '',
    `width` int unsigned NOT NULL DEFAULT 0,
    `height` int unsigned NOT NULL DEFAULT 0,
    `created_at` datetime NOT NULL,
    PRIMARY KEY (`id`),
    KEY `idx_photo_uploads_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Events: an actor people say they are going to.
--
-- An event is a node like a group: followed, posted on, administered. What
-- only an event has is kept here, one row an event: its time zone, how many
-- may go, where to join online, and whether it was called off. Saying you are
-- going is following it, with the answer in the follow edge's meta.
--
-- Owned by graph-grpc-service. No foreign keys, like the rest of this schema.
CREATE TABLE IF NOT EXISTS events (
    -- The event, which is an actor: node.actor.actor-service.event.v1. Its
    -- name, description, access and when it starts and ends are on the node.
    `node_id` bigint unsigned NOT NULL,
    -- The zone the times were given in, as IANA names it (America/Toronto),
    -- so they read correctly on either side of a clock change.
    `timezone_name` varchar(64) NOT NULL DEFAULT '',
    -- How many may be going. 0 for no limit.
    `capacity` int unsigned NOT NULL DEFAULT 0,
    -- Where to join when it is online. Sent only to whoever is going, and to
    -- whoever looks after the event.
    `online_url` varchar(512) NOT NULL DEFAULT '',
    -- Whether everybody who follows the group hosting it may see that it is
    -- on and answer it, without being invited one by one. Only means
    -- something on an event that has a host and is not open to everybody
    -- already.
    `open_to_host_followers` tinyint(1) NOT NULL DEFAULT 0,
    -- Set when it is called off. It is kept, so people can see that it was.
    `cancelled_at` datetime DEFAULT NULL,
    -- Set when whoever is going has been reminded, so it is done once.
    `reminded_at` datetime DEFAULT NULL,
    PRIMARY KEY (`node_id`),
    KEY `idx_events_reminded_at` (`reminded_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Add indices

CREATE INDEX idx_edges_node_a_id ON edges (node_a_id);
CREATE INDEX idx_edges_node_b_id ON edges (node_b_id);
CREATE INDEX idx_edges_type     ON edges (type);

-- Composite indices for frequent relationship lookups
CREATE INDEX idx_edges_aid_type_bid ON edges (node_a_id, type, node_b_id);
CREATE INDEX idx_edges_bid_type_aid ON edges (node_b_id, type, node_a_id);

CREATE INDEX idx_nodes_id        ON nodes (id);
CREATE INDEX idx_nodes_owner_id  ON nodes (owner_id);
CREATE INDEX idx_nodes_type      ON nodes (type);
CREATE INDEX idx_nodes_access    ON nodes (access);
CREATE INDEX idx_nodes_created_at ON nodes (created_at);

CREATE INDEX idx_nodes_parent_id ON nodes (parent_id);

CREATE INDEX idx_edges_lookup ON edges (node_a_id, node_b_id, type);
CREATE INDEX idx_nodes_feed   ON nodes (owner_id, type, created_at);
CREATE INDEX origin_domain    ON nodes (origin_domain);

-- Fast lookups by type + direction
CREATE INDEX idx_edges_type_nodea_nodeb  ON edges (type, node_a_id, node_b_id);
CREATE INDEX idx_edges_type_nodeb_nodea  ON edges (type, node_b_id, node_a_id);

-- --------------------------------------------------------
-- OAuth tables
-- --------------------------------------------------------

CREATE TABLE oauth_authorizations (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    `code_hash` varchar(255) NOT NULL,
    `client_id` varchar(255) NOT NULL,
    `user_id` bigint(11) unsigned NOT NULL,
    `redirect_uri` text NOT NULL,
    `scopes` text DEFAULT NULL,
    `code_challenge` varchar(128) DEFAULT NULL,
    `code_challenge_method` varchar(10) DEFAULT NULL,
    `nonce` varchar(255) DEFAULT NULL,
    `expires_at` datetime NOT NULL,
    -- Defaulted, not supplied by the insert. Under MySQL strict mode a
    -- NOT NULL column with no default and no value is error 1364, not a
    -- silent zero — every one of these rows is written without naming
    -- created_at.
    `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_code_hash` (`code_hash`),
    KEY `idx_client_id` (`client_id`),
    KEY `idx_user_id` (`user_id`),
    KEY `idx_expires_at` (`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- --------------------------------------------------------

CREATE TABLE oauth_clients (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    `client_id` varchar(255) NOT NULL,
    `secret_hash` varchar(255) NOT NULL,
    `name` varchar(128) NOT NULL,
    `redirect_uris` JSON NOT NULL,
    `grant_types` JSON NOT NULL,
    `scopes` JSON NOT NULL,
    `token_expiry` int(11) NOT NULL DEFAULT 900,
    `confidential` tinyint(1) NOT NULL DEFAULT 1,
    `skip_consent` tinyint(1) NOT NULL DEFAULT 0,
    -- First-party clients are the apps we ship ourselves. Separate from
    -- skip_consent: a first-party client may still need a consent
    -- screen, and a third-party client must never be treated as ours
    -- regardless of how consent is configured.
    `first_party` tinyint(1) NOT NULL DEFAULT 0,
    `active` tinyint(1) NOT NULL DEFAULT 1,
    -- Defaulted because seeded first-party clients have no creating
    -- person — SeedDefaultClients runs at startup, before any request.
    `created_by` bigint(11) unsigned NOT NULL DEFAULT 0,
    -- created_at/updated_at rather than create_time/update_time: the
    -- repository selects created_at, and every other oauth_* table here
    -- already uses the _at suffix. Both defaulted — neither Add nor
    -- Edit supplies them, and under MySQL strict mode a NOT NULL column
    -- with no default is error 1364 rather than a silent zero.
    `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_client_id` (`client_id`),
    KEY `idx_active` (`active`),
    KEY `idx_created_by` (`created_by`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- --------------------------------------------------------

CREATE TABLE oauth_refresh_tokens (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    `token_hash` varchar(255) NOT NULL,
    `user_id` bigint(11) unsigned NOT NULL,
    `client_id` varchar(255) NOT NULL,
    `scopes` text DEFAULT NULL,
    `revoked` tinyint(1) NOT NULL DEFAULT 0,
    -- Records WHEN the token was revoked, which reuse detection needs in
    -- order to tell a rotation race from a replay: revoked moments ago is
    -- a client refreshing concurrently with itself, revoked long ago is an
    -- attacker replaying a stolen token, and the two want opposite
    -- responses. See ReuseGracePeriod in auth-service.
    --
    -- Nullable on purpose: a revoked row carrying no timestamp reads as
    -- "revoked long ago" and still trips the containment cascade, which is
    -- the safe direction to be wrong in.
    `revoked_at` datetime DEFAULT NULL,
    `expires_at` datetime NOT NULL,
    -- Defaulted: the insert does not name it. See oauth_authorizations.
    `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    -- No index on revoked_at: reuse detection finds the row by token_hash,
    -- which is already UNIQUE, and reads revoked/revoked_at off it.
    UNIQUE KEY `idx_token_hash` (`token_hash`),
    KEY `idx_user_id` (`user_id`),
    KEY `idx_client_id` (`client_id`),
    KEY `idx_revoked_expires` (`revoked`, `expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- --------------------------------------------------------

CREATE TABLE oauth_signing_keys (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    `key_id` varchar(255) NOT NULL,
    `algorithm` varchar(10) NOT NULL DEFAULT 'RS256',
    `public_key` text NOT NULL,
    `private_key` text DEFAULT NULL,
    `active` tinyint(1) NOT NULL DEFAULT 1,
    `expires_at` datetime NOT NULL,
    -- Defaulted: neither Add nor the KMS seeder names it. See
    -- oauth_authorizations.
    `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `backend` varchar(255) NOT NULL DEFAULT 'local',
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_key_id` (`key_id`),
    KEY `idx_active` (`active`),
    KEY `idx_active_expires` (`active`, `expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- --------------------------------------------------------

-- Single-use tokens emailed to a person: password resets and invites.
--
-- created_by / updated_by are node ids (nodes.id), the same soft
-- reference auth_logs.created_by uses. No foreign key, matching the
-- rest of this schema.
CREATE TABLE tokens (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    -- Where the token is SENT. For an emailchange token that is the
    -- person's current address, not the one they are moving to: the
    -- current inbox is what has to approve the move, so a stolen session
    -- alone cannot repoint the account.
    `recipient_email` varchar(255) NOT NULL,
    -- The requested address, on emailchange tokens only; NULL on every
    -- other type. Separate from recipient_email precisely so the two
    -- cannot be confused — swapping them turns the confirmation flow
    -- from a control into a way to hand the account away.
    `new_email` varchar(255) DEFAULT NULL,
    -- 'passwordreset' or 'invite' (constants.TokenType*). A varchar
    -- rather than an ENUM so adding a token type is a code change, not
    -- an ALTER TABLE on a table that gets written on every reset.
    `type` varchar(50) NOT NULL,
    -- The token itself: 64 random bytes, hex-encoded. Indexed because
    -- ReadByBody is the lookup every reset/invite link performs.
    `body` varchar(255) NOT NULL,
    -- One-time codes only (constants.TokenTypeLoginOTP and its siblings); unused
    -- by every other type. code_hash is an HMAC of the six digits, never the
    -- digits: a plain hash of a six-digit value can be reversed by trying all
    -- million. code_attempts counts wrong submissions, in the database so that
    -- guesses arriving together cannot share one. channel is how the code was
    -- sent: 'email', and later 'sms'.
    `code_hash` varchar(64) DEFAULT NULL,
    `code_attempts` int(11) NOT NULL DEFAULT 0,
    `channel` varchar(16) DEFAULT NULL,
    -- Defaulted: Token.NewToken sets StartTime in Go but tokens.Add
    -- does not include it in the insert column list.
    `started_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    -- Lifetime in seconds from started_at. Expiry is computed in Go
    -- (Token.SetStatus) rather than stored, so changing the default
    -- does not require rewriting existing rows.
    `expired_after` int(11) NOT NULL DEFAULT 0,
    `is_used` tinyint(1) NOT NULL DEFAULT 0,
    `created_at` datetime NOT NULL,
    `created_by` bigint(11) unsigned NOT NULL,
    `updated_at` datetime NOT NULL,
    `updated_by` bigint(11) unsigned NOT NULL,
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_tokens_body` (`body`),
    KEY `idx_tokens_type` (`type`),
    KEY `idx_tokens_recipient_email` (`recipient_email`),
    KEY `idx_tokens_created_by` (`created_by`),
    KEY `idx_tokens_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- --------------------------------------------------------

-- WebAuthn (passkey) credentials. One row per authenticator a person
-- has enrolled — laptop Touch ID, phone, hardware key.
--
-- user_id is a node id (nodes.id), a soft reference like the rest of
-- this schema. Nothing cleans these rows up automatically, so person
-- deletion must call DeleteByPersonID explicitly: a recycled node id
-- would otherwise inherit a stranger's passkey.
CREATE TABLE webauthn_credentials (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    `user_id` bigint(11) unsigned NOT NULL,
    -- Raw authenticator-generated identifier, stored as bytes rather
    -- than base64url so the login lookup does not depend on encoding
    -- choices matching across library versions. VARBINARY(255) is the
    -- longest a credential ID can be per the WebAuthn spec, and unlike
    -- BLOB it can carry a UNIQUE index directly.
    `credential_id` varbinary(255) NOT NULL,
    -- COSE-encoded public key. The private key never leaves the device.
    `public_key` blob NOT NULL,
    `attestation_type` varchar(255) NOT NULL DEFAULT '',
    -- Authenticator model identifier. Not used for any security
    -- decision — it drives the name/icon shown in the passkey list.
    `aaguid` varbinary(16) DEFAULT NULL,
    -- Monotonic counter for clone detection. Synced platform
    -- authenticators (iCloud Keychain, Google Password Manager) always
    -- report 0, which is expected and not a fault.
    `sign_count` bigint(20) unsigned NOT NULL DEFAULT 0,
    `clone_warning` tinyint(1) NOT NULL DEFAULT 0,
    -- Transport hints ("internal", "hybrid", "usb", "nfc", "ble").
    -- JSON because MySQL has no array type; the Postgres original used
    -- TEXT[]. Read and written through entities.StringSlice.
    `transports` JSON NOT NULL,
    -- BE / BS flags: whether this passkey can sync to a cloud keychain,
    -- and whether it currently is. Together they tell the person
    -- whether the passkey survives losing the device.
    `backup_eligible` tinyint(1) NOT NULL DEFAULT 0,
    `backup_state` tinyint(1) NOT NULL DEFAULT 0,
    -- Person-supplied label. Required: three unlabelled passkeys is not
    -- a list anyone can safely revoke from.
    `nickname` varchar(255) NOT NULL,
    `last_used_at` datetime DEFAULT NULL,
    `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    -- A credential ID identifies exactly one enrolment, globally. This
    -- is the login lookup.
    UNIQUE KEY `idx_webauthn_credentials_credential_id` (`credential_id`),
    KEY `idx_webauthn_credentials_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- --------------------------------------------------------

-- Abuse reports: what people have reported, and what was decided about it.
--
-- Two tables. A CASE is one reported node and its decision; a REPORT is one
-- person's complaint inside a case. A post that twenty people report is one
-- case for an administrator to handle, not twenty.
--
-- Owned by abuse-report-service. No foreign keys, like the rest of this
-- schema: a case must outlive the node it is about, which is the point of
-- keeping the target's type and owner on the row.

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

-- What somebody has chosen about being suggested profiles to follow, and
-- about being suggested to others. One row a person, and only once they have
-- changed something: no row is every choice at its default, which is yes.
--
-- Owned by recommendation-service. No foreign keys, like the rest of this
-- schema.
CREATE TABLE IF NOT EXISTS recommendation_preferences (
    `person_id` bigint unsigned NOT NULL,
    -- Whether to be suggested people, and groups.
    `suggest_people` tinyint(1) NOT NULL DEFAULT 1,
    `suggest_groups` tinyint(1) NOT NULL DEFAULT 1,
    `suggest_events` tinyint(1) NOT NULL DEFAULT 1,
    -- Whether what they liked and wrote may be used to choose for them.
    `use_activity` tinyint(1) NOT NULL DEFAULT 1,
    -- Whether they may be suggested to other people.
    `discoverable` tinyint(1) NOT NULL DEFAULT 1,
    -- How much each reason counts for when choosing for them, as JSON by the
    -- reason's name, where they have set it. Empty for the service's own.
    `weights` varchar(1000) NOT NULL DEFAULT '',
    `updated_at` datetime NOT NULL,
    PRIMARY KEY (`person_id`),
    KEY `idx_recommendation_preferences_discoverable` (`discoverable`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- A suggestion somebody said they were not interested in: that profile is
-- not suggested to them again. Theirs alone to see, and to take back.
CREATE TABLE IF NOT EXISTS recommendation_dismissals (
    `person_id` bigint unsigned NOT NULL,
    `actor_id` bigint unsigned NOT NULL,
    `created_at` datetime NOT NULL,
    PRIMARY KEY (`person_id`, `actor_id`),
    KEY `idx_recommendation_dismissals_actor` (`actor_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- A video on its way to being a post: sent by a browser straight to
-- storage, then made into the files that are shown. The row is the upload,
-- the work waiting to be done on it, and afterwards the record of how it
-- went; the post itself is a node, made only once there is something to
-- show.
--
-- Owned by video-service and its worker. No foreign keys, like the rest of
-- this schema.
CREATE TABLE IF NOT EXISTS video_uploads (
    -- Public, and unguessable: it is in the addresses the uploader asks.
    `id` char(36) NOT NULL,
    -- Who is posting it, and the profile it is posted on.
    `person_id` bigint unsigned NOT NULL,
    `owner_id` bigint unsigned NOT NULL,
    -- The post as it will be made: said when the upload began, and settled
    -- then, so that nothing has to be asked again when the video is ready.
    `name` varchar(255) NOT NULL DEFAULT '',
    `body` text,
    `access` varchar(32) NOT NULL DEFAULT '',
    `language` varchar(50) NOT NULL DEFAULT '',
    `reply_access` varchar(64) NOT NULL DEFAULT '',
    `quote_policy` varchar(16) NOT NULL DEFAULT '',
    -- Whether the post may be downloaded, as chosen when the upload began.
    `allow_download` tinyint(1) NOT NULL DEFAULT 0,
    -- 'uploading' while the browser sends it, 'processing' once it is all
    -- there and waiting for or with a worker, then 'ready', 'failed', or
    -- 'cancelled' for one given up on.
    `status` varchar(16) NOT NULL DEFAULT 'uploading',
    -- Where the file sent is in storage, and the store's own name for the
    -- upload while it is in parts.
    `source_key` varchar(255) NOT NULL,
    `storage_upload_id` varchar(255) NOT NULL DEFAULT '',
    -- What the uploader said it was, and what arrived.
    `declared_bytes` bigint unsigned NOT NULL DEFAULT 0,
    `size_bytes` bigint unsigned NOT NULL DEFAULT 0,
    -- The post, once there is one.
    `node_id` bigint unsigned DEFAULT NULL,
    -- Why it failed, in a word the uploader is shown the meaning of.
    `error` varchar(64) NOT NULL DEFAULT '',
    -- How many times a worker has taken it, and when one last did. A worker
    -- that dies leaves it taken; it is taken again after a while.
    `attempts` int NOT NULL DEFAULT 0,
    `claimed_at` datetime DEFAULT NULL,
    `created_at` datetime NOT NULL,
    `updated_at` datetime NOT NULL,
    PRIMARY KEY (`id`),
    KEY `idx_video_uploads_status` (`status`, `updated_at`),
    KEY `idx_video_uploads_person` (`person_id`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- A person's copy of their own data: one archive of what they wrote and
-- who they follow, made when they ask and kept a week for them to fetch.
-- The row is the request, the work to be done on it, and the record of it.
--
-- Owned by export-service and its worker. No foreign keys, like the rest of
-- this schema.
CREATE TABLE IF NOT EXISTS data_exports (
    -- Public, and unguessable: it is in the addresses its owner asks.
    `id` char(36) NOT NULL,
    `person_id` bigint unsigned NOT NULL,
    -- 'pending' until a worker takes it, 'building' while one has it, then
    -- 'ready', 'failed', 'cancelled' for one given up, or 'expired' once
    -- its archive has been deleted.
    `status` varchar(16) NOT NULL DEFAULT 'pending',
    -- The person, while the export is being made, and NULL afterwards.
    -- Unique, which is what makes "one at a time for each person" a rule of
    -- the database and not of whichever request got there first. Any number
    -- of finished exports store NULL and share the index.
    `active_person_id` bigint unsigned GENERATED ALWAYS AS (IF(`status` IN ('pending', 'building'), `person_id`, NULL)) STORED,
    -- Where the archive is in storage, and how big it is.
    `package_key` varchar(255) NOT NULL DEFAULT '',
    `size_bytes` bigint unsigned NOT NULL DEFAULT 0,
    -- Why it failed, in a word.
    `error` varchar(64) NOT NULL DEFAULT '',
    -- How many times a worker has taken it, and when one last did.
    `attempts` int NOT NULL DEFAULT 0,
    `claimed_at` datetime DEFAULT NULL,
    -- Where it was asked for from, kept as the record of the request.
    `ip` varchar(64) NOT NULL DEFAULT '',
    `user_agent` varchar(255) NOT NULL DEFAULT '',
    `created_at` datetime NOT NULL,
    `updated_at` datetime NOT NULL,
    `ready_at` datetime DEFAULT NULL,
    -- When the archive is deleted.
    `expires_at` datetime DEFAULT NULL,
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_data_exports_active_person` (`active_person_id`),
    KEY `idx_data_exports_person` (`person_id`, `created_at`),
    KEY `idx_data_exports_status` (`status`, `updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- ActivityPub: who has chosen to be on the fediverse, and the key each signs
-- with. See migrations/20261027000001_activitypub.sql.
CREATE TABLE IF NOT EXISTS activitypub_accounts (
    `person_id` bigint unsigned NOT NULL,
    `enabled` tinyint(1) NOT NULL DEFAULT 0,
    -- When they last turned it on.
    `enabled_at` datetime DEFAULT NULL,
    -- Their own updated_at when their followers' servers were last sent
    -- their description. See migrations/20261102000001_activitypub_told.sql.
    `told_updated_at` datetime DEFAULT NULL,
    `created_at` datetime NOT NULL,
    `updated_at` datetime NOT NULL,
    PRIMARY KEY (`person_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS actor_keys (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    `node_id` bigint unsigned NOT NULL,
    `algorithm` varchar(16) NOT NULL,
    -- PEM.
    `public_key` text NOT NULL,
    `private_key_encrypted` text NOT NULL,
    `created_at` datetime NOT NULL,
    `revoked_at` datetime DEFAULT NULL,
    `active_node_id` bigint unsigned GENERATED ALWAYS AS (IF(`revoked_at` IS NULL, `node_id`, NULL)) STORED,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uq_active_node_id` (`active_node_id`),
    KEY `node_id` (`node_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- What is known of the people on other servers. See
-- migrations/20261028000001_activitypub_actors.sql.
CREATE TABLE IF NOT EXISTS activitypub_actors (
    `node_id` bigint unsigned NOT NULL,
    -- Their address: the same as the node's uri.
    `uri` varchar(512) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    `inbox` varchar(512) NOT NULL,
    -- Where what is for anybody on their server is sent once. '' for none.
    `shared_inbox` varchar(512) NOT NULL DEFAULT '',
    -- The name of the key they sign with, and its public half, as PEM.
    `key_id` varchar(512) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    `public_key` text NOT NULL,
    -- Where their avatar is on their own server. '' for none.
    `icon_url` varchar(512) NOT NULL DEFAULT '',
    -- Where their cover is on their own server. '' for none.
    `header_url` varchar(512) NOT NULL DEFAULT '',
    -- Whether a follow of theirs waits to be approved.
    `locked` tinyint(1) NOT NULL DEFAULT 0,
    -- When they were last read from their server.
    `fetched_at` datetime NOT NULL,
    `created_at` datetime NOT NULL,
    PRIMARY KEY (`node_id`),
    UNIQUE KEY `uq_uri` (`uri`),
    KEY `key_id` (`key_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- What is waiting to be sent to other servers, and who follows whom
-- across them. See migrations/20261029000001_activitypub_follows.sql.
CREATE TABLE IF NOT EXISTS activitypub_deliveries (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    -- Said by whoever adds a row, to add it once however often they try:
    -- the activity and the inbox it is for. NULL for no such promise.
    dedupe_key varchar(191) CHARACTER SET ascii COLLATE ascii_bin DEFAULT NULL,
    -- Who here it is sent for.
    actor_id bigint unsigned NOT NULL,
    -- Where it goes.
    target varchar(512) NOT NULL,
    payload mediumblob NOT NULL,
    -- pending or failed. What was sent is removed. What a worker has taken
    -- is pending still, with next_attempt_at put off: see Claim.
    status varchar(16) NOT NULL DEFAULT 'pending',
    attempts int unsigned NOT NULL DEFAULT 0,
    next_attempt_at datetime NOT NULL,
    -- When a worker last took it.
    claimed_at datetime DEFAULT NULL,
    last_error varchar(500) DEFAULT NULL,
    created_at datetime NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_dedupe_key (dedupe_key),
    KEY status_next_attempt_at (status, next_attempt_at),
    KEY actor_id (actor_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS activitypub_follows (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT,
    -- The person here, and the person elsewhere.
    `local_id` bigint unsigned NOT NULL,
    `remote_id` bigint unsigned NOT NULL,
    -- 'in': the person elsewhere follows the one here. 'out': the other way.
    `direction` varchar(3) NOT NULL,
    -- What the follow is called between servers: the address of the Follow
    -- that asked for it.
    `activity_id` varchar(512) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    -- 'pending' until it is accepted, then 'accepted'.
    `state` varchar(16) NOT NULL DEFAULT 'pending',
    `created_at` datetime NOT NULL,
    `updated_at` datetime NOT NULL,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uq_pair` (`local_id`, `remote_id`, `direction`),
    KEY `remote_id` (`remote_id`),
    KEY `activity_id` (`activity_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Which posts made here other servers have been told of. See
-- migrations/20261030000001_activitypub_objects.sql.
CREATE TABLE IF NOT EXISTS activitypub_objects (
    `node_id` bigint unsigned NOT NULL,
    -- The post's UUID: what its address between servers is made of.
    `uuid` char(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    -- Whose it is: the person here who wrote it.
    `actor_id` bigint unsigned NOT NULL,
    -- For a reply: the inbox of whoever it answers, when they are on
    -- another server. See migrations/20261103000001_activitypub_object_reply.sql.
    `reply_inbox` varchar(512) NOT NULL DEFAULT '',
    -- The post's own updated_at when other servers were last sent it.
    `told_updated_at` datetime DEFAULT NULL,
    `created_at` datetime NOT NULL,
    PRIMARY KEY (`node_id`),
    KEY `actor_id` (`actor_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- The servers this installation will have nothing to do with. See
-- migrations/20261031000001_federation_domain_blocks.sql.
CREATE TABLE IF NOT EXISTS federation_domain_blocks (
    `domain` varchar(255) NOT NULL,
    -- Why, for the administrators who come after. Never shown to anybody
    -- else, and never sent to the server refused.
    `reason` varchar(500) NOT NULL DEFAULT '',
    `created_by` bigint unsigned NOT NULL,
    `created_at` datetime NOT NULL,
    PRIMARY KEY (`domain`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;





