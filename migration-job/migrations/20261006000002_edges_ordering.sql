-- Puts back edges.ordering on a database built from schema/init.sql.
--
-- The column was removed from init.sql by mistake on 2026-10-02, in the change
-- that dropped thirteen unused columns from `nodes`. nodes.ordering was one of
-- those and really was unused; edges.ordering is a different column, and every
-- edge is written with it. A database upgraded from an earlier one, or from
-- the PHP application, never lost it. One created fresh after that date has no
-- such column, and on it nobody can follow, like or subscribe: the INSERT
-- names a column that is not there.
--
-- Nothing caught it because the tests that run against a fresh schema set up
-- their edges with SQL of their own. There is now one that adds an edge the
-- way the service does.
--
-- GUARDED: it adds the column only where it is missing.

-- +goose Up
-- +goose StatementBegin
CREATE PROCEDURE anahita_add_edges_ordering()
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'edges'
          AND COLUMN_NAME = 'ordering'
    ) THEN
        ALTER TABLE edges ADD COLUMN `ordering` int(11) unsigned DEFAULT NULL AFTER `modified_by`;
    END IF;
END;
-- +goose StatementEnd
CALL anahita_add_edges_ordering();
DROP PROCEDURE anahita_add_edges_ordering;

-- +goose Down
-- Nothing. The column belongs in the table, and removing it is what this
-- migration undoes.
SELECT 1;
