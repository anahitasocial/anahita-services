# Dissolve `people_people`: move person and user data into graph-grpc-service

## Context

`people_people` is one table written by two services. `person-service` owns the
profile half (`given_name`, `family_name`, `gender`); `auth-service` owns the
credential half and additionally reads and writes `nodes` directly.

`person-service` and `auth-service` are the only two HTTP services in the fleet
that still own a repository. Group, photo, text, socialgraph, avatar, cover,
feed, like, hashtag, location, story, search and comment have none.

Two ownership violations follow:

- `auth-service/data/repositories/users.go:272` runs `INSERT INTO nodes`;
  `:384,487,527` run `UPDATE nodes`. Signup, username change and account restore
  write the graph's table without going through graph-grpc.
- `person-service/data/repositories/mention_relation.go` runs `INSERT`/`DELETE`
  on `edges` directly.

**Outcome:** `people_people` is gone. Identity lives on `nodes`, credentials live
in a new `auth_accounts` table. `person-service` has no repositories and
matches `group-service`. `auth-service` stops writing `nodes`.

This overturns the decision recorded at
`person-service/data/repositories/people.go:291-294` — graph "has never read
`people_people` and should not start."

---

## Status

| item | state | commit |
| --- | --- | --- |
| 0. `UserType*` → `PersonType*` | ✅ done | `ce6401b` |
| 1. Rewrite `init.sql` / `upgrade.sql` | ✅ done | `2241802`, `54d0e88` |
| 2. New columns + actor-alias index | ✅ done | `1273320` |
| 4. graph carries the new fields | ✅ done | `5f8ffeb` |
| 8. auth stops writing `nodes` | ✅ done | `e426286` |
| 6. person-service loses its repositories | ⬜ next | |
| 7. Mention relations to graph | ⬜ | |
| 5. Last-super-admin guard into graph | ✅ done | `57c181c` |
| 10. Signup, invite-accept, first super admin | ⬜ | |
| 3. Backfill and drop `people_people` | ⬜ | |
| 9. anahita-react | ⬜ | |

Also landed alongside: `fcbae16` (one `make test` target instead of two
shadowing each other) and `30297f1` (these plan documents).

**Sequencing correction.** The order below omitted item 4, but item 8 has to set
`person_type` through `ActorsGRPC` and cannot until the graph accepts the field.
The real order is **1 → 2 → 4 → 8 → 5 → 6 → 7 → 10 → 3 → 9**. Item 5 also
had to move ahead of item 6: person-service cannot drop its repository while
four of its handlers still call `CountOtherActiveSuperAdmins` on it.

**Applied to the reference database.** Items 1 and 2 have been run against the
live `anahita` database, which was recreated from `db-data/production.data.sql`
through the rewritten `upgrade.sql`: 23,058 nodes, 31,431 edges, 822 people,
zero data defects.

---

## What the live data says

Measured on the running `anahita` database, 823 people.

| Check | Result | Consequence |
| --- | --- | --- |
| `nodes.alias` vs `people_people.username` | 0 of 823 differ | no backfill needed |
| `nodes.name` populated on person nodes | 823 of 823 | no backfill needed |
| `nodes.timezone` vs `people_people.time_zone` | 0 mismatches | exact duplicate |
| `nodes.language` vs `people_people.language` | 0 mismatches | exact duplicate |
| duplicate aliases among 846 actor nodes | 0, even case-insensitively | unique index applies cleanly |
| duplicate aliases among all nodes | 244 | index must be actor-scoped |
| `network_presence <> 0` | 0 rows | dead column |
| `activation_code` Go references | 0 (215 legacy rows) | dead column |
| `gender` set | 263 of 823 | dropped, not converted |
| `last_visit_date` writes in Go | INSERT only, never on login | frozen value — open question 3 |

`given_name`/`family_name` are **already corrupted**: 23 rows are truncated at
the first non-ASCII character (`José`→`Jos`, `Sellström`→`Sellstr`) while
`nodes.name` holds the correct value in every case.

---

## Column disposition

| Column | Destination |
| --- | --- |
| `people_person_id` | drop — surrogate key, no Go reader |
| `node_id` | PK of `auth_accounts` |
| `given_name`, `family_name` | drop → `nodes.name` |
| `username` | drop → `nodes.alias` |
| `usertype` | → `nodes.person_type` |
| `gender` | drop → new `nodes.person_pronouns` |
| `last_visit_date` | → `nodes.last_visit_date` |
| `time_zone`, `language` | drop — `nodes.timezone`/`nodes.language` already identical |
| `network_presence`, `activation_code` | drop — dead |
| `email`, `password`, `totp_secret`, `totp_enabled`, `totp_recovery_codes`, `totp_last_verified_at` | → `auth_accounts` |

---

## Naming convention

Settled, and it governs every rename below.

| Term | Means | Where |
| --- | --- | --- |
| **person** | the social identity — one per human, held on `nodes` | every service, including auth |
| **account** | the login account: credentials, TOTP, consent | auth-service only, over `auth_accounts` |
| **user** | *only* where an external specification says so | `/oauth/userinfo`, the OIDC `sub` claim, and the `user_id` columns on `oauth_authorizations`, `oauth_refresh_tokens`, `webauthn_credentials` |

The rule exists because `User` and `Person` are near-synonyms, and two
near-synonyms naming the same row is what let them drift in the first place:
`auth.entities.User` and `entities.Person` are today near-duplicate structs over
the same row, both embedding `AnahitaNode`, both carrying
`UserType`/`Email`/`Username`/`Password`. `Account` and `Person` cannot be
confused for one another.

Protocol vocabulary is deliberately exempt. `user_id` in the OAuth and WebAuthn
tables is spec surface, not domain modelling, and renaming it would fight the
specification for no gain.

---

## Work items

### 0. Mechanical rename: `UserType*` → `PersonType*`  ✅ DONE (ce6401b)

Its own commit, landed **before** anything else, because it touches services
this plan does not otherwise go near — group-service, middleware, feed-service —
and a bisect should point at the rename rather than at the refactor.

- `anahita-libs/constants/users.go` — `UserTypeGuest`, `UserTypeRegistered`,
  `UserTypeAdmin`, `UserTypeSuperAdmin` → `PersonType*`. Rename the file to
  `person_types.go` while there.
- `entities.Viewer.UserType` → `PersonType`; `entities.Person.UserType` →
  `PersonType`.
- `graphpb` `Viewer.user_type` → `person_type`. Field *numbers* are unchanged,
  so the wire format is unaffected; only the generated accessors change.
- 82 call sites across 16 files. Every one is compiler-checked, so the risk is
  churn rather than correctness.

The **values** are untouched — `guest`, `registered`, `administrator`,
`super-administrator` are what the database holds and what any existing token
carries.

### 1. Rewrite `init.sql` and `legacy/upgrade.sql`  ✅ DONE (2241802, 54d0e88)

`init.sql` cannot currently produce a working install. It declares `time_zone`,
`enabled_optional_features`, `actor_gender` and `permissions`; the live schema
has `timezone` and `features` and none of the others. The Go read path selects
`node.features` and `node.timezone`
(`graph-grpc-service/data/repositories/helpers.go:96`), so a fresh install fails
every node read with `Unknown column 'node.features'`.

Rewrite both to match the live schema plus the target below. Also:

- convert the schema to `utf8mb4` / `utf8mb4_0900_ai_ci` throughout — `nodes` is
  currently `utf8` with only `name`/`alias`/`body` overridden, which is the
  likely cause of the truncated `given_name` values;
- delete the 123 `nodes` rows carrying the legacy type
  `ComComponentsDomainEntityAssignment,…`, which `legacy/upgrade.sql:45` misses
  because it deletes by `component NOT IN (...)`.

### 2. Add the new columns and the actor-alias index  ✅ DONE (1273320)

```sql
ALTER TABLE nodes
  -- guest / registered / administrator / super-administrator.
  -- NOT a node type string, unlike owner_type and parent_type.
  ADD COLUMN person_type     varchar(50)  DEFAULT NULL,
  -- Free text, e.g. "she/her", "iel". Never an enum, never used for logic.
  ADD COLUMN person_pronouns varchar(50)  DEFAULT NULL,
  ADD COLUMN last_visit_date datetime     DEFAULT NULL,
  ADD KEY person_type (person_type),
  ADD COLUMN unique_alias varchar(300)
    GENERATED ALWAYS AS (
      CASE
        WHEN alias IS NULL OR alias = '' THEN NULL
        WHEN type LIKE 'node.actor.%'    THEN CONCAT('actor:', alias)
        ELSE NULL
      END
    ) STORED,
  ADD UNIQUE KEY uq_unique_alias (unique_alias);

CREATE TABLE auth_accounts (
  `node_id`               bigint unsigned NOT NULL,
  `email`                 varchar(255) DEFAULT NULL,
  `password`              varchar(255) DEFAULT NULL,
  `totp_secret`           varchar(255) DEFAULT '',
  `totp_enabled`          tinyint(1) NOT NULL DEFAULT 0,
  `totp_recovery_codes`   text DEFAULT NULL,
  `totp_last_verified_at` datetime DEFAULT NULL,

  -- Terms of Service acceptance. The version is recorded alongside the
  -- timestamp so a revision can re-prompt only those on an older version,
  -- and the ip/user_agent/os trio is what makes the record evidential
  -- rather than merely a boolean.
  `tos_accepted`            tinyint(1) NOT NULL DEFAULT 0,
  `tos_accepted_at`         datetime     DEFAULT NULL,
  `tos_accepted_version`    varchar(20)  DEFAULT NULL,
  `tos_accepted_ip`         varchar(45)  DEFAULT NULL,
  `tos_accepted_user_agent` varchar(255) DEFAULT NULL,
  `tos_accepted_os`         varchar(100) DEFAULT NULL,

  -- Privacy Policy acceptance, same shape.
  `pp_accepted`             tinyint(1) NOT NULL DEFAULT 0,
  `pp_accepted_at`          datetime     DEFAULT NULL,
  `pp_accepted_version`     varchar(20)  DEFAULT NULL,
  `pp_accepted_ip`          varchar(45)  DEFAULT NULL,
  `pp_accepted_user_agent`  varchar(255) DEFAULT NULL,
  `pp_accepted_os`          varchar(100) DEFAULT NULL,

  PRIMARY KEY (`node_id`),
  UNIQUE KEY `email` (`email`),
  KEY `tos_accepted_version` (`tos_accepted_version`),
  KEY `pp_accepted_version` (`pp_accepted_version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**Named `auth_accounts`, not `auth_credentials`** — it now holds legal consent as
well as secrets, and "credentials" would be the wrong label for a table
recording what somebody agreed to. One table rather than two: it is 1:1 on
`node_id`, so a second table would buy a join and nothing else.

**Two details worth getting right:**

- `*_accepted_at` defaults to `NULL`, not `NOW()`. A row that never accepted
  anything must not carry a timestamp, or the pair
  `(accepted=0, accepted_at=<some date>)` is ambiguous and the audit trail
  becomes unreadable.
- The version columns are indexed. Revising the terms means finding everyone on
  an older version to re-prompt, which is a query on exactly this column.

The `_ip` / `_user_agent` / `_os` columns are the audit trail that makes the
record legally useful — without them you know somebody clicked, not who or from
where. `varchar(45)` fits an IPv6 address.

*A newsletter opt-in does not belong here.* It is a delivery preference, and
`notification-service` already owns those.

Notes on the index:

- **It must exist.** `nodes.alias` has no index at all today, so handle
  uniqueness is enforced *only* by `people_people.username`. Dropping that table
  without this silently loses the guarantee.
- **It must be actor-scoped.** A global unique index is impossible — 244 alias
  values are duplicated, legitimately: article revisions share their article's
  slug, and a location, a photo-set and its photos share a place slug. MySQL has
  no partial indexes, hence the generated column.
- **All actor types share one namespace.** WebFinger resolves
  `acct:name@domain` to exactly one actor and has no type dimension, so a person
  and a group with the same alias would make both unaddressable once Anahita
  federates.
- The `alias <> ''` guard is required because MySQL allows many `NULL`s in a
  unique index but treats `''` as an ordinary value.
- The `actor:` prefix looks redundant with one branch; it is there so a second
  namespace is a one-line `CASE` addition rather than a new column and index.

### 3. Backfill and drop  ⬜ TODO

```sql
UPDATE nodes n JOIN people_people p ON p.node_id = n.id
SET n.person_type     = p.usertype,
    n.last_visit_date = p.last_visit_date;

INSERT INTO auth_accounts
  (node_id, email, password, totp_secret, totp_enabled,
   totp_recovery_codes, totp_last_verified_at)
SELECT node_id, email, password, totp_secret, totp_enabled,
       totp_recovery_codes, totp_last_verified_at
FROM people_people;

DROP TABLE people_people;
```

Only two columns move to `nodes`; `name`, `alias`, `timezone` and `language` are
already correct there. Export `gender` to a file first if you want the 263
values recoverable — they are dropped, not converted.

### 4. graph-grpc-service: carry the new fields  ✅ DONE (5f8ffeb)

- `data/repositories/helpers.go` — add `person_type`, `person_pronouns`,
  `last_visit_date` to the shared select. This file keeps the column list and
  two positional `.Scan(...)` lists (`FetchNode`, `FetchNodes`) in lockstep; a
  column must be added to all three or every read fails at runtime.
- `cmd/server/handlers/actor_fields.go` — add the fields to
  `applyEditableActorFields` and `newActorFromRequest`. A field missing from
  these whitelists is silently dropped on Edit.
- `anahita-libs/proto/graphpb/node.proto` — add the three fields as new field
  numbers (next free is 78; do not reuse 30, which is `reserved`).
- Reject an alias that collides with an existing actor in `Add`/`Edit` with a
  validation error, rather than surfacing a duplicate-key error. Creating a
  group whose alias matches a person's will start failing; nothing enforces
  that today.

### 5. graph-grpc-service: own the last-super-admin guard  ✅ DONE (57c181c)

Add `CountOtherActiveSuperAdmins` to `NodesRepoInterface`. With `person_type` on
`nodes` it is a single-table query:

```sql
SELECT COUNT(*) FROM nodes
WHERE person_type='super-administrator' AND id<>? AND deleted_at IS NULL AND enabled=1
```

Call it from `SoftDeleteActor`, `ArchiveActor`, `SetEnabled` and
`PurgeActorContent`. The check is currently duplicated across four
`person-service` handlers (Delete, Archive, SetEnabled, Purge); moving it into
the lifecycle methods removes all four copies and closes the gap where a new
handler forgets it.

### 6. person-service: delete both repositories  ⬜ TODO

- Delete `data/repositories/` entirely and the `Repositories` wiring in
  `cmd/api/main.go:84`. The service becomes structurally identical to
  `group-service`.
- `cmd/api/handlers/person.go` — remove the `fetchPerson` stitch (`:750-761`)
  that merges a `people_people` row into an actor node; the actor node is now
  whole. The `Browse` nil-deref at `:102` disappears with it.
- `responses/person.go` — `PersonNodeItem` loses `email`, `given_name`,
  `family_name`, `gender`; `username` becomes `alias`, `usertype` becomes
  `person_type`, `person_pronouns` is added.
- `anahita-libs/requests/person.go:46` — replace `GivenName`/`FamilyName` with
  `Name string \`form:"name" validate:"required,max=100"\``. Drop
  `alphanumunicode`: it is `^[\p{L}\p{N}]+$`, so `Jean-Luc`, `O'Brien` and
  `van der Berg` cannot be saved today.

### 7. Move mention relations to graph  ⬜ TODO

`mention_relation.go` needs no new RPCs — every query maps to an existing one:

| current direct SQL | replacement |
| --- | --- |
| `SELECT id FROM nodes WHERE alias IN (...) AND type=person` | `NodeService.BatchReadByAliases` |
| `SELECT node_a_id FROM edges WHERE ...` | `EdgeService.BatchReadByFilters` |
| `INSERT INTO edges` | `EdgeService.BatchAdd` |
| `DELETE FROM edges` | `EdgeService.BatchDeleteByFilters` |

### 8. auth-service: stop writing `nodes`  ✅ DONE (e426286)

- `data/repositories/users.go` — the `INSERT INTO nodes` / `UPDATE nodes`
  statements (`:272,384,487,527`) become `ActorsGRPC` calls. The repository
  shrinks to `auth_accounts`.
- `UsersInterface` splits: identity reads (`ReadByUsername`, `Browse`) go to
  graph; credential reads (`ReadByEmail`, `EditPassword`, TOTP) stay.
  `ReadByEmailOrUsername` becomes two lookups — email in `auth_accounts`,
  alias in graph — resolving to the same `node_id`.
- `data/services/jwt.go:155-157` — the `name` claim comes from `node.Name`. Drop
  the `given_name`/`family_name` claims and their entries in
  `oauth_wellknown.go:68-70`; both are optional OIDC claims and `name` is
  currently derived from them anyway.
- `cmd/api/notifiers/*` — roughly 25 call sites use `user.GivenName` for
  greetings; these become `user.Name`.
- Delete `cmd/api/handlers/user.go` — dead code, `NewUserHandler` is never
  called and no `/users` route exists.

### 9. anahita-react  ⬜ TODO

- `src/utils/node.js` — `getPersonName` (`:100`) and `getActorInitials` (`:78`)
  already prefer `name` with a `givenName familyName` fallback; simplify once the
  fallback is unused. `readIdentifier` (`:242`) can stop branching on namespace
  once every actor is addressed by alias.
- Forms — `AuthSignupForm.jsx`, `containers/people/Settings/InfoForm.jsx`,
  `containers/people/Add/PersonAddForm.jsx`: one **Name** field instead of two,
  and the `gender` select replaced by a free-text **Pronouns** input.
- Pronoun display — `getPersonName` is the chokepoint most name rendering goes
  through, so one helper covers most surfaces. Renders nothing when empty.
- `usertype` → `person_type` at `src/utils/node.js:35,39,43` and
  `src/proptypes/Person.js:22`.
- `src/proptypes/Person.js`, `PersonDefault.js`, `Viewer.js`, `ViewerDefault.js`.
- `src/languages/en-GB/people.js` and `fr-FR/people.js` — `givenName`/
  `familyName` labels collapse to `name`; `gender` becomes `pronouns`.
- `containers/people/Settings/InfoRead.jsx` and
  `containers/actors/Notifications/Form.jsx` also reference `gender`.

---

### 10. Signup, invite-accept and the first super administrator  ⬜ TODO

Three gaps found while checking how a person node gets created. The first two
block this refactor; the third blocks any fresh install.

**Actions:**

- **Signup collects a single `Name`**, with the same validation as the person
  edit request (`required,max=100`, no `alphanumunicode`). Invite-accept's
  `GivenName`/`FamilyName` pair collapses to the same field.
- **Unify the password rule** across both paths. Invite-accept's `min=15` is the
  defensible one; signup's `min=6` is the outlier.
- **Both paths collect ToS and Privacy acceptance** and write the twelve
  `auth_accounts` columns, including IP, user agent and OS taken from the
  request rather than the form. The request gains `tos_agreement bool` and
  `tos_version string` (`validate:"required,version_number"`) plus the privacy
  pair. Acceptance is captured **at account creation**, not deferred — an
  account should not exist without it.
- **Seed the first super administrator at first boot**, out of band. A
  `SUPER_ADMIN_EMAIL` env var; on boot, if no node has
  `person_type = 'super-administrator'`, auth-service creates the person node
  via `ActorsGRPC` plus its `auth_accounts` row. This follows the existing
  bootstrap pattern in `auth-defaults/oauth_seed_clients.json`, which seeds
  first-party OAuth clients the same way — and it must be **idempotent and
  skip-if-exists** for the same reason that file is: a redeploy must never
  silently re-create or overwrite. The signup endpoint keeps assigning
  `registered` unconditionally, so there is no race window in which a stranger
  can claim the installation.

#### How the first super administrator logs in

The seeded account is created with an **empty password hash, which is
deliberately unusable** — `TestEmptyPasswordHashIsUnusable`
(`auth-service/data/repositories/users_null_test.go:157`) already pins that
behaviour. So the account cannot be signed into until a password is set, and
there is nothing to guess in the meantime.

Access comes from a `passwordreset` token in the existing `tokens` table (64
random bytes hex-encoded, TTL, single-use), landing on the existing
server-rendered `auth-service/web/templates/password_reset.html`. No new flow.

**Deliver that token two ways, and this is the important part:**

1. email it to `SUPER_ADMIN_EMAIL`, and
2. **print the full URL to the container log.**

The log line is not a convenience, it is the guarantee. On a fresh deployment
SMTP is frequently the last thing configured and the first thing misconfigured;
if email were the only path, a bad `SMTP_*` setting would lock the operator out
of a brand-new installation with no recovery except hand-written SQL. The
operator reads it with `docker compose logs auth-service`. Both mechanisms
already exist, so this costs almost nothing.

Give the bootstrap token a **long TTL** (24h+) rather than the normal reset
window — the operator may not read the logs immediately — and re-issue it on
each boot while the seeded account still has no password, so a stale link is
never a dead end.

Remaining details for this path:

- **Alias**: derive from the email local-part, slugified, with a numeric suffix
  on collision. Optionally `SUPER_ADMIN_USERNAME` to set it explicitly.
- **Name**: `SUPER_ADMIN_NAME`, defaulting to the email local-part. `nodes.name`
  is `NOT NULL` in spirit now that it is the display name, so it cannot be left
  empty.
- **Terms acceptance**: the seeded admin has accepted nothing, and the
  post-login onboarding flow is a separate plan. Rather than depend on it, put
  the ToS and Privacy checkboxes **on the set-password page** — the admin
  accepts as they claim the account, and `auth_accounts` gets an honest record
  with a real IP and user agent. This keeps item 10 self-contained.
- The **post-login onboarding flow** (avatar, bio, favourite topics, and
  re-acceptance when a terms version changes) is a separate plan —
  `docs/plans/onboarding.md`, to be written. Only the account-creation half is
  in scope here.

**1. Signup collects no name.** `auth-service/requests/signup.go:13-17` takes
only email, username and password, and `signup.go` never sets a name. Every one
of the 823 existing people has `nodes.name` populated because they are all
legacy rows — a Go-created signup lands with a NULL name. Since this plan makes
`nodes.name` *the* display name, **signup must collect it**, or onboarding must
set it before the profile is usable.

**2. Signup and invite-accept have diverged.**

| | signup | invite-accept |
| --- | --- | --- |
| transport | JSON API | server-rendered form |
| name | **not collected** | given/family, both required |
| password minimum | **6** | **15** |
| ToS acceptance | none | none |

The password asymmetry is the notable one: six characters on the public path,
fifteen on the invited path. Both paths need the same rules and the same single
`Name` field after the collapse.

**3. Nothing in the Go stack ever assigns `UserTypeSuperAdmin`.** It appears
only in comparisons (`person.go:336,427,441,519,581,601`,
`group.go:601`, `viewer.go:68`) — never as an assignment. So a fresh install
from `init.sql` has **no way to create a super administrator at all**; the one
in the live database came from the legacy migration. This needs solving
independently of the refactor, and it is where `person_type` gets written for
the first time.

**ToS acceptance is specified but does not exist.** Four comments describe the
intended mechanism — `requests/invite.go:14-17` and
`invite_accept.go:28-29,69,147-148` say agreement happens "in the SPA via the
post-login AgreementsPage interstitial gated by AuthenticatedRoute." But
`AgreementsPage` does not exist anywhere in `anahita-react`, and there is no
`terms_accepted` column in the schema. The content exists
(`src/assets/default/pages/tos.md`, `privacy.md`); the capture does not. So
acceptance is currently recorded nowhere.

That also means the **onboarding-at-first-login** approach is already the
documented intent, just never built.

## Field notes

**`person_type`** — named for the node, not for auth: graph has no notion of a
"user". Matches the `owner_type`/`parent_type` convention, with a column comment
noting that unlike those two it holds `registered`/`super-administrator`, not a
node type string. Values and the `constants.UserType*` names in
`anahita-libs/constants/users.go` are unchanged; only the column, the struct
field (`AnahitaNode.PersonType`) and the JSON key move. `IsAdmin()` /
`IsSuperAdmin()` keep working.

**`person_pronouns`** — free text, optional, displayed alongside the author name.
Not an enum: an enum hard-codes English grammar, is meaningless in Persian,
Turkish and Finnish, which have no gendered third-person pronoun, and is awkward
in French where `il`/`elle` is grammatically determined. Empty renders nothing —
no parentheses, no separator. Federates as a `PropertyValue` attachment, the
same mechanism that will carry `website_url`.

The existing `gender` field is broken independently of this: the validator allows
`oneof=male female other` (`anahita-libs/requests/person.go:49`) while the
handler only stores `male`/`female`/`neutral`
(`person-service/cmd/api/handlers/person.go:212-215`), so `other` passes
validation and is discarded, and the 7 existing `neutral` rows would fail
validation.

**`email` stays out of graph.** `graphpb.Node` is the universal payload returned
by `NodeService`, `ActorService`, `MediumService`, `SearchService` and
`FeedService` to sixteen callers; any field on it is one missing access check
away from public. `/people/` currently returns `email` for every person
(`person-service/responses/person.go:13`) — removing the repository removes that
exposure. React reads `person.email` only in the admin Add form.

**Aliases are `utf8mb4_0900_ai_ci`** — case- and accent-insensitive. Case
insensitivity matches auth's `LOWER()` comparison in `IsUsername`; accent
insensitivity means `renee` and `renée` become the same handle.

---

## Sequencing

Each step leaves the stack working: 1 → 2 → 8 → 6 → 7 → 10 → 3 → 9.

Item 8 (auth stops writing `nodes`) is the biggest correctness win and is
independent of the rest, so it can land early. Item 10 must precede item 3,
because the super-admin seed writes `person_type` and the signup changes write
`auth_accounts` — both need the new schema but must work before the old table
goes. Item 3 (the drop) comes last among the backend steps, once nothing reads
the table.

---

## Verification

```bash
# schema matches the live DB after the rewrite
docker compose exec -T mysql mysql -uroot -proot anahita -e "
  SELECT COLUMN_NAME FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA='anahita' AND TABLE_NAME='nodes' ORDER BY ORDINAL_POSITION;"

# nothing lost in the move — expect 823 / 823 / 1
docker compose exec -T mysql mysql -uroot -proot anahita -e "
  SELECT (SELECT COUNT(*) FROM nodes WHERE type='node.actor.person-service.person.v1') AS people,
         (SELECT COUNT(*) FROM auth_accounts) AS creds,
         (SELECT COUNT(*) FROM nodes WHERE person_type='super-administrator') AS supers;"

# accented names survive — expect "Johan Sellström"
docker compose exec -T mysql mysql -uroot -proot --default-character-set=utf8mb4 anahita \
  -e "SELECT name FROM nodes WHERE name LIKE '%Sellstr%';"

# index holds, media slugs untouched — expect 0 and non-zero (239 today)
docker compose exec -T mysql mysql -uroot -proot anahita -e "
  SELECT COUNT(*) AS should_be_zero FROM (
    SELECT unique_alias FROM nodes WHERE unique_alias IS NOT NULL
    GROUP BY unique_alias HAVING COUNT(*)>1) d;
  SELECT COUNT(*) AS media_dups_still_allowed FROM (
    SELECT alias FROM nodes WHERE type NOT LIKE 'node.actor.%'
    GROUP BY alias HAVING COUNT(*)>1) d;"

make test          # includes the OAuth scope-literal guard
make graph person auth
```

End-to-end through nginx at `http://localhost:80`:

- `POST /signups` with `Jean-Luc Picard` — must succeed; it cannot today.
- `GET /people/<alias>` — returns `name` and `person_type`; no `email`, no
  `given_name`.
- `PATCH /people/:id` with `person_pronouns` — round-trips and renders beside the
  name; empty renders clean with no stray separator.
- `PATCH /people/:id` with a legacy `gender` field — ignored, not a 500.
- `PATCH /username/change` — `nodes.alias` updates, no `people_people` left to
  fall out of sync.
- Create a group whose alias equals an existing person's — rejected with a
  validation error, not a duplicate-key 500.
- Create two articles with the same title — slugs must still collide happily,
  proving the index did not leak past the actor family.
- `DELETE /people/:id` on the only super-administrator — must still return
  `409 last_super_administrator`.
- Sign in; confirm the `name` claim in the JWT and at `GET /oauth/userinfo`.
- Front end at `http://localhost:3000` — profile renders, settings form saves.

`git grep -n "people_people"` should return only the migration SQL when done.

---

## Open questions

1. **JSON key for `person_type`** — rename the response field too (four React
   call sites, in files already being touched), or keep the wire format as
   `usertype`? Renaming breaks any non-`anahita-react` client.
2. **Pronoun display scope** — every author line, or profile and hover card
   only? Front-end only, no schema impact.
3. **`last_visit_date`** — written only at INSERT and never on login, so it is
   stale for all 823 people. Wire auth to update it through graph at login, or
   drop the column?
4. **Signup atomicity** — signup becomes two writes across two services (graph
   creates the node, auth inserts the `auth_accounts` row) with no transaction;
   a failure on the second leg leaves an orphan person node that owns a handle
   nobody can sign into. Suggest a compensating `ActorService.Purge` from auth
   on failure. This now also applies to the super-admin seed in item 10. Decide
   before item 10.
7. **Terms version source.** Where does the current ToS/Privacy version come
   from — a config value, or read from `anahita-react/src/assets/default/pages/`?
   Something has to be the authority the signup request is validated against,
   since `version_number` validation only checks the shape.
5. **Handle release after purge** — `Users.Delete` leaves the `nodes` row so a
   handle is never re-issued, and the index reinforces that, but the alias is
   then reserved forever. Free it by nulling or suffixing in `TombstoneActor`, or
   leave it reserved?
6. **Reserved aliases** — should graph reject route segments (`people`, `groups`,
   `search`) and names like `admin`/`support` on `Add`? No actor uses one today.

---

## Adjacent findings — out of scope

Recorded so they are not lost. None block this work.

- **`website_url` untouched**, as agreed. A multi-link profile — an
  `actor_links` table with `{label, url, ordering, verified_at}`, or a links
  BREAD service — is a separate design. The migration comment at
  `20260906000001_nodes_website_url.sql` already documents how a labelled list
  slots in without migrating what is stored.
- **8 hashtag aliases collide** (`html5`/`HTML5`, `test`/`Test`, …) with 203
  edges attached. Hashtags are addressed by alias (`GET /hashtags/:alias`), so
  the duplicates are a live ambiguity. Root cause is
  `anahita-libs/data/entities/node_hashtag.go:27`, which does not normalise case,
  plus a read-then-create race at `hashtag_events.go:66-105`. Fixing it means
  merging the pairs and adding a `hashtag:` branch to `unique_alias`.
- **Location duplicates are a search-quality problem.** A place is a named thing
  at a location, so neither name nor coordinates is identity and no unique key is
  possible. But `anahita-react/src/containers/locations/Add/List.jsx:39-40`
  hardcodes `nearby_latitude: 0` / `nearby_longitude: 0`, which disables
  proximity ranking that is fully built on the backend (`nodes.go:56-68`, 20 km
  default). Small front-end fix, prevents most duplicates.
- **26% of edges carry legacy `com:` type strings** — all 8,314 hashtag edges and
  587 location edges, because `legacy/upgrade.sql:104-123` covers people, groups
  and media but not the tag family. These columns are query filters and part of
  `uc_edge UNIQUE(...)`, so untagging can silently no-op and re-tagging can
  insert a duplicate edge. Worth folding into item 1 while that file is open.
- **Article revisions are a clean extraction** — 645 rows, 0 edges, 0
  comments/votes, all parented. They are the main reason `alias` duplicates
  within a single type.
- **`edges` are hard-deleted** (`edges_delete.go:22`, no `deleted_at`). Federated
  `Undo(Follow)` must cite the original activity id, and the edge table is
  otherwise already shaped like an activity log
  (`{id, type, actor, object, published}`). Adding the column now is far cheaper
  than retrofitting an activity log later.
- **`AnahitaNode.Urls []string`** (`anahita-libs/data/entities/node.go:120`) is
  dead — nothing populates or reads it.
- **Onboarding workflow** — post-login steps (avatar, bio, favourite topics) and
  re-acceptance when a terms version changes. To be written as
  `docs/plans/onboarding.md`. Note that four comments
  (`requests/invite.go:14-17`, `invite_accept.go:28-29,69,147-148`) already
  describe an `AgreementsPage` interstitial gated by `AuthenticatedRoute`; it
  was never built, and those comments should be corrected or removed once the
  real mechanism lands.
- **Events are groups with an expiry date** plus organiser features — a separate
  plan. Nothing here blocks it: an event is a `node.actor.…` type, so it joins
  the actor alias namespace automatically, federates as an AP `Group` actor, and
  `nodes` already carries `start_date` / `end_date`.
- **Feed generators / discovery** — `docs/plans/feed-generators.md`.
