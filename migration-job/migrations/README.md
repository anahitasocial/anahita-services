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
agree. The folded migrations are in the repository's history, before the
commit that removed them.
