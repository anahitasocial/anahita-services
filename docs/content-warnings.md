# Content warnings

[Documentation](README.md) › Content warnings

A post can carry a content warning: what it holds, said by its author, so
that a reader sees the warning first and chooses whether to see the post.
Nothing is removed for being sensitive; the reader is asked. This page covers
the kinds, what is stored and sent, and what the web app does.

The routes are in the API reference: [text-service](api/index.html#text-service), [photo-service](api/index.html#photo-service) and [video-service](api/index.html#video-service).
The plan, with what is still to come, is
[Content warnings](plans/content-warnings.md) (roadmap row 122).

## Contents

1. [The kinds](#the-kinds)
2. [On a post](#on-a-post)
3. [A warning the administrators gave](#a-warning-the-administrators-gave)
4. [Where it is shown](#where-it-is-shown)
5. [What the web app does](#what-the-web-app-does)
6. [Where the code is](#where-the-code-is)
7. [What is not built](#what-is-not-built)

## The kinds

Bluesky's four labels, so that they can cross one for one, and a fifth for
a warning in the author's own words, as the fediverse writes them.

| `warning` | Said to people | For |
| --- | --- | --- |
| `adult` | Adult content | Explicit sexual images |
| `suggestive` | Sexually suggestive | Sexual, without nudity |
| `graphic` | Graphic media | Violence, gore or disturbing images |
| `nudity` | Non-sexual nudity | Artistic nudes, for example |
| `other` | The author's words, up to 100 characters | Spoilers, grief, anything else |

The kinds are in `anahita-libs/data/entities/warning.go`.

## On a post

Any post can have one: a note, a photo, a video or a sound, an article, a
topic, and a reply of any kind.

1. **Made with one:** `warning` and, for `other`, `warning_text`, on
   `POST /notes/:owner_id/`, `/photos/:owner_id/`, `/articles/:owner_id/`,
   `/topics/:owner_id/`, `POST /videos/:owner_id/uploads` (a video or a
   sound, kept with the upload until the post is made), and every reply:
   `POST /notes/:id/replies`, `/photos/:id/replies`, and a video upload
   with `parent_id`.
2. **A thread** takes one warning, on `POST /notes/:owner_id/thread` or
   `/notes/:id/replies/thread`, and every note of it has it.
3. **Changed, or taken off:** `PATCH /notes/:id/warning`, and the same under
   `/photos`, `/videos`, `/articles` and `/topics`, with `warning` and
   `warning_text`; an empty `warning` takes it off. Whoever may edit the post
   may; a reply, its author alone.
4. **Refused** with 400 `warning_invalid`: a kind that is not one, or `other`
   with no words or more than 100 characters. Words sent with another kind
   are dropped.
5. **Sent with every post,** in lists too, and with a post shown inside
   another (what a repost, a quote or a reply shows):
   `warning: { kind, text, by_moderator }`, or no `warning` at all. A
   person's download keeps it, as `warning` and `warning_text` on each post
   in `posts.ndjson` ([Download your data](data-export.md)).

It is stored on `nodes`: `warning` (`varchar(16)`, empty for none),
`warning_text` (`varchar(100)`) and `warning_by_moderator` (`tinyint(1)`),
migration `20261122000017_content_warnings`; and on `video_uploads`,
`warning` and `warning_text`, for a video still being sent.

## A warning the administrators gave

`warning_by_moderator` says that the site's administrators gave a post its
warning. Its author cannot change it or take it off: they are answered 403
`warning_not_allowed`, unless they ask for the warning it already has. The
web app does not offer the choice on such a post.

Nothing sets it yet: the report reason, the case's action and the post's
menu for administrators are phase 4 of the plan.

## Where it is shown

1. **Everywhere a post is drawn** in the web app (below).
2. **A link pasted elsewhere** shows nothing of the post: web-shell gives no
   picture of it (the site's own is used), and the warning in place of its
   words, "Content warning: graphic media" or "Content warning: " and the
   author's words ([The web shell](web-shell.md)).
3. **Other servers** are not told yet: a post goes to the fediverse and
   Bluesky as it did, without its warning, and a warning from there is not
   read. That is phase 3 of the plan.

## What the web app does

1. **The composer**, short and long, has **Content warning** among a post's
   choices: on the short composer's Details page a row, saying the kind
   chosen or None; in the long composer an icon beside the audience, with
   the kind beside it once chosen. It opens a dialog of None and the five
   kinds, each with its line; Other asks for the words right under it. It
   is asked of each post: the next starts with none.
2. **The reply form** has the same, an icon among its pictures and GIF.
3. **A post's menu**, on its own page, has **Content warning** for whoever
   may edit it, and a reply's menu for its writer; not on a warning the
   administrators gave.
4. **A post with one** shows the warning with its kind's icon and
   **View** over its photo, GIF, video or sound, drawn blurred behind it so
   that the card keeps its height and nothing jumps, and its words as they
   are, as Bluesky and Mastodon show a post marked sensitive (the user,
   2026-11-09); a link's card under them is blurred too. A post of words
   alone, and an article or a topic, whose words may hold pictures, have
   the warning over all of it. View opens it for that showing of the page.
   The same in feeds, on profiles, on a post's own page, in replies,
   quotes, reposts and posts shown in an article; in the photo theatre, a
   veil over the picture, each post behind its own as one steps through
   them, and the warning for its title.
5. **A profile's Photos card** leaves out a photo with a warning: the card
   is a glance at who somebody is.

Everybody sees every kind behind its warning; each reader's own choice of
Show, Warn or Hide is phase 2.

## On the fediverse and Bluesky

A post's warning goes with it to both networks, as it is shown here:

| Network | A post with a photo, a GIF, a video or a sound | A post of words alone |
| --- | --- | --- |
| The fediverse | `sensitive: true`: its pictures blurred, its words shown | `sensitive: true`, and the warning as its `summary`, which Mastodon shows to be clicked past |
| Bluesky | The self-label of its kind, in the record's `labels`: its pictures blurred for those who asked to be warned | The same label, which hides nothing there: Bluesky warns of pictures |

The warning's words are its author's for Other, and its kind's name in
English for the rest, "Graphic media" for example: one post goes to every
server. An article keeps its opening as its summary, and is sent
`sensitive`.

Bluesky's labels are its four kinds, one for one: `adult` is `porn`,
`suggestive` is `sexual`, `graphic` is `graphic-media`, and `nudity` is
`nudity`. Other has none: a warning in one's own words is not something
Bluesky has, and a post with one goes there without a warning.

A warning given or changed after a post was sent goes to the fediverse as
an `Update`. Bluesky has no changing a post: there, a post keeps the
warning it was sent with.

Warnings from other servers are not read yet: a post from elsewhere marked
sensitive is still kept without its pictures ([Federation](federation.md)).

## Where the code is

| What | Where |
| --- | --- |
| The kinds, and checking what a request says | `anahita-libs/data/entities/warning.go` (`NormalizeWarning`) |
| Setting it, 400 `warning_invalid`, 403 `warning_not_allowed` | `anahita-libs/handlers/warning.go` (`SetWarning`), tested in `warning_test.go` |
| Sent with a post | `anahita-libs/responses/warning.go`, used in `medium.go`, `parent.go`, `node.go` |
| `PATCH /…/:id/warning` | `text-service/cmd/api/handlers/medium_warning.go`, `photo-service/…/photo_warning.go`, `video-service/…/video_warning.go` |
| Where it is stored | table `nodes`; `graph-grpc-service/data/repositories/nodes_add.go`, `nodes_edit.go`, `helpers.go`; `node.proto` fields 101 to 103 |
| A video's, until it is made | `video-service/data/repositories/uploads.go`, `video-service/worker/worker.go` |
| In a person's download | `import-export-service/archive/archive.go` |
| A link pasted elsewhere | `web-shell/shell/pages.go` (`warningDescription`) |
| Sent to the fediverse | `activitypub-service/ap/note.go` (`WithWarning`), its words from `entities.WarningSaid` |
| Sent to Bluesky | `atproto-service/bsky/outgoing.go` (`WithLabel`), its label from `entities.WarningLabel` |
| The web app | `src/utils/warnings.js`, `src/components/ContentWarningDialog.jsx`, `ContentWarningButton.jsx`, `WarningCover.jsx`, `src/containers/controls/medium/ContentWarning.jsx` (anahita-web) |

## What is not built

The rest of the plan, each a phase of it:

1. **Each reader's choices** of Show, Warn or Hide for each kind, in a
   Content tab of their settings.
2. **Warnings from elsewhere:** `sensitive` and `summary` read from the
   fediverse, self-labels from Bluesky, and their pictures kept and
   blurred, not left out as they are now ([Federation](federation.md)).
3. **The administrators' warning:** a report reason, the case's action, the
   post's menu, and a notification to the author.
4. **Made with AI**, a sixth kind, chosen for an author when the file says
   so.
