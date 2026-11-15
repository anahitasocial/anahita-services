# Who sees your followers and who you follow (roadmap row 151)

Asked for by the user on 2026-11-15: "people should have standard privacy
settings for who gets to see their followers and followings without having
to make their profiles more private." Today the lists of a profile's
followers and of whom it follows are seen by whoever may see the profile
(`GET /socialgraph/:actor_id/followers/` and `/leaders/`, and on the
fediverse `/users/:uuid/followers` and `/following`). A public profile
shows both to anybody.

## Contents

1. [The setting](#the-setting)
2. [What it limits](#what-it-limits)
3. [Out of scope](#out-of-scope)
4. [Documentation](#documentation)
5. [Phases](#phases)

## The setting

1. **One choice for both lists,** under Settings › Privacy: "Who can see
   your followers and who you follow". The access levels a profile already
   uses ([Permissions](../permissions.md#access-levels)): anyone, anyone
   signed in, followers, people I follow, mutuals, only me. People I follow
   and mutuals for people only, as everywhere.
2. **It never widens.** Somebody refused the profile is refused its lists
   whatever this says.
3. **Default: as the profile,** so that nothing changes until somebody
   chooses. A column on `nodes`, `graph_access`, empty meaning the
   profile's own.
4. **For groups and events too,** by their admins, with the levels that fit
   them (no people I follow, no mutuals).

## What it limits

| Where | Refused |
| --- | --- |
| `GET /socialgraph/:actor_id/followers/`, `/leaders/` | 403, as a list the viewer may not see; the web app shows the counts without opening the dialog |
| The fediverse's `followers` and `following` collections | Their totals only, with no items, as Mastodon does for an account that hides them |
| Search, suggestions and mentions | Unchanged: they never list a profile's followers |

1. **Counts stay,** on the profile and in the collections: a number tells
   nothing of who.
2. **The profile's owner and its admins** always see their own lists.
3. **Recommendations** (who follows whom) keep using the graph, as they
   do for private profiles: what is suggested never names who followed.
   Phase 1 checks that against `recommendation-service`'s reasons ("followed
   by people you follow"), which may name somebody the viewer could not
   otherwise see in a list.

## Out of scope

1. Hiding the counts.
2. One setting for each list.

## Documentation

1. anahita-services `docs/permissions.md` (Following, the access levels),
   `docs/profiles.md`, `docs/federation.md` (the collections), the API
   reference.
2. anahita-web `docs/architecture.md` (Settings › Privacy).
3. Both changelogs.

## Phases

| Phase | What | Status |
| --- | --- | --- |
| 1 | `graph_access` stored, checked by socialgraph-service and activitypub-service; recommendations' reasons checked | ⬜ |
| 2 | Settings › Privacy, the counts without a list, every language base | ⬜ |
| 3 | The docs; tried at both sizes and from a Mastodon account; changelogs | ⬜ |
