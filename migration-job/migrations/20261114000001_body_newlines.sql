-- Every line break of what was written stored as \n (the user, 2026-10-16).
--
-- A browser sends a form's text with \r\n, and that is how it was kept, so
-- the same post was stored differently by what sent it, and a code block
-- carried \r into what a reader copied. The graph writes \n from now on
-- (graph-grpc-service repositories/newlines.go); this does the same to
-- what is there. Only the line breaks change. legacy/upgrade.sql does it
-- too, for a site upgraded from the PHP one.
--
-- Idempotent. Down does nothing: which breaks were \r\n is not kept, and
-- nothing reads the difference.

-- +goose Up
-- +goose StatementBegin
UPDATE nodes
SET body = REPLACE(REPLACE(body, CONCAT(CHAR(13), CHAR(10)), CHAR(10)), CHAR(13), CHAR(10))
WHERE body LIKE CONCAT('%', CHAR(13), '%');
-- +goose StatementEnd

-- +goose Down
SELECT 1;
