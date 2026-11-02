-- Whether a refresh token was revoked by being exchanged for a new pair: the
-- one revocation that spends it, so the only one whose token, presented
-- again, is a replay and revokes the person's other tokens for its client.
-- Without it, a token revoked by that cascade set off the next cascade when
-- its session came back, signing the person out on every device every
-- fifteen minutes (banu.io, 2026-11-02).
--
-- Rows revoked before it read as not rotated, and are refused quietly.

-- +goose Up
ALTER TABLE oauth_refresh_tokens
    ADD COLUMN `rotated` tinyint(1) NOT NULL DEFAULT 0 AFTER `revoked_at`;

-- +goose Down
ALTER TABLE oauth_refresh_tokens
    DROP COLUMN `rotated`;
