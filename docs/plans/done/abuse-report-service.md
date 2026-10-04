# Abuse reports

*Roadmap item 4b, planned 2026-09-28. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

## Context
Signed-in people need to report any node (person, group, article, note, topic, photo, comment, hashtag or location) with a reason. Instances configure the reasons in a JSON file, and we ship a sensible default list. Admins and super-admins work through the reports in the admin area (item 4a). Nothing exists for this yet: no code, no tables, and nothing in legacy Anahita.

## Design

**Data model: reports are grouped into cases, one per reported node.** A node that twenty people report is one case for the admin to handle, not twenty.
- **`abuse_cases`:**
  - `id`, `target_id`, `target_type`, `target_owner_id` (the author or owner at the time of the report, kept after deletion);
  - `status`: `open` | `actioned` | `dismissed`;
  - `report_count`, `first_reported_at`, `last_reported_at`;
  - `decided_by`, `decided_at`, `decision_action` (`none` | `deleted` | `disabled` | `archived` | `blocked` | `other`), `decision_note`;
  - `target_gone_at`.
  - Keys: a unique key on (`target_id`, `status='open'`) enforced in code, and indexes on status and `last_reported_at`.
- **`abuse_reports`:** `id`, `case_id`, `reporter_id`, `reason_key`, `details` (text, max 1000 characters), `created_at`, `created_ip`. Unique on (`case_id`, `reporter_id`): one report per person per case, and a second submission updates the reason and details.
- If a node is reported again after its case was closed, a **new** case opens, and the history stays visible on the case page.
- Following the signup-requests precedent (`20260914000001_signup_requests.sql`), add the tables in three places: a goose migration, `migration-job/schema/init.sql` and `legacy/upgrade.sql`.

**Reasons JSON:** `abuse-report-defaults/reasons.json`. It is read on each request, like `feed-defaults` (`feed-service/data/defaults/loader.go:16-37`), so edits take effect without a restart.
- It is baked into the image at `/app/defaults/reasons.json` and mounted read-only in compose. The env var `REASONS_PATH` **defaults to `/app/defaults/reasons.json`**. That avoids the path mismatch the k8s manifests have for `feed-defaults` and `graph-grpc-defaults` (the images copy to `/app/defaults` but the config defaults to `/defaults`). Fix those two while we're here.
- Entry shape:
  ```json
  { "key": "hate_racism",
    "label": { "en-GB": "Racism", "fr-FR": "Racisme" },
    "description": { "en-GB": "Attacks or slurs based on race or ethnicity", "fr-FR": "..." },
    "applies_to": ["node.actor.", "node.medium.", "node.base.comment-service."],
    "requires_details": false,
    "atproto_reason": "com.atproto.moderation.defs#reasonRude" }
  ```
  - `applies_to` uses type prefixes, and an empty list means every type.
  - `atproto_reason` (and later an ActivityPub `Flag` mapping) is optional and prepares for sending reports to other servers on the fediverse. It is unused in v1.
- **Default list:**
  - Spam
  - Harassment or bullying
  - Racism
  - Sexism
  - Other hate (religion, sexual orientation, gender identity, disability)
  - Violence or threats
  - Self-harm or suicide
  - Sexual content shared without consent
  - Child safety
  - Impersonation (actors only)
  - Private information or doxxing
  - Misleading or false information
  - Intellectual property or copyright
  - Trademark
  - Illegal content or activity
  - Other (details required)
- **Validation at load:** unique keys, and a label present for the default language. An invalid file logs an error and falls back to the copy baked into the image.

**Service: new `abuse-report-service` (Go)**, laid out like `hashtag-service`, with its own dockerfile, compose, nginx `location /abuse-reports/`, k8s manifest, ingress rule, Makefile targets and `go.work` entry.

| Endpoint | Who | What |
| --- | --- | --- |
| `GET /abuse-reports/reasons?type=<node type>` | signed in | The reasons that apply to that type, labels in the requested language |
| `POST /abuse-reports` `{ target_id, reason_key, details }` | signed in (`an_mw.Authorize`) | Create or update the viewer's report. Returns 201, or 200 if updated. |
| `GET /abuse-reports/cases?status=open&limit&offset` | admin | Cases, newest report first, in a `{data, pagination}` envelope like signup requests |
| `GET /abuse-reports/cases/summary` | admin | `{ open: N }` for the badge |
| `GET /abuse-reports/cases/:id` | admin | The case, its reports (reporter, reason, details, time), earlier cases for the same target, and the hydrated target |
| `POST /abuse-reports/cases/:id/resolve` `{ status, action, note }` | admin | Close as actioned or dismissed |
| `POST /abuse-reports/cases/:id/reopen` | admin | Reopen a closed case |

- **Permissions:** a `permissions/abuse_report.go` in the style of `auth-service/cmd/api/permissions/signup_request.go`: scope check, then role check with `viewer.IsAdminOrSuperAdmin()`.
- **Checks on `POST /abuse-reports`:**
  - The target must exist and the reporter must be allowed to see it. Otherwise return 404, so private nodes can't be probed.
  - Use a graph-grpc visibility check for the viewer. Generalise item 3's `FilterActorsForViewer` into `FilterNodesForViewer(ids)`, the same RPC covering media, comments and tags. Whichever of items 3 and 4b is built first introduces it.
  - The reason must apply to the target type.
  - People can't report themselves or their own content.
  - Rate limit: at most 20 reports per reporter per hour. Move the fixed-window limiter from `auth-service/cmd/api/handlers/rate_limit.go` into `anahita-libs` so both services share it (it needs Redis).
- **Hydrating the target for admins:** `GraphNodeGRPC.Read(id)` (`anahita-libs/data/services/graph_node_grpc.go`). It reads any type and applies no access check, which is right for admins. It also sees disabled and archived nodes, since admins bypass the lifecycle gates.
- **What reporters and reported people see:**
  - The reported person is never told who reported them, and v1 doesn't tell them about reports at all.
  - The reporter only gets the confirmation.
  - Reporter ids are visible to admins only.
- **Events:**
  - Publish `entity.abuse-report-service.report.added.v1` (for the future: webhooks, forwarding to the fediverse).
  - Subscribe to `node.*.deleted.v1` and `node.*.purged.v1`: set `target_gone_at`, and close open cases for that target as `actioned` with the action `deleted`.
  - Events are a hint only (the bus can lose messages), so the case page also re-checks the target when an admin opens it.
- **Admin notifications:** none by email (decided 2026-09-28). The open-cases count feeds the Admin badge from item 4a.

**Admin actions:** the service stays out of other services' business.
- The case page in the web app calls the **existing** endpoints for the actual action:
  - `PATCH /people/:id/enabled` (disable a person) and `POST /people/:id/archive`
  - the same pair for groups
  - `DELETE` on the media, comment, hashtag or location
  - block
- Then it calls `resolve` with the matching `action`.
- Existing permissions still apply to each action; for example archive and purge are super-admin only. The page offers only the actions the viewer is allowed to take.

## Web (anahita-web)
- **Report item:** a new `src/containers/controls/Report.jsx`, a `MenuItem` wrapped in `forwardRef` like `controls/Feature.jsx`.
  - It opens a dialog with a radio list of reasons, fetched per target type and cached per type, plus the description of the chosen reason.
  - It also has an optional details field, required for "Other". The dialog pattern to copy is `containers/auth/SignupRequests/DecisionDialog.jsx`, and the RadioGroup pattern is `containers/actors/Notifications/Form.jsx`.
  - Submitting shows "Thanks, the admins will review this" (`actions.app.alert.success`).
- **Adding it to every node menu**, each of which needs its gate widened so signed-in people who aren't the author or an admin still see the menu:

  | Menu | Change |
  | --- | --- |
  | `containers/actors/Read/Controls.jsx` | Widen `showCommands` in `actors/Read/index.jsx:95` |
  | `containers/media/MediaMenu.jsx` | Stop returning `null` at :53 when Report is available |
  | `containers/feed/Menu.jsx` | Same at :48 |
  | `containers/comments/components/Menu.jsx` | Already shown to signed-in viewers |
  | `containers/hashtags/Menu.jsx` | Admin-only today; show Report to non-admins (also rename the misnamed `LocationMenu` component) |
  | `containers/locations/Menu/index.jsx` | Same |
  | `containers/stories/Menu.jsx` | Check during implementation |

  Hide Report on the viewer's own nodes and when logged out.
- **Admin Reports tab** (item 4a): `containers/admin/Reports/`.
  - **List:** Open, Actioned and Dismissed filters, with each row showing a summary of the target, the report count, the top reasons and the last-reported time. Copy the local-state list from `containers/auth/SignupRequests/index.jsx` and its "remove the row after a decision; refetch on 409" handling.
  - **Case page:**
    - the target, rendered with the type switch from `containers/search/Browse/SearchList.jsx:127-132` (actor card, `MediumNode`, `CommentCard`, plus simple rows for hashtags and locations, which need `isHashtag`/`isLocation` helpers in `utils/node.js`), with a link to it via `getURL`;
    - the list of reports;
    - earlier cases;
    - action buttons, then a resolve dialog with a note.
- **Plumbing:** `src/api/abuseReports.js` registered in `api/index.js:113-147`, and a new `abuseReports` i18n namespace in en-GB and fr-FR.

## Estimate

| Part | Time |
| --- | --- |
| Service skeleton + deploy wiring | 2 h |
| Schema (3 places), reasons loader + default JSON, endpoints, permissions, shared rate limiter, tests | 7–8 h |
| `FilterNodesForViewer` (or extending item 3's RPC) + target checks | 2 h |
| Delete/purge event listener, k8s defaults-path fix | 1–2 h |
| Web: Report control + dialog + widening 6–7 menu gates | 4–5 h |
| Web: Reports list + case page + actions | 5–6 h |
| Docs (`docs/abuse-reports.md`, `docs/configuration.md` section on reasons), i18n | 1–2 h |
| **Total** | **about 22–27 h, about 6 sessions** |

The backend can ship first, testable with `curl`, then the Report menu item, then the admin tab.

## Verification
- **Service tests:**
  - Reporting a node you can't see gives 404, reporting yourself gives 400, and a reason that doesn't apply to the type gives 400.
  - Reporting twice updates the same report.
  - The 21st report in an hour gives 429.
  - A non-admin on `/cases` gets 403.
  - Resolving and then reporting again opens a new case.
- **Reasons config:** editing the mounted `reasons.json` changes the dialog without a restart, and a broken file falls back to the baked copy.
- **Manual end to end:** user A reports B's photo as "Racism" and user C reports it as "Violence".
  - The admin sees one case with 2 reports, and the badge shows 1.
  - The admin deletes the photo from the case page, and the case becomes actioned/deleted.
  - Purging a reported actor closes its open case through the event.

## Decisions (items 4a/4b)
- The admin area is its own item (4a), done before reports.
- Admins are notified by the badge only, no email.
- Only signed-in people can report, with at most 20 reports per hour.

## Dependencies
- 4a before 4b.
- 4b shares `FilterNodesForViewer` with item 3; whichever comes first builds it.
- The shared rate limiter moves into `anahita-libs` here.

## Documentation

A phase is finished only when these pages match the code. anahita-services pages are under its `docs/`, anahita-web pages under its `docs/`. When a page is added, add it to that repo's `docs/README.md` contents too.

- **New services page `abuse-reports.md`**: the `reasons.json` format with an example, how to override it per instance, the case model, and the admin workflow.
- **services `configuration.md`**: `REASONS_PATH` and the report rate limit.
- **services `architecture.md`**: abuse-report-service, `/abuse-reports/`, its events, and the shared rate limiter moved into anahita-libs.
- **services `permissions.md`**: who may report, and who may browse and resolve cases.
- **services `oauth.md`**, *Scopes*: any new scope the endpoints need.
- **web `architecture.md`**: the Report control in node menus, and the admin Reports tab routes.

## What was built (2026-10-03)

**Services**
- **Tables** `abuse_cases` and `abuse_reports` (migration `20261004000002`, mirrored in `init.sql` and `legacy/upgrade.sql`). One open case per target is a rule of the database: a generated column holds the target while the case is open and is uniquely indexed.
- **`abuse-report-service`**, with the endpoints in the table above, in docker-compose, the gateway (`/abuse-reports`), the Makefile and the Kubernetes base.
- **Reasons** (`data/reasons`): the default list is built into the service; `REASONS_PATH` names an installation's own file, re-read when it changes and replaced by the built-in list while it is unusable.
- **`FilterVisible`** in graph-grpc (`NodeService.FilterVisible`, `nodes_visible.go`): which of some nodes, of any kind, a viewer may see, judged by the same gates the listings use. Item 3 can use it as is.
- **`anahita-libs/ratelimit`**: the fixed-window counter moved out of auth-service, which now calls it.
- **Events:** the service closes a case when it hears its target was deleted (posts and comments) or purged (people and groups), and checks again whenever a case is opened.
- **Docs:** `docs/abuse-reports.md`, and additions to configuration, architecture, permissions, oauth and deploying.

**Web**
- **Report** in the menus of profiles, posts, feed items, comments, hashtags and places (`containers/reports`), with a dialog of reasons for the kind of thing reported.
- **Administration › Reports** (`containers/admin/Reports`): cases by status, a case page with the reports, who made them and earlier cases, and dialogs to dismiss, record an action, or reopen. Open cases are counted on the Administration menu and its tab.

**Tests:** the reasons loader; the handlers (reporting, the refusals, the limit, administrators' reads and decisions, deletion); the storage against a real MySQL, including twenty simultaneous reports of one node; `FilterVisible` against a real MySQL for six kinds of viewer; the rate limiter; and, in the web app, who is offered Report, who sees which tab, and how a reported item is described.

**Done differently from the plan:**

- **The case page does not delete or disable anything itself.** It has "Open it", which goes to the reported thing, where Delete, Block and the settings' Danger zone already are, each with its own confirmation and permission. Deleting a post or comment there closes its case by itself; for anything else the administrator records what was done. Building a second set of those controls on the case page was the larger half of the plan's web estimate, and would have duplicated their confirmations.
- **No `abuse-report-defaults/` directory.** The default list is embedded in the binary, so there is one copy. An installation's own file is mounted wherever it likes.
- **No new OAuth scope.** Reporting needs `account:write`; reading and deciding cases need `actors:read` and `actors:write`. A new scope would not have reached clients that were already seeded.
- **Reasons are found by `status = 'open'` uniqueness in the database,** where the plan said "enforced in code".
- **A person's or group's case is not closed when it is deleted, only when it is purged:** deletion has a 30-day wait and can be undone.
- **`entity.abuse-report-service.report.added.v1` is not published.** Nothing listens for it yet.
- **The feeds and graph defaults-path fix** the plan mentions was done with item 9.
- **There is no `stories` menu to add Report to:** it is dead code that item 15 removes.

**Not checked:** no report has been filed through the running stack, because that needs a signed-in session. On the dev stack the service starts, the tables exist, and every route refuses a signed-out caller. The web pages have not been opened in a browser here; lint, 121 tests and the production build pass.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-09-28 |
| Implementation | done 2026-10-03, on branch `m3-admin-moderation` |
| Applied to the dev database | done 2026-10-03 |
