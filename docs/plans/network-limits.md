# Posts that fit every network

*Roadmap row 100, planned 2026-10-19. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

**Status:** part 1 done (2026-10-19); parts 2 and 3 wait for the picture
limit from the user. Branch `issues/network-limits` in both
repositories.

## Context

A post here can be longer, and hold more, than other networks take. What
each takes, and what happens now:

| | Here | Fediverse (Mastodon) | Bluesky |
| --- | --- | --- | --- |
| Letters in a note | 8000 | the whole Note is sent; Mastodon shows it all | 300 (`bsky.PostMax`) |
| Pictures in a post | 4 (`constants.PhotoMaxFiles`) | all are sent; Mastodon shows 4 | 4 (`bsky.ImagesMax`) |
| A note cut for being long | — | never cut | cut at 299 letters with "…" |

**The gaps** (found 2026-10-19, `atproto-service/cmd/api/handlers/out.go`):

1. A note cut for Bluesky gets a card that links to it here only when it had
   no embed already (`post.Cut && post.Embed == nil`). One with pictures or a
   video is cut with no way to the rest. (A note with a link's card is not
   affected: the cut is looked at first, and its own card wins.)
2. Nothing tells the writer. The composer does not say that a note going to
   Bluesky will be cut at 300 letters.

The user, 2026-10-19: fix those gaps; and raise the pictures in a post above
four, telling the writer that "only the first 4 photos will go out with a
link back to the original post", rather than splitting one post into several
on other networks. Pixelfed and Misskey show every picture of a post.

## Decisions

Settled with the user on 2026-10-19:

1. **A fediverse post is one object for every server**, so it cannot be
   split for one server and not another. It carries every picture, each with
   its description; what a server does not show, it can link to.
2. **Never a post cut with no way to the rest.** Wherever a post going out is
   cut, or carries fewer pictures than it has, a link to it here goes with it.
3. **The composer says so, once, in a line**, only for a writer whose posts go
   out (`federation` on for that profile), and only when it applies: more
   than 300 letters for Bluesky, more than four pictures for either.
4. **More pictures per post:** the limit to be set by the user. Proposed: 20,
   as Instagram and Pixelfed. **Open.**

## Design

### 1. Bluesky: a link to the rest (atproto-service) — done 2026-10-19

1. A post cut, with pictures or a video: Bluesky takes those or a card, not
   both. The pictures stay, and the text ends with the post's address here
   on a line of its own, as a link facet (`bsky.OutPost.EndWithLink`), cut
   shorter to make room. The address needs no words in any language.
2. Tests: `TestAPostCutEndsWithALinkToTheRest` (bsky) and
   `TestAPhotoPostCutLinksToTheRest` (handlers).

### 2. More pictures in a post (anahita-libs, photo-service, web)

1. `constants.PhotoMaxFiles` to the new limit; NodeInfo's `photoMaxFiles`
   says it, and the web app reads it already (`src/utils/photoFiles.js`).
2. Check what assumes four: the stepper and grid in the web app, the
   theatre, the edit dialog, Download, the fediverse `attachment` list, the
   OpenGraph image, the `photo_files` reads and anything that pages them.
3. Bluesky: the first four, in order, and the link to the post in the text,
   "See all 15 photos" (a facet).
4. Fediverse: every picture in `attachment`, and the same "See all 15
   photos" link at the end of the content when there are more than four.

### 3. The composer says so (web)

1. Under the text, when the profile federates to Bluesky and the text is
   over 300 letters: "Bluesky shows the first 300 characters, with a link
   to the rest here."
2. Under the pictures, when there are more than four and the profile
   federates: "Mastodon and Bluesky show the first 4 photos, with a link to
   see all of them here."
3. A reserved line, so nothing jumps; every string in every base.

## Checking

1. Post from a federating test profile: a long note with pictures, a long
   note with a link, 15 pictures. Read what went out (`out.go` logs, the PDS,
   the outbox JSON) and the rows in `photo_files`.
2. The composer at desktop and phone sizes, with and without federation.

## Docs to change

`docs/federation.md`, `docs/websites.md` (any limits named),
`docs/api.md` and `docs/api/photo-service.json` (the photo limit),
anahita-web `docs/` where the composer is described.
