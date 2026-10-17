-- Domains somebody has proved are theirs.
--
--   verified_domains   a domain being proved or proved, by a TXT record
--                      or a file holding a token (anahita-libs/domains),
--                      for the profile it is proved for. A website rests on
--                      one; people and groups are to use the same proof
--                      later. Checked again every day.
--
-- Belongs to website-service.

-- +goose Up
CREATE TABLE IF NOT EXISTS verified_domains (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    -- The domain proved (example.com), and the host its file is fetched
    -- from (www.example.com): the site's own address, without a leading
    -- "www." for the domain.
    domain varchar(253) CHARACTER SET ascii COLLATE ascii_general_ci NOT NULL,
    host varchar(253) CHARACTER SET ascii COLLATE ascii_general_ci NOT NULL,
    -- What is put in the TXT record at _anahita.<domain>, or in
    -- /.well-known/anahita-verification.txt on the host.
    token varchar(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    -- Who is proving it.
    person_id bigint unsigned NOT NULL,
    -- The profile it is proved for: a website, and later a person or a
    -- group. NULL until that profile is made.
    profile_id bigint unsigned DEFAULT NULL,
    -- dns or file: where the token was last found.
    method varchar(8) DEFAULT NULL,
    -- When it was first proved; NULL until it is.
    verified_at datetime DEFAULT NULL,
    -- When it was last looked at, and since when the token has been in
    -- neither place. NULL while the proof holds.
    checked_at datetime DEFAULT NULL,
    failing_since datetime DEFAULT NULL,
    created_at datetime NOT NULL,
    updated_at datetime NOT NULL,
    PRIMARY KEY (id),
    KEY domain (domain),
    KEY person_id (person_id),
    KEY profile_id (profile_id),
    KEY checked_at (checked_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE IF EXISTS verified_domains;
