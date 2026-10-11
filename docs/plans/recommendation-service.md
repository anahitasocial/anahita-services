# Actor recommendation service (no AI)

*Roadmap item 3, planned 2026-09-28. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

## Context
We want "who to follow" suggestions for people and groups, based on what people pick during onboarding and how they interact with other nodes, plus a page where people tune their preferences. No models: weighted SQL signals with an explanation for every suggestion. It should be its own service behind a small HTTP contract, so other instances can swap in their own.

At a few hundred actors the whole graph is tiny: thousands of edges, not millions. **Scoring on each request with plain SQL, plus a short cache, is enough.** There's no need for precomputation, cron jobs or embeddings. Precomputation can come later without changing the contract.

## What exists today
- **No recommender.** The only "suggestion" is the admin-curated featured list (`nodes.featured_at`, `GET /people/?featured=1`, `GET /groups/?featured=1`). Onboarding uses it in `anahita-web/src/containers/onboarding/featured.js:56-72`.
- **Onboarding collects:** avatar, name, bio, pronouns, featured follows, `auth_accounts.invited_by` and `onboarded_at`. It collects **no interests, language or location**. The `nodes.language`, `timezone` and `geo_*` columns exist but nothing writes them.
- **Useful signals already in the DB:**

  | Signal | Where |
  | --- | --- |
  | Follows | `edge.actor.follow.v1`; group membership is also a follow |
  | Likes | `edge.base.voteup.v1` |
  | Subscriptions | `edge.base.subscription.v1` |
  | Mentions | `edge.tag.mention.v1` |
  | Hashtags and locations tagged on content | `edge.tag.hashtag.v1`, `edge.tag.location.v1` |
  | Comments and reposts | nodes with `created_by` and `parent_id` |
  | Inviter | `auth_accounts.invited_by` |
  | Activity | `last_visited_at`, `follower_count` |
  | Location | `geo_*`, with a Haversine helper at `graph-grpc-service/data/repositories/nodes.go:233` |

- **Reusable graph code:** `BrowseOrderedByDegree` and `Trending` (`graph-grpc-service/data/repositories/graph.go:209,291`), plus the batch relation lookup (`socialgraph_rel.go:47-224`, exposed as `SocialgraphGRPC.Browse(ids, viewerID)`) for dropping people the viewer already follows or has blocked.
- **Privacy gotcha:** most graph read paths don't apply the lifecycle rules. `SetLifecycleQuery` and `SetArchiveQuery` (`lifecycle.go:31-76`) are used only by search and featured, and `BatchRead` (`nodes_read.go:70-90`) applies no access gate at all. Unverified email lives in `auth_accounts.email_verified_at`, which the graph never checks. The recommender must not rely on those paths to filter.
- **Service template:** `hashtag-service` (Echo, `an_mw.SetViewer`, gRPC clients from `anahita-libs/data/services`, dockerfile, compose, nginx `location`, k8s manifest, Makefile targets). The events template is `notification-service/cmd/api/event_listener.go`, on RabbitMQ exchange `anahita_events`.

## Design

**1. The contract (what a replacement service must implement)**, documented in `anahita-services/docs/recommendations.md`:
- `GET /recommendations/actors?kind=person|group&limit=20&context=onboarding|dashboard`
  - Returns `{ items: [{ actor_id, score, reasons: [{ type, ids? }] }] }`.
  - Reason types: `followed_by_leaders` (ids of up to 3 leaders), `follows_you`, `shared_interest` (hashtag or location ids), `you_engaged` (you liked or commented on their posts), `inviter`, `nearby`, `popular`, `featured`.
- `POST /recommendations/dismissals` `{ actor_id }` and `DELETE /recommendations/dismissals/:actor_id`: "not interested".
- `GET` and `PUT /recommendations/preferences`: `{ suggest_people, suggest_groups, use_activity, use_location, discoverable }`.
- The response is a **skeleton** (ids + reasons), the same model as `feed-generators.md`. The web app hydrates the actors through the existing actor endpoints, so a replacement never widens visibility.
- The web app treats 404 or 503 from `/recommendations/` as "no recommender installed" and hides every suggestion surface.

**2. New `recommendation-service` (Go)**
- Copy the `hashtag-service` layout: `cmd/api/{main,routes}.go`, handlers, requests, responses, `config/` (`DSN`, `GRAPH_SERVICE_ADDR`, `REDIS_ADDR`, `AMQP_ADDR`, OAuth env), `go.work` entry, `dockerfiles/recommendation-service.dockerfile`, compose entry, nginx upstream + `location /recommendations/`, `k8s/NN-recommendation.yml` + ingress rule, Makefile targets.
- Every route requires a logged-in viewer (`an_mw.Authorize`); suggestions only make sense for a logged-in person.
- It reads the DB **read-only** for signals (one SQL query per signal, each `LIMIT 200`) and writes only its own tables.

**3. Scoring.** For each candidate, add up weighted signals, then multiply by an activity factor. Weights live in config, so an instance can tune them without code.

| Signal | How | Default weight |
| --- | --- | --- |
| Follows you (not followed back) | follow edge candidate → viewer | 4 |
| Followed by your leaders | count of the viewer's leaders who follow the candidate (friends of friends); for groups, groups your leaders belong to | 3 per leader, max 5 |
| Inviter or invitee | `auth_accounts.invited_by` in either direction | 3 |
| Shared interest | viewer's interest edges ∩ hashtags and locations on the candidate's posts from the last 12 months (and group topics) | 2 per match |
| You engaged | the viewer liked, commented on or mentioned the candidate's content in the last 90 days | 2 per interaction, max 4 |
| Nearby | both have `geo_*` and are within 50 km, only if `use_location` | 1 |
| Popular fallback | `Trending` follow edges in the last 30 days, then featured | 0.5 |

- **Activity factor:** 1.0 if the candidate posted or visited in the last 90 days, 0.5 up to 12 months, 0 after that.
- **Quality bar** (from `done/onboarding.md:201-208`): must have an avatar or a bio.
- **Cold start:** `context=onboarding` uses only signals that need no follows (inviter, shared interest, nearby, popular or featured). These are the cold-start rules already in `done/onboarding.md`.

**4. Exclusions.** Every candidate is dropped if it is any of the following:
- the viewer; people and groups they already follow; pending follow requests;
- blocks in either direction (reuse `relNotBlockedEitherWay` logic from `access.go:187-209`);
- actors that are disabled, archived or deleted;
- people whose email is unverified;
- actors whose access the viewer doesn't pass (`SetActorAccessQuery` rules);
- anyone the viewer has dismissed;
- people with `discoverable = false`.

Who does what:
- **The recommender's own SQL** handles the cheap relationship checks: the viewer, existing follows and follow requests, dismissals, and `discoverable`.
- **A new graph-grpc RPC, `FilterActorsForViewer(ids) → ids`** (build it generically as `FilterNodesForViewer`, since item 4b needs it for media, comments and tags too), handles the privacy rules: actor access, lifecycle, archive, blocks in either direction and email verification. The recommender calls it on its top ~100 candidates before the final cut.

That keeps the privacy rules in one place, so a replacement recommender gets them for free. The web app's hydration through the actor endpoints is a second line of defence.

**5. Storage**
- **Interests:** new graph edge `edge.actor.interest.v1` (person → hashtag or location), written by new graph-grpc and actor-service endpoints `PUT /people/:id/interests` and `GET /people/:id/interests`. Add the constant to `anahita-libs/constants/entity_types.go`. Feeds and a future "follow a hashtag" feed can reuse these edges.
- **The recommender's own tables** (goose migration plus a matching addition to `legacy/upgrade.sql`, per the migration catch noted in item 1):
  - `recommendation_preferences (person_id PK, suggest_people, suggest_groups, use_activity, use_location, discoverable, updated_at)`. Default when there's no row: all true, i.e. discoverable by default with opt-out.
  - `recommendation_dismissals (person_id, actor_id, created_at, PK(person_id, actor_id))`.
- **Cache:** results per viewer and kind in Redis for 10 min. Clear it on `edge.follow.*`, `edge.block.*` and actor archive/disable/delete events from RabbitMQ. Events are a hint only; lost ones just wait for the TTL.

**6. Web (anahita-web)**
- `src/api/recommendations.js` plus redux actions and reducer, following the `api/onboarding.js` and `featured.js` pattern. Hydrate actors with the existing batch actor fetch.
- **Dashboard "Who to follow" card:** 3–5 actors with a one-line reason ("Followed by Ana and 2 others", "Posts about #permaculture"), a Follow button (reuse `Follow.jsx`) and a dismiss ✕. Put it in `src/containers/Dashboard.jsx:30-44` (a new card, or an `md:4` side column). Link to a full list.
- **Suggestions page:** `/suggestions`, with tabs for People and Groups, a longer list and the same actions.
- **Preferences:** a new settings section, "Recommendations", in `src/containers/actors/Settings/sections.js` (`SECTIONS` / `ITEMS` / `ALL_SECTIONS`, `viewerOnly: true`), with its component mapped in `Settings/index.jsx:197-223`. It contains:
  - interest chips (hashtag and location autocomplete from the hashtag and location services);
  - toggles: suggest people, suggest groups, use my activity, use my location;
  - "Show me in other people's suggestions";
  - a list of dismissed actors with Undo.
- **Onboarding phase 3** (already planned in `docs/plans/done/onboarding.md`): an "Interests" step (the same chip picker), then a "Suggested for you" step (`context=onboarding`), next to the Featured step. Both are skipped when empty, following the existing rule.
- i18n for en-GB and fr-FR.

## Build order inside the item

| Step | Part | Estimate |
| --- | --- | --- |
| 1 | Interest edges + endpoints (graph-grpc, actor-service, anahita-libs) | 2–3 h |
| 2 | `FilterActorsForViewer` RPC + tests for every exclusion | 2–3 h |
| 3 | Service skeleton + deploy wiring (compose, nginx, k8s, Makefile) | 2 h |
| 4 | Signal SQL, scoring, reasons, preferences and dismissals tables + endpoints, cache | 6–8 h |
| 5 | Web: API/redux, dashboard card, suggestions page | 4–5 h |
| 6 | Web: Recommendations settings section (interests, toggles, dismissals) | 3–4 h |
| 7 | Web: onboarding Interests + Suggested steps | 3 h |
| 8 | Contract doc (`docs/recommendations.md`) + update `done/onboarding.md` | 1 h |
| | **Total** | **about 23–29 h, about 6 sessions** |

Steps 1–4 are the backend (about 3 sessions) and can ship and be tested with `curl` before any UI.

## What was built: 2026-10-11

Row 20, on branch `m7-discovery`. What the service does is in `docs/recommendations.md`; this is what was done and where it differs from the sections above.

**Built**

- **The service** (`recommendation-service`, with its dockerfile, compose entry, nginx location, Kubernetes manifest and Makefile targets), its two tables (migration `20261020000001_recommendations`, `schema/init.sql`, `legacy/upgrade.sql`), and the endpoints: suggestions, preferences, dismissals.
- **Seven signals** in SQL, each ending in one clause that keeps it to profiles that may be suggested, and a ranking that reads nothing.
- **Visibility asked of the graph:** `FilterVisible`, which item 4b built, on the best hundred and on what the reasons name.
- **Tests:** the signals and the two stores against a real database (every exclusion in the list, each reason, groups, somebody who has just joined, activity switched off); the ranking; the endpoints over fakes.
- **Web:** see the web app's `docs/architecture.md`.

**Different from the plan above**

- **Interests are taken from what somebody does,** the hashtags on what they wrote and liked, and not picked by hand. No `edge.actor.interest.v1`, no interests endpoints, no picker. `use_activity` switches it off.
- **The answer carries the profiles,** as `actor`, and what the reasons name, as `related`, each gated by the graph. The plan had ids alone, with the web app reading the profiles; the installation has no way to read several profiles by id over HTTP, and the gate is the graph's either way.
- **A dormant profile counts for a tenth, not nothing.** The development data is an installation brought over from an older one, where nobody has been active for a year: with the plan's rule nobody at all was suggested.
- **A profile's making counts as activity,** so a group started yesterday is alive.
- **Weights are constants,** not settings.
- **No cache, and so nothing listens for follows and blocks.** The whole answer takes under a tenth of a second on the development data.
- **No `nearby` and no `use_location`.**
- **Onboarding is not changed.** `context=onboarding` is answered, and nothing asks it yet.
- **`GET /recommendations/dismissals`** was added, for the list in settings.

**Added the same day, at the user's asking**

- **Events are suggested,** with a reason of their own: a group the viewer follows is hosting it.
- **Each person can set their own weights,** from 0 to 10 for each reason, kept with their choices. The plan had weights as the installation's configuration; these are each person's, as the custom feeds plan says a ranked feed's would be.

## Verification
- **Seed data:** an integration test DB in the style of `access_integration_test.go`, with people A–F.
  - A follows B and C; B and C follow D, so D should come first for A with the reason `followed_by_leaders [B, C]`.
  - E blocked A, so E never appears.
  - F is archived, so F never appears.
  - One person with unverified email never appears either.
- **Preferences:** dismissing D removes D, and undoing brings D back. With `discoverable=false`, the person disappears from everyone's results. With `use_location=false`, `nearby` reasons disappear.
- **Swappability:** stop the container, and the dashboard card, the suggestions page and the onboarding step all disappear without errors.
- **Manual:** a new account picks interests in onboarding and sees suggestions whose reasons match those picks.

## Decisions (item 3)
- Interests are graph edges (`edge.actor.interest.v1`), not recommender-owned tables.
- People are discoverable by default with an opt-out toggle. Restricted profiles are excluded by the access rules anyway.

## Dependencies
- Items 0 and 1 first: the access gate should be sound before the recommender builds on it.
- Onboarding phase 3 (step 7) depends on steps 1–5.
- The later feed-generators work can reuse the interest edges.

## Documentation

A phase is finished only when these pages match the code. anahita-services pages are under its `docs/`, anahita-web pages under its `docs/`. When a page is added, add it to that repo's `docs/README.md` contents too.

- **New services page `recommendations.md`**: the HTTP contract a replacement must implement (skeleton plus reasons), the signals and their weights, the exclusions, and how to swap or remove the service.
- **services `configuration.md`**: a *Recommendations* section (weights, cache TTL, service address).
- **services `architecture.md`**, *Services* and *Routing*: recommendation-service, `/recommendations/`, and the new `FilterNodesForViewer` RPC under graph-grpc.
- **services `permissions.md`**: the "Show me in other people's suggestions" opt-out.
- **web `architecture.md`**, *Routes*: `/suggestions`; *State*: the recommendations API.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-09-28 |
| Implementation | not started |
