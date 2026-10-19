# Working on anahita-services

Anahita's Go microservices: a social platform and framework for operators
leaving Meta and X, on ActivityPub and the AT Protocol. This file is for
anybody changing the code, a person or an agent. It holds only what the
code cannot tell you; everything else is in the docs.

- Website: <https://www.anahita.io>
- Founder & Core Architect: Rastin Mehr
- Maintained and developed by rmd Studio, <https://www.rmdstudio.com>
- Commercial inquiries: <https://www.rmdstudio.com/contact>

## Where to begin

| To learn | Read |
| --- | --- |
| What there is, page by page | [`docs/README.md`](docs/README.md) |
| How the services fit together | [`docs/architecture.md`](docs/architecture.md) |
| Building, running and testing | [`docs/getting-started.md`](docs/getting-started.md) |
| What is planned, and in what order | the roadmap, in anahita-web: `docs/plans/roadmap.md` |
| Why something is as it is | its plan in [`docs/plans/`](docs/plans/) or `docs/plans/done/`: *What was built* and *Different from the plan* |
| Every route, what it takes and sends | [`docs/api/`](docs/api/index.html) |

Roadmap rows are referred to by number. Read the row and its plan before
starting one, and keep the plan's status up to date as work lands.

## Building and testing

| Command | Does |
| --- | --- |
| `make up_build` | Builds and starts the whole stack. Use it after a change to `anahita-libs`, the node entity or a `.proto`, not single services |
| `make <service>` | Rebuilds and restarts one service (`make graph`, `make auth`, …) |
| `make test` | Every module's tests, the order check of `legacy/upgrade.sql`, `make docs_check`, and `make schema_test` (needs Docker) |
| `make docs_check` | Every link between the docs, and every `#heading`, leads somewhere |
| `make api_docs` | Writes the API reference again; commit it with any change to a route or to what it takes or sends |

Tests that need a database run only when given one, through a
`*_TEST_DSN` variable: a throwaway MySQL, never the database you develop
on ([getting-started.md › Tests](docs/getting-started.md#tests)). Remove a
throwaway container with `docker rm -fv`, which takes its volume too.

People and places made up for a test are named after Star Trek characters
and places.

## Rules that have each been broken once

1. **A schema change is three things in one commit:** a goose migration in
   `migration-job/migrations/`, the same change in
   `migration-job/schema/init.sql`, and the same in
   `migration-job/legacy/upgrade.sql`. Then `make schema_test`
   ([migration-job/migrations/README.md](migration-job/migrations/README.md)).
2. **NULL only where it means something:** no other row, made here, no
   place, a date that has not happened or does not apply, or a unique column
   with nothing in it. Everything else is `NOT NULL` with `''`, `0` or false.
   COALESCE only for those, for a `LEFT JOIN`ed row, an aggregate over no
   rows, or a choice between two values
   ([architecture.md › The database](docs/architecture.md#the-database)).
3. **A single node read from the graph is not access-checked.** Before
   showing a post reached by its id, ask `handlers.MaySee` (or
   `GraphNodeGRPC.FilterVisible`).
4. **An address somebody typed is fetched only through `anahita-libs/safehttp`,**
   which goes to public addresses alone.
5. **Do not run `go mod tidy` on `anahita-libs`.** Add a library with
   `go get` at a pinned version that moves nothing else.
6. **Compare two schemas row by row:** `information_schema.COLUMNS` and
   `STATISTICS`, one row per column, then a diff. `GROUP_CONCAT` cuts at
   1,024 characters. Rehearse a change to data on a copy, and time it.
7. **Remove "not built" from the docs the day it is built,** and grep the
   docs for a feature whenever it changes.

## Code

- Raw SQL is for people to read: each query a named constant, one clause a
  line, one column a line when there are several, a short comment on
  anything not obvious. Squirrel: one builder call a line.
- A date with a time ends in `_at`; a date alone in `_on`.
- Live sites run on Kubernetes; docker-compose is for development.
- In zsh, write `"${var}:latest"`, not `$var:latest` (`:l` is a modifier).

## Product principles

- Fediverse first: ActivityPub and AT Protocol natively; nothing that does
  not federate sensibly. Federation is opt-in for each profile: sharing is
  consented to, never assumed.
- Sovereignty: S3-compatible storage only, never the local filesystem; no
  service only one cloud has; offline tools where possible.
- An optional feature is a separate, swappable service behind a small
  contract.
- The default feed is chronological. There is no site-wide "For You"; a
  ranked feed is one a person makes, and is always called a **custom feed**.
- Nobody is tracked: no profile analytics, share counts or viewer lists.
- Old and useful counts as much as new: nothing is pushed down only for
  being old, and how fast the past fades is the operator's choice.
- The breadth (groups, events, articles, custom feeds) is the product, for
  schools, newsrooms, libraries, research groups and non-profits.
- Email is off unless a person turns it on. A notification is marked sent
  before its email goes; a failed send never resends the batch; mail has a
  site-wide off switch.

## Wording, in anything people read

- A group has **followers**, never members. "Member" is somebody with an
  account on the site.
- **Location**, as Instagram has it, for a place posts are tagged with; "Place"
  only for the ActivityStreams type a post from elsewhere carries.
- **Profile**, not "actor" or "account". "Actor" is the code's word;
  "account" is for signing in.
- **Account type** for registered, administrator or super administrator,
  never "user type", "person type" or "role"; `person_type` is the code's
  word.
- **+ Follow**, with no name after it: no "Follow back", no "Follow #tag".
- Action labels name no entity: "Delete", not "Delete reply", unless two
  actions in one menu would otherwise read the same.
- Releases are **Embryo** (the main branch) and **Birth** (a tagged
  release), not alpha and beta.

## Docs

- One set of docs, for people and agents alike: descriptive, plain and
  current. The same word for the same thing every time; exact paths,
  settings, endpoints and defaults, in tables where there are several.
- A phase is not finished until its docs match the code. A finished plan
  moves to `docs/plans/done/` in the same commit, with every link to it
  fixed.
- When a feature, a setting or an endpoint changes, grep the docs of both
  repositories for its name and change every page that says it. Remove "not
  built" the day it is built.
- A renamed heading breaks the links to it: `make docs_check` names them.

## Commits and branches

- One branch per milestone or feature, from an up-to-date `main`
  (`git fetch` first); one commit per item.
- A commit message is a short subject line, a blank line, then one line per
  change, each beginning with its kind: `- added:`, `- changed:`,
  `- removed:`, `- fixed:` or `- note:` (what stays the same, what it goes
  with). Only the kinds that apply.
