# Migrations

Goose migrations for databases already running: each a change to the schema
made after the migrations were folded into `schema/init.sql` and
`legacy/upgrade.sql` on 2026-10-17 (roadmap row 27).

A schema change is three things, in one commit:

1. a migration here, named `<YYYYMMDDHHMMSS>_<what>.sql`, with `-- +goose Up`
   and `-- +goose Down`;
2. the same change in `schema/init.sql`, as the table is declared;
3. the same change in `legacy/upgrade.sql`, for a site coming from the PHP
   application.

Then `make schema_test` checks that the two ways of building a database still
agree.

A migration runs once: its version is kept in `goose_db_version`. On each
start the job takes out the versions of the folded migrations, numbered as
far as 20261122000001, but never one that has a file here, whatever its
number (`entrypoint.sh`). Until 2026-10-19 it took out every version in that
range, and the first migrations written after the fold, numbered by their
real dates, ran again on every start. A migration written since is numbered after
20261122000001 all the same, so that its number says it came after the
fold. The folded migrations are in the repository's history, before the
commit that removed them.
