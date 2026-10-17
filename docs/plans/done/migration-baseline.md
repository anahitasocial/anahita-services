# One clean schema: fold the migrations into a baseline

*Roadmap item 27, planned 2026-10-03. **Done 2026-10-17,** before row 29 at the user's word. Done last, before the first public release. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md).*

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

## Where the legacy path differs from a fresh schema today (found 2026-10-05)

Compared row by row for every table, columns and indexes. The two are the
same except for these, both from long before, and both for this item to put
right:

- **`nodes.service` allows NULL** on a database built by `legacy/upgrade.sql`
  and does not on a fresh one. The legacy script renames `component` to
  `service` without saying NOT NULL.
- **The columns of `nodes` are in a different order.** `website_url`,
  `person_type`, `person_pronouns`, `last_visited_at` and `unique_alias` are
  added at the end by the legacy script and sit in the middle of a fresh
  table. Nothing reads columns by position, so it changes nothing that runs.

Earlier comparisons reported the schemas as matching. They compared one long
string per table, which MySQL cuts off at 1,024 characters, so only the first
dozen columns were ever compared. The schema test this item adds has to
compare rows, not a joined string.

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

## What was built (2026-10-17)

1. **The PHP upgrade was broken, and is fixed.** Run on the user's copy of
   anahita.io's production database (2022-09-06), `legacy/upgrade.sql` stopped
   at `pinned_at`, which used `updated_at` before the PHP names were renamed
   to it; every PHP site's upgrade would have stopped there. Then the two
   differences: `nodes.uuid` and `nodes.uri` came out as utf8mb4, taken along
   by the table's conversion, and `nodes.service` allowed NULL.
2. **`make schema_test`** (`migration-job/check_schema.sh`, run by `make test`):
   `schema/init.sql` into one database, the PHP schema without rows
   (`legacy/fixtures/php-schema.sql`) through `legacy/upgrade.sql` into
   another, on a throwaway MySQL, compared column by column and index by
   index. It fails on the old file with the error the upgrade stopped at.
3. **NULL only where it means something** (asked for by the user the same
   day: "We want our databases to be pristine whether we create them from
   scratch or migrating an existing Anahita instance"). Every PHP-era table
   says what it holds; NULL is kept for references, an address elsewhere, a
   place and the dates of what may not have happened
   ([Architecture](../../architecture.md#null-only-where-it-means-something)).
   The data had three faults this showed: 784 nodes with 0 for no parent,
   which a query asking `IS NULL` missed; 3,129 at 0,0 for no place; 519
   that are not events with a start date, which every node was read as
   having, "now". The code lost every COALESCE it no longer needs, and the
   database tests found a repost written without `updated_at` and three
   statements writing NULL that now could not.
4. **The fold.** The 70 migrations are gone; `entrypoint.sh` records goose's
   baseline alone, and takes the folded versions' rows out of a database
   that ran them. Tried on a copy of dev and on an empty database.
5. **Comments that pointed to migration files** say why themselves.
   What anahita.io's own data needed went to `legacy/sites/anahita-io.sql`,
   run by hand, the start of the move to banu.io.

## Different from the plan

- **`init.sql` was tidied, not written again from `SHOW CREATE TABLE`:** it
  already described its columns, and matched the live schema but for two
  lines. Regenerating it would have lost that.
- **`legacy/upgrade.sql` was not regrouped** into conversion, repairs and
  cleanup. Its order is what makes it work, and the schema test now holds it
  to the result; regrouping 2,000 lines would have been risk without gain.
- **"Portrait" is not retired:** it is in 93 places in the services and 46
  in the web app, and in the API, which makes it a change of its own, not of
  the schema.
- **There is no staging database** (the user, 2026-10-17): only a fresh one
  and the user's copy of production through the upgrade.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-10-03 |
| Implementation | done 2026-10-17 |
| Left | retiring "portrait" (an API change); the banu.io move's own data changes, in `legacy/sites/anahita-io.sql` |
