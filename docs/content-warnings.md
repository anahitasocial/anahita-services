# Content warnings

[Documentation](README.md) › Content warnings

A post can carry a content warning: what it holds, said by its author, so
that a reader sees the warning first and chooses whether to see the post.
Nothing is removed for being sensitive; the reader is asked. This page covers
the kinds, what is stored and sent, and what the web app does.

The routes are in the API reference: [text-service](api/index.html#text-service), [photo-service](api/index.html#photo-service) and [video-service](api/index.html#video-service).
The plan, done, is
[Content warnings](plans/done/content-warnings.md) (roadmap row 122).

## Contents

1. [The kinds](#the-kinds)
2. [On a post](#on-a-post)
3. [A warning the administrators gave](#a-warning-the-administrators-gave)
4. [What each reader chooses](#what-each-reader-chooses)
5. [Where it is shown](#where-it-is-shown)
6. [What the web app does](#what-the-web-app-does)
7. [Made with AI](#made-with-ai)
8. [On the fediverse and Bluesky](#on-the-fediverse-and-bluesky)
9. [Where the code is](#where-the-code-is)
10. [What is not built](#what-is-not-built)

## The kinds

Bluesky's four labels, so that they can cross one for one, a fifth for a
warning in the author's own words, as the fediverse writes them, and a sixth
for what was made with AI.

| `warning` | Said to people | For |
| --- | --- | --- |
| `adult` | Adult content | Explicit sexual images |
| `suggestive` | Sexually suggestive | Sexual, without nudity |
| `graphic` | Graphic media | Violence, gore or disturbing images |
| `nudity` | Non-sexual nudity | Artistic nudes, for example |
| `other` | The author's words, up to 100 characters | Spoilers, grief, anything else |
| `ai` | Made with AI | Pictures or video made or changed by AI ([Made with AI](#made-with-ai)) |

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

A site administrator who did not write a post gives it its warning as a
moderator (built on 2026-11-16), from three places:

1. **A report:** somebody reports a post as **Should have a content
   warning** (`needs_warning`, for posts alone).
2. **Its case:** "Content warning" in the case's ⋮ menu gives the post a
   warning, and closes the case as `warned` ([Reports](abuse-reports.md)).
3. **The post's ⋮ menu,** without a report, and a reply's.

The warning goes through the same `PATCH /…/:id/warning`: an administrator
who is not the post's author may change any post's, a reply or one from
another server too, and what they give is marked `warning_by_moderator`.
Taking it off takes the mark with it.

1. **Its author is told,** by a `medium_warned` notification that no choice
   turns off and that does not name the administrator
   ([Notifications](notifications.md)).
2. **Its author cannot change it or take it off:** they are answered 403
   `warning_not_allowed`, unless they ask for the warning it already has.
   The web app does not offer them the choice on such a post. They can
   report it, as with any decision.
3. **It goes to the fediverse and Bluesky** as an author's does, and an
   edit there does not take it off here.

## What each reader chooses

Each person chooses, for each kind, what they see of a post with that
warning (built on 2026-11-16):

| Choice | What they see |
| --- | --- |
| `show` | The post as any other |
| `warn` | The warning and View, the post blurred behind it |
| `hide` | In a list, one line, "Hidden: Graphic media", which opens the post if pressed; on the post's own page and in the photo theatre, as Warn |

Every kind starts at `warn`, but `ai` at `show`: it says what a post is,
not what it holds. Somebody not signed in has those and no choice. A post a moderator gave its warning is seen as its kind, as any
other.

1. **Read:** `GET /people/:id/content-preferences`, answered with
   `{ "choices": { "adult": "warn", … } }`, every kind there.
2. **Changed:** `PATCH /people/:id/content-preferences` with
   `{ "choices": { "graphic": "hide" } }`; the kinds not named stay as they
   were. Refused with 400 `content_preference_invalid` for a kind that is
   not one or a choice that is not `show`, `warn` or `hide`.
3. **Theirs alone:** both answer 403 for anybody but the person, an
   administrator too. What somebody chose not to see is nobody else's
   business.

It is stored in `content_preferences` (migration
`20261122000024_content_preferences`): a person's choices as JSON, only the
kinds not at `warn`, removed with the person when they are purged. The
server hides nothing because of it: a hidden post is still sent, and the
web app draws it as one line, so that a page holds as many posts as it did
and nothing is read again when a choice changes.

## Where it is shown

1. **Everywhere a post is drawn** in the web app (below).
2. **A link pasted elsewhere** shows nothing of the post: web-shell gives no
   picture of it (the site's own is used), and the warning in place of its
   words, "Content warning: graphic media" or "Content warning: " and the
   author's words ([The web shell](web-shell.md)).
3. **Other servers** are told, and their warnings are read
   ([On the fediverse and Bluesky](#on-the-fediverse-and-bluesky)).

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
   administrators gave. A site administrator has it on every post and
   reply they did not write, and a case about a post has it in its ⋮ menu
   ([A warning the administrators gave](#a-warning-the-administrators-gave)).
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
   them, and the warning for its title. Each reader sees it as they chose
   for its kind: as any other at Show, and at Hide, in a list, as one line
   that opens it.
5. **A profile's Photos card** leaves out a photo with a warning: the card
   is a glance at who somebody is.
6. **Settings › Content** is where each person chooses Show, Warn or Hide
   for each kind ([What each reader chooses](#what-each-reader-chooses)).

## Made with AI

The sixth kind (built on 2026-11-16) says that a post's pictures or video
were made or changed by AI. It is a mark, not a warning of what a post
holds:

1. **Shown, not hidden:** readers start at `show`, and see the post with a
   quiet line under it, "Made with AI". A reader can choose Warn or Hide
   for it as for any kind.
2. **The author says so,** in the composer as for any kind; or **the file
   does:** the web app reads a chosen photo or video for IPTC's digital
   source type `trainedAlgorithmicMedia` or
   `compositeWithTrainedAlgorithmicMedia`, in its XMP or its C2PA manifest,
   and if it is there chooses Made with AI and says why. The author sees it
   and may take it off. It is read in the browser, since a video goes from
   there straight to storage and a photo's signs are gone once it is made
   again at its sizes; another client can read the same words.
3. **A reader can ask for it:** **Made with AI, not marked**
   (`needs_ai_label`) is a reason to report a post, and the administrators
   add the kind as any warning
   ([A warning the administrators gave](#a-warning-the-administrators-gave)).
4. **No guessing:** nothing looks at a picture to decide whether AI made
   it.
5. **Not sent to other networks,** which have no field for it, and not read
   from them: on the fediverse, as a warning, it would hide the post. A
   link pasted elsewhere shows the post as it is.

## On the fediverse and Bluesky

A post's warning goes with it to both networks, as it is shown here, but
Made with AI, which goes to neither:

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

**From other servers** (built on 2026-11-15), a post comes with its
warning, and its pictures, GIF, video or sound are kept behind it, as a
post written here is:

| Network | What it sends | Kept here as |
| --- | --- | --- |
| The fediverse | A Note's `summary`, the words its author put before it | Other, in those words, cut to 100 characters |
| The fediverse | `sensitive: true` with no `summary` | Other, "Sensitive" |
| The fediverse | An Article's `summary` | Nothing: an article's summary is its opening |
| Bluesky | `porn`, `graphic-media` (or the older `gore`), `sexual`, `nudity` | Adult, Graphic, Suggestive, Nudity; with several, the first in that order |

A warning added or taken off by an edit there is followed here, unless a
moderator here gave the post its warning, which stays. A post kept before
this was built keeps what it was kept with until it is next changed there.

## Where the code is

| What | Where |
| --- | --- |
| The kinds, and checking what a request says | `anahita-libs/data/entities/warning.go` (`NormalizeWarning`) |
| Setting it, 400 `warning_invalid`, 403 `warning_not_allowed` | `anahita-libs/handlers/warning.go` (`SetWarning`), tested in `warning_test.go` |
| Sent with a post | `anahita-libs/responses/warning.go`, used in `medium.go`, `parent.go`, `node.go` |
| `PATCH /…/:id/warning` | `anahita-libs/handlers/post_settings.go` (`PostSettings.EditWarning`), for every kind of post |
| Where it is stored | table `nodes`; `graph-grpc-service/data/repositories/nodes_add.go`, `nodes_edit.go`, `helpers.go`; `node.proto` fields 101 to 103 |
| A video's, until it is made | `video-service/data/repositories/uploads.go`, `video-service/worker/worker.go` |
| In a person's download | `import-export-service/archive/archive.go` |
| A link pasted elsewhere | `web-shell/shell/pages.go` (`warningDescription`) |
| Sent to the fediverse | `activitypub-service/ap/note.go` (`WithWarning`), its words from `entities.WarningSaid` |
| Sent to Bluesky | `atproto-service/bsky/outgoing.go` (`WithLabel`), its label from `entities.WarningLabel` |
| Made with AI | `anahita-libs/data/entities/warning.go` (`WarningAI`, `WarningHides`, `WarningChoiceDefaultFor`); `src/utils/madeWithAI.js` (anahita-web) |
| A moderator's warning | `anahita-libs/handlers/warning.go` (`IsWarningModerator`, `SetWarningBy`); the event `moderation.medium.warned.v1` from `graph-grpc-service/cmd/server/handlers/medium.go` (`announceWarned`); `notification-service/cmd/api/handlers/event_medium_warned.go`; the reason in `abuse-report-service/data/reasons/reasons.json` |
| Each reader's choices | `anahita-libs/data/entities/warning.go` (`WarningChoices`); `graph-grpc-service/data/repositories/content_preferences.go`; `actor-service/cmd/api/handlers/person_content_preferences.go` |
| The web app | `src/utils/warnings.js`, `src/utils/contentChoices.js`, `src/containers/actors/Settings/ContentChoices.jsx`, `src/components/ContentWarningDialog.jsx`, `ContentWarningButton.jsx`, `WarningCover.jsx`, `src/containers/controls/medium/ContentWarning.jsx` (anahita-web) |

## What is not built

Everything in the plan is built. What it leaves out on purpose is in its
[Out of scope](plans/done/content-warnings.md#out-of-scope).
