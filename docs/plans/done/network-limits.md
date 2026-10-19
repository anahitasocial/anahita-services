# Posts that fit every network

*Roadmap row 100, planned 2026-10-19. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

**Status:** done 2026-10-19, all three parts. Branch `issues/network-limits`
in both repositories.

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
4. **Twenty pictures per post** (the user, 2026-10-19), as Instagram and
   Pixelfed; **fewer where an installation says so** in `PHOTO_MAX_FILES`
   (the user, the same day: "some servers may want to cap the file
   uploads"). Never more than twenty.
5. **The link back is the post's address,** shown without its scheme, on a
   line of its own: it needs no words in any language.

## Design

### 1. Bluesky: a link to the rest (atproto-service) — done 2026-10-19

1. A post cut, with pictures or a video: Bluesky takes those or a card, not
   both. The pictures stay, and the text ends with the post's address here
   on a line of its own, as a link facet (`bsky.OutPost.EndWithLink`), cut
   shorter to make room. The address needs no words in any language.
2. Tests: `TestAPostCutEndsWithALinkToTheRest` (bsky) and
   `TestAPhotoPostCutLinksToTheRest` (handlers).

### 2. More pictures in a post — done 2026-10-19

1. `constants.PhotoMaxFiles()` reads `PHOTO_MAX_FILES`: 1 to 20, 20 unless
   said (`PhotoMaxFilesCeiling`). Read by auth-service (NodeInfo's
   `photoMaxFiles`, which the web app reads already), photo-service, and the
   two federation services, which keep no more pictures of a post from
   elsewhere (`federation.PostPicturesMax()`). The address that gives a
   picture to other servers takes positions up to the ceiling, so a post
   made before the limit was lowered keeps its pictures.
2. The web app needed no change: the composer, the edit dialog and replies
   read the limit from NodeInfo, and the slides' dots fit twenty on a phone.
3. Bluesky: the first four, in order, and the post's address at the end of
   its words (`withPictures` says there are more).
4. Fediverse: every picture in `attachment`, and when there are more than
   four (`ap.PicturesShownMax`), a last paragraph that links to the post
   here, in `content` and every `contentMap` language.
5. Tests: `TestHowManyImagesAPostHolds`,
   `TestAPhotoPostWithMorePicturesThanBlueskyTakes`,
   `TestAPhotoPostWithMorePicturesThanMastodonShows`.

### 3. The composer says so (web) — done 2026-10-19

1. Under the text, when the viewer's Bluesky account is connected and the
   post is public on their own profile, past 300 characters: "Bluesky shows
   the first 300 characters, with a link to the rest here." Its line is kept
   (empty) while the post goes to Bluesky, so nothing moves.
2. Under the pictures, past four, when the post goes to the fediverse,
   Bluesky or both: "Mastodon and Bluesky show the first 4 photos, with a
   link to all of them here" (or one of the two).
3. `constants/limits.js` holds `BLUESKY_POST_MAX` and `PICTURES_SHOWN_MAX`;
   the strings are in every base.
4. Checked in headless Chrome with both networks' settings answered as on:
   both lines at desktop size, the Bluesky line on a phone.

## Checking

1. Post from a federating test profile: a long note with pictures, a long
   note with a link, 15 pictures. Read what went out (`out.go` logs, the PDS,
   the outbox JSON) and the rows in `photo_files`.
2. The composer at desktop and phone sizes, with and without federation.

## Docs to change

`docs/federation.md`, `docs/websites.md` (any limits named),
`docs/api.md` and `docs/api/photo-service.json` (the photo limit),
anahita-web `docs/` where the composer is described.
