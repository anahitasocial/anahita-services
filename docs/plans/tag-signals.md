# Tags as a signal for suggestions and search

*Roadmap row 104, planned 2026-10-20. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

**Status:** planned, not started. After the static www.anahita.io (row 91)
and the banu-devops repository for banu.io (rows 92 and 93), by the user's
order.

## Context

Row 103 built tags: who is in a photo or a video, each tag approved by the
person unless they let anybody tag them ([permissions.md](../permissions.md#people-in-pictures-and-videos)).
Search already finds a photo or a video by the name or handle of somebody
in it. Nothing else reads tags: recommendation-service suggests profiles
from follows, invitations, citations, hashtags and likes, and reads neither
mentions nor tags ([recommendations.md](../recommendations.md)).

The user, 2026-10-20, asked to use approved tags in suggestions and in
search. An approved tag is a strong signal and a consented one: the person
was there, and agreed to be named. Hashtags and follows are cheap to add; a
tag is a sign of people knowing each other.

## Decisions

Settled with the user on 2026-10-20:

1. **Only approved tags,** never one waiting (`meta` `approved`).
2. **Only public posts,** as `shared_interest` reads only public posts: a
   tag in a private group's post, or in a post for followers, never shapes
   what anybody is shown elsewhere.
3. **Never named.** A suggestion says why in general words ("In photos
   with people you follow"); it does not say who, or which photo.
4. **Weights as the others are:** each person's for suggestions (the cog on
   Recommendations), the operator's for search (`search_ranking.json`); 0
   turns it off.
5. **Somebody who lets nobody tag them has no tags,** and so nothing here
   reads anything about them.

## Design

### Suggestions (recommendation-service)

Two reasons, for people only, beside those in
`recommendation-service/data/repositories/signals.go`:

| Reason | Found when | Each | Most |
| --- | --- | --- | --- |
| `pictured_with_you` | They and the viewer are tagged in the same public photo or video, or one of them posted one the other is tagged in | 3 | 6 |
| `pictured_with_leaders` | They are tagged in public photos or videos with people the viewer follows, or in ones those people posted. `ids` are those people | 2 | 8 |

- Both are what the viewer did or was part of, so they are read only when
  the viewer lets their activity be used (`use_activity`), as
  `you_engaged` and `shared_interest` are.
- New weights in `recommendation-service/constants/constants.go`
  (`Weights`), offered in each person's weights dialog with the rest.
- The words under a suggestion, in all five bases: "In photos with you",
  "In photos with people you follow".

### Search (graph-grpc-service, `data/repositories/search.go`)

Already: a photo or a video is found, with a hashtag's points, by the name
or handle of somebody in it.

New, in its standing (`Standing`), when the viewer is signed in:

1. **A post with people the viewer follows in it** earns part of the
   `following` bonus, as a post on a profile the viewer follows does.
2. **A person who appears in public photos with people the viewer follows**
   earns part of it too, when profiles are searched.

One new weight in `graph-grpc-defaults/search_ranking.json`,
`pictured_with_followed` (proposed 8, beside `following` 15), and in
`ranking.Weights`. "Things from people you follow first"
(`followed_first`) counts both as followed.

### Not in this row

- Custom feeds filtering by who is in a post.
- Somebody on another server tagged here (row 103 left it for later).

## Open, to settle when the row begins

1. The two weights: 3 and 2 for suggestions, 8 for search, as proposed?
2. Whether being tagged by somebody (`pictured_with_you`, one posted the
   other's photo) counts the same as being tagged together.

## Docs to change

`docs/recommendations.md` (the reasons table, what each person chooses),
`docs/search.md` (the points and the standing), `docs/permissions.md`
(where tags show), the API reference; anahita-web `docs/architecture.md`
(the weights dialog, the words under a suggestion).
