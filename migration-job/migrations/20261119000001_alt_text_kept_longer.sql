-- An image's description from another server is kept up to 10,000
-- characters, Mastodon's own limit, rather than cut at 1,500 (the user,
-- 2026-10-17: our lengths consistent and fit for the fediverse; see
-- anahita-libs/constants/limits.go). One written here is up to 2,000.

-- +goose Up
ALTER TABLE photo_files MODIFY `alt_text` text NOT NULL DEFAULT ('');

-- +goose Down
ALTER TABLE photo_files MODIFY `alt_text` varchar(1500) NOT NULL DEFAULT '';
