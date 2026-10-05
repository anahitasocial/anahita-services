# One clean schema: fold the migrations into a baseline

*Roadmap item 27, planned 2026-10-03. Done last, before the first public release. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md).*

## Context
`migration-job` has three ways to build a database, kept in step by hand:

- `schema/init.sql`, for an empty database;
- `legacy/upgrade.sql`, for a database from the PHP app;
- `migrations/*.sql` (goose), for a database that already went through one of the two.

Most of the goose migrations exist only because of the PHP schema: they repair data it left behind (edge types, private notes, photo sets) or drop columns it carried. A fresh install has nothing to repair, yet every one of them has to be mirrored into `legacy/upgrade.sql` and reflected in `init.sql`, and each must be guarded so it survives running against a schema that already has its change. That is three places to change for every schema edit, and the reason `check_sql_order.py` exists.

While nothing is live, this can be collapsed.

## Decision
Once the roadmap's items are done, and before the first public release:

- **`schema/init.sql` becomes the one clean starting point**, written as the schema should be, with no history in its comments.
- **`legacy/upgrade.sql` holds everything that concerns the PHP schema:** the conversion and every legacy data repair. It ends at exactly the schema `init.sql` creates.
- **`migrations/` starts empty.** The first migration written after the release is the first one any installation runs.

Until then every item keeps writing a goose migration, because that is how databases that were already upgraded (the development and staging ones) pick up each change.

## Implementation
1. **Bring every existing database fully up to date first:** run the migration job on dev and staging so each is at the last goose version. After the squash there is no path from an older version.
2. **Rewrite `schema/init.sql`** from the live schema (`SHOW CREATE TABLE` for each table), tidied: consistent types, indexes that queries use, comments that describe the column rather than its history.
3. **Tidy `legacy/upgrade.sql`:** group it into conversion, repairs and cleanup; remove steps that create something a later step drops; keep `check_sql_order.py` passing.
4. **Delete `migrations/*.sql`.** In `entrypoint.sh`, `stamp_migrations` then records only goose's baseline row. Existing databases still hold version rows for files that no longer exist: reset `goose_db_version` to the baseline row in the same step, and check on the dev database how goose behaves before and after, rather than assuming it ignores them.
5. **Add a schema test:** load `init.sql` into one database and the legacy dump through `upgrade.sql` into another, and compare `information_schema` for every table (columns, types, nullability, defaults, indexes). Today they differ in at least one place (`nodes.service` is nullable after an upgrade), and nothing checks.
6. **Document the rule** in `docs/getting-started.md`: from the first release on, schema changes are goose migrations, mirrored in `init.sql`, and migrations are never squashed again without a documented upgrade path.

## Names to put right at the same time (added 2026-10-05)

A rename costs the same three files and a pass through the graph service
whenever it is done, so these wait for this item, when the schema is being
rewritten anyway. The reasoning is in `docs/architecture.md`, Naming images.

- ~~`nodes.avatar_filename`~~ **Done on 2026-10-05**, ahead of this item at
  the user's request: migration `20261009000001_avatar_filename.sql`. An
  actor's avatar has its own column, and `filename` is the file a node is.
  When the migrations are folded, the column simply belongs in the clean
  `init.sql`, and the move of old values in `legacy/upgrade.sql`.
- **Retire "portrait".** In the code and in the API's `portrait_urls` it
  means a photo post's image. Photo posts carry their images in `files`;
  `portrait_urls` goes once the web app reads the first image from there.

## Estimate
About 4–6 h, 1–1½ sessions. Most of it is the schema comparison test and reading `upgrade.sql` end to end.

## Verification
- A fresh install from `init.sql` and an upgrade of the legacy dump produce identical schemas, by the test in step 5.
- The dev database, already at the last version, starts cleanly with the empty `migrations/` folder and the migration job reports nothing to run.
- Every module's tests pass against both schemas.

## Dependencies
- **After every other roadmap item** that changes the schema, and before the first live installation.

## Documentation

A phase is finished only when these pages match the code.

- **services `getting-started.md`**: how the database is created, and the rule for schema changes after the release.
- **services `architecture.md`**: the migration job's two paths.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-10-03 |
| Implementation | not started |
