# Anahita Microservices

- **Website:** www.anahita.io
- **Founder & Architect:** user Mehr

## Architecture

Anahita is a social networking platform and framework.

- **Graph Architecture:** Anahita uses a graph architecture where actors (people, groups, events, etc.), media (photos, text, videos), and tags (hashtags, locations) are nodes. Relationships between these entities are represented as edges.
- **Separate Databases:** Other data entities, such as notifications, authentication data (logs, credentials, OAuth tokens, WebAuthn), are stored in separate database tables outside the graph.
- **Compatibility and Scalability:** All design decisions should prioritize compatibility and scalability with ATProtocol and ActivityPub, and facilitate easy integration into the fediverse.

## Coding Guidelines

Please adhere to these guidelines:

- **Readability and Maintainability:** Ensure that your code is readable and easy to maintain for human developers.
- **Commit Messages:** Keep commit messages concise, human-friendly, and informative.
- **Release Terminology:** Instead of using “Alpha” and “Beta” releases, we use “Embryo” and “Birth” releases. The main branch is always the Embryo, while tagged releases are Birth releases.
- **Testing Names:** Use Star Trek names of characters and places when testing.
- **Commit Before Committing:** Always ask for permission before committing and display the commit message.
- **Main Branch Practice:** If you are on the main branch, create a branch under “issues” before committing.
- **Documentation Updates:** After building new features or modifying the underlying architecture, update the documentation using human-friendly language to help developers quickly understand and learn it.
- Commit messages: a short subject line, a blank line, then one line per
  change, each starting with its kind: `- added:`, `- changed:`,
  `- removed:` or `- note:` (what stays the same, what to merge with).
  Only the kinds that apply.

## Working with User

- Number every list in replies (1., 2., 3.), not bullets, so items can be
  referred to by number. Keep the numbers stable when a list is repeated.
- Roadmap rows are referred to by number. Check `docs/plans/roadmap.md` and
  the row's plan file before starting work, and update the plan's status as
  work lands.
- When a name or direction is still open, settle it before making bulk
  changes.
- One branch per milestone (or per feature), one commit per item, in each
  repo. user pushes and merges. Branch from an up-to-date `main`
  (`git fetch` first). Don't suggest merging mid-milestone.
- `gh` is not installed: write PR text for user to paste.
- Before beginning a milestone, check the docs of both repos against the
  code.

## Product principles

- Fediverse first: ActivityPub and AT Protocol natively; nothing that
  doesn't federate sensibly. Federation is opt-in per person: sharing must
  be consensual.
- Sovereignty: S3-compatible storage only, never the local filesystem; no
  AWS-only services; offline tools where possible.
- Optional features are separate, swappable services behind small
  contracts.
- The default feed is chronological. No site-wide "For You". A ranked feed
  is one a person makes, and is always called a **custom feed**.
- No tracking of who viewed what: no profile analytics, share counts or
  viewer lists.
- Old and useful counts as much as new. Anahita builds knowledge systems as
  well as news: never push content down only for being old. Weigh whether
  it is still found useful, and leave how fast the past fades to the
  operator.
- Anahita is for operators and outside builders (schools, newsrooms,
  libraries, research groups, non-profits) leaving Meta and X. Its breadth
  (groups, events, articles, custom feeds) is the product, not its
  lightness.
- Email is off unless a person turns it on. Mark each notification sent
  before its email goes out; a failed send never resends the batch; keep a
  site-wide off switch for mail.

## Wording

- A group has **followers**, never members ("abonnés" in French). "Member"
  is fine for somebody with an account on the site.
- "Profile", not "actor" or "account", in anything people read. "Actor" is
  the code's word. "Account" is fine for sign-in and credentials.
- One follow wording everywhere: **+ Follow** (a plus icon, then Follow),
  with no name after it: no "Follow back" or "Follow #tag".
- Action labels carry no entity name: "Delete", "Edit", "View", not
  "Delete reply". Keep a name only where two actions in one menu would read
  the same.

## Services (anahita-services)

- Raw SQL in repositories is laid out for people to read: each query a
  named constant, one clause per line, one column per line when there are
  several, a short comment on anything non-obvious. For squirrel builders,
  one builder call per line.
- Every schema change needs a goose migration AND the same change in
  `schema/init.sql` and `legacy/upgrade.sql`. Test on a throwaway MySQL,
  never on the dev database.
- Remove throwaway MySQL containers with `docker rm -fv` (the volume goes
  too). If a statement hangs with no CPU use, check the disk (`df`) before
  tuning SQL.
- Compare two schemas row by row (`information_schema.COLUMNS` and
  `STATISTICS`, one row per column, then diff). `GROUP_CONCAT` is cut at
  1,024 characters. Rehearse a data migration on a copy of the dev data and
  time each statement.
- After a change to the node entity or a protobuf, rebuild the whole stack
  with `make up_build`, not single services. Start the stack with
  `make up` / `make up_build`, which create `./grpc-certs`.
- Don't run `go mod tidy` on `anahita-libs`. Add libraries with `go get` at
  pinned versions.
- New timestamp columns and fields end in `_at` (`_on` only for date-only
  columns).
- `MediaGRPC.Read` and the graph's single-node reads are not access-checked.
  Ask `GraphNodeGRPC.FilterVisible` (or `handlers.MaySee`) before showing a
  post reached by id.
- Live sites run on Kubernetes; docker-compose is for local development
  only.
- With minikube, always pass `--context minikube`, and switch back to the
  `protect-services` EKS context afterwards.
- In zsh, write `"${var}:latest"`, not `$var:latest` (`:l` is a modifier).

## Docs

- Docs are for people: descriptive, plain and current. One set for people
  and AI alike.
- Use the same term for the same thing every time. Give exact paths,
  settings, endpoints and defaults, in tables where there are several.
- Remove "not built yet" lines the moment something is built.
- A phase isn't done until its docs are updated.
- When a roadmap row is done, move its plan to `docs/plans/done/` in the
  same commit, and fix every link to it.

