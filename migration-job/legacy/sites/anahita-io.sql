-- What anahita.io's own data needs after legacy/upgrade.sql, on its way to
-- banu.io, and on no other site. Not run by entrypoint.sh: run it by hand,
-- once, after the upgrade:
--
--   mysql anahita < migration-job/legacy/sites/anahita-io.sql
--
-- The rest of the move (photos taken from the Anahita Atrium group to their
-- authors' profiles, and the like) is to be written here when it is planned.

-- Two avatars set by hand during development, moved here from the end of
-- legacy/upgrade.sql, where every PHP site would have run them.
UPDATE nodes SET avatar_filename = '8944e519-1d9f-46c4-a16f-3da487ade61f.jpg' WHERE alias = 'rastin';
UPDATE nodes SET avatar_filename = '423f925ce9bb8dce0c8f1876778a96a8.jpg' WHERE alias = 'rmundel';
