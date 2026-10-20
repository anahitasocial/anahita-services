# Tagging people in pictures and videos

*Roadmap row 103, planned 2026-10-19. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

**Status:** started 2026-10-19, on branch `issues/people-in-media` in both
repositories. Done 2026-10-19, every part.

**Parts, in order:**

1. The tag and the choice — done 2026-10-19: `edge.tag.person.v1` from the
   person to the post, `meta` `pending` or `approved`
   (`anahita-libs/constants/person_tags.go`); "Who can tag you?", the `tag`
   permission (`registered`, `approve` the default, `admins`);
   `permissions.TagDecision`. Only a person here may be tagged: somebody on
   another server cannot approve it here.
2. The endpoints — done 2026-10-19, in actor-service:
   `GET /people/tagged-in/:post_id`, `POST|PUT|DELETE
   /people/:id/tags/:post_id`, and the events `edge.tag.*.requested|added|
   deleted.v1`. Tried as the administrator tagging Worf and Dax in a photo.
   Found on the way: the graph's edge edit called itself for ever (no edge
   had ever been edited), and three handlers carried on after writing a
   refusal.
3. Notifications — done 2026-10-19: `actor_tag_request` with
   `answer_tag` (Accept and Decline) while it waits, `actor_tag` for one
   made at once, `actor_tag_accept` to the author; all under the `mentions`
   choice, and all taken back with the tag. Tried with Dax.
4. Where tags go — done 2026-10-19: the `tagged` filter of a profile's
   feed; search finds a photo or a video by the name or handle of somebody
   in it (the hashtag's points); the fediverse, each person who federates
   as a `Mention`, the post sent again when a tag is approved or taken off.
   Recommendations read no mentions, and read no tags either.
5. The web app — done 2026-10-19: "Who can tag you in photos and
   videos?" in Settings › Permissions (and "Nobody" for mentions' narrowest
   choice); "With Worf and Dax" under a photo or a video on its page and in
   the theatre, Accept and Decline or Remove me there; Tag people in the
   composer and in a post's menu; the Tagged tab; the notifications, with
   Accept and Decline. Profiles are now found by handle in lists too.

**Left for later:** a spot on the picture; tagging somebody on another
server, who cannot approve it here; tags in the composer for a video, which
is a post only once its upload is done (tagged from its menu after); names
on feed cards.

## Context

The user, 2026-10-19: tag people in pictures and videos. It is not a
mention: a mention is a name in the words, saying "this is for you"; a tag
says "this person is in it". Tags feed search and recommendations, and
people can choose not to be tagged, or to approve each tag first.

Mentions already have "Who can mention you?" (`features.js`), whose
narrowest choice is `admins`. When nobody may mention someone, no edge is
made between them and the post (`actor-service/data/services/mentions.go`).

## Decisions

Settled with the user on 2026-10-19:

1. **A tag is an edge** between the post and the person, as a mention is,
   of its own kind; search and recommendations read it.
2. **Each person chooses:** anybody may tag me, tags wait for my approval,
   or nobody may. A tag waiting is shown to nobody but the person tagged and
   the post's author, and makes no edge until approved.
3. **"Who can mention you?"**: its narrowest choice is labelled **Nobody**
   (it is `admins` in the code), in every base.

Settled with the user the same day, when the row began:

4. **A list of who is in it,** not a spot on the picture: "Tag people"
   picks profiles, and their names show under the picture or the video. A
   spot can come later.
5. **Approved tags go to the fediverse** as a Mention in the post's `tag`,
   so the person's server hears of it; nothing to Bluesky, which has no
   tags.
6. **Everybody approves each tag first** until they choose otherwise.
7. **A Tagged tab** on a person's profile, after Posts: the posts they are
   approved in.

## Design

1. **Data:** the tag edge, with `pending` until approved; the person's
   choice beside "Who can mention you?".
2. **Services:** add and remove tags on a photo or video post (its author);
   approve or decline (the tagged); a notification of each, with Accept and
   Decline (row 97's actions); untag oneself any time. A tagged person's
   Tagged tab, or a filter, is to be decided with the web design.
3. **Search and recommendations** read approved tags as they read mentions.
4. **Web:** "Tag people" in the photo and video composer and edit dialog;
   the people in it under the picture; the choice in a person's settings.

## Docs to change

`docs/api.md` and the reference, `docs/notifications.md`, `docs/federation.md`,
anahita-web `docs/` for posts and settings.
