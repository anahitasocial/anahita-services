# Content warnings (roadmap row 122)

Asked for by the user on 2026-11-03: "In Fediverse, both activitypub and
atproto, is there a way to flag a post as 18+ or violent content? Can we
blur them with a warning so viewing them requires clicking on a view
button?", and then: "We don't want to censor, but we want to give viewers
consent." Shaped with the user the same day, after Bluesky's moderation
settings and Mastodon's content warnings. **Made with AI** added as a sixth
kind on 2026-11-07, at the user's word, after asking how to tell AI images
apart ([Made with AI](#made-with-ai)). **Phase 1 built on 2026-11-09:** posts
carry a warning, everybody at Warn ([Content warnings](../content-warnings.md));
the rest is not built.

## Contents

1. [Why](#why)
2. [What the networks do](#what-the-networks-do)
3. [The kinds](#the-kinds)
4. [What an author does](#what-an-author-does)
5. [What a reader chooses](#what-a-reader-chooses)
6. [What a post looks like](#what-a-post-looks-like)
7. [When a moderator adds one](#when-a-moderator-adds-one)
8. [Federation](#federation)
9. [Storage and the API](#storage-and-the-api)
10. [Made with AI](#made-with-ai)
11. [Out of scope](#out-of-scope)
12. [Phases](#phases)

## Why

Some posts are not for every reader at every moment: sex, nudity, violence,
grief, a film's ending. Anahita removes none of them for being so. What it
gives is consent: the author says what a post holds, and each reader
chooses whether to see that kind at once, behind a warning, or not at all.

Today an author cannot say it, and a post from elsewhere that its author
marked is kept without its pictures (`docs/federation.md`): the opposite of
both halves of that.

## What the networks do

**ActivityPub** has two fields. `summary`, in ActivityStreams itself, is a
warning in the author's own words, which Mastodon shows in place of the post
until **Show more** is pressed. `sensitive: true`, which Mastodon added and
nearly every server reads (declared as `as:sensitive` in its JSON-LD
context; not in the published vocabulary), blurs the pictures. There are no
kinds: what a post holds is said only in the warning's words. Readers choose
how media marked sensitive is shown and whether warnings open by
themselves; moderators can mark another person's media sensitive.

**AT Protocol** has self-labels on a post, from a fixed list: `porn`,
`sexual`, `nudity` and `graphic-media` (`gore` is its old name). There is no
warning in words. Each reader chooses **Show**, **Warn** or **Hide** for each
label; labellers can label other people's posts.

## The kinds

Bluesky's four, so that labels cross both ways one for one, a fifth for
the warnings in words that the fediverse writes, and a sixth for what was
made with AI ([Made with AI](#made-with-ai)).

| Kind (`warning`) | Said to people | AT label | ActivityPub |
| --- | --- | --- | --- |
| `adult` | Adult content: explicit sexual images | `porn` | `sensitive`, `summary` "Adult content" |
| `suggestive` | Sexually suggestive: no nudity | `sexual` | `sensitive`, `summary` "Sexually suggestive" |
| `graphic` | Graphic media: violence, gore, disturbing images | `graphic-media` | `sensitive`, `summary` "Graphic media" |
| `nudity` | Non-sexual nudity: artistic nudes, for example | `nudity` | `sensitive`, `summary` "Nudity" |
| `other` | Other warnings, in the author's words: spoilers, grief… | none | `sensitive`, `summary` the author's words |
| `ai` | Made with AI: pictures or video made or changed by AI | none | none, for now |

The `summary` sent for the first four is in the installation's default
language.

## What an author does

The composer, short and long, and the reply form have **Content warning**
beside the audience. It opens the five kinds as a list, each with its line
of explanation; Other asks for the words, up to 100 characters. The chosen
kind shows beside the button, and can be taken off before sending. Any kind
of post can have one: notes, photos, videos, articles, topics and replies.

Editing a post can add, change or take off its warning.

## What a reader chooses

A **Content** tab in a person's settings, one card: each kind with its name,
its line, and a full-width row of **Show · Warn · Hide**. Every kind starts
at **Warn**, for everybody, so that seeing any of it is chosen, but Made with
AI, which starts at **Show**: it says what a post is, not what it holds. Somebody not
signed in has Warn and no choice.

## What a post looks like

As the reader's choice for its kind says. Where they differ, a post with a
warning that a moderator gave is treated as that kind.

- **Warn.** In place of its words and pictures: the warning, with its kind's
  icon, and **View**. The pictures are drawn blurred behind it, as a blocked
  profile's are, so the card keeps its height and nothing jumps. View opens
  that post for that showing of the page.
- **Show.** The post as any other, under one quiet line saying its kind.
- **Hide.** The post is a single line where it would have been: "Hidden:
  graphic media", which opens it if pressed. It stays in the list rather than
  being left out by the server, so that a page holds as many as it did and
  nothing has to be read again when the choice is changed.

The same goes everywhere a post is drawn: feeds, profiles, a post's own page,
search, quotes and replies. A link to a post shared elsewhere does not show
its picture when it has a warning (`og:image` in web-shell), and the
download of somebody's data keeps the warning.

## When a moderator adds one

A report can give a new reason, **Should have a content warning**
(`needs_warning`, for posts). The administrators who decide it can add a
warning of a kind they choose, from the case, and the case is closed as
`actioned` with the action `warned`. The author is told by a notification
which kind was added, and may not take off a warning a moderator added; they
can report it, as with any decision. A warning added this way is sent on to
the fediverse and Bluesky as an author's is.

The same is open to an administrator from a post's ⋮ menu, without a
report.

## Federation

**Out.** As in the table: ActivityPub's `sensitive` and `summary`; for an
Article, whose `summary` carries its opening today, the warning takes its
place. Bluesky's self-label, as `labels` on the post record; Other sends
none.

**In.**
- ActivityPub: `sensitive` or a `summary` on a Note makes a post with an
  `other` warning in those words, or "Sensitive" when there are none. Its
  pictures are kept, and drawn behind the warning, no longer left out.
- Bluesky: each of the four labels makes a post of its kind; several, the
  first of `porn`, `graphic-media`, `sexual`, `nudity`.
- A warning added or taken off by an edit there is followed here.

## Storage and the API

1. Two columns on `nodes`, for a medium: `warning` (one of the five keys, or
   empty) and `warning_text` (up to 100 characters, for `other`), and
   `warning_by_moderator`. One migration, in `init.sql` and `upgrade.sql` too.
2. Sent with every post as `warning: { kind, text, by_moderator }`, or no
   `warning`.
3. Taken when a post is made or changed as `warning` and `warning_text`,
   refused (`warning_not_allowed`) when a moderator's warning would be taken
   off by its author.
4. The reader's choices in their settings, as their other choices are kept:
   `GET` and `PATCH /people/:id/content-preferences`, `{ adult: "warn", … }`.
5. `POST /abuse-reports/cases/:id/warn` `{ kind, text }`, and the same on a
   post for an administrator.

## Made with AI

Asked about on 2026-11-07: how to tell AI images from others. Camera details
in a file prove nothing (they are written in seconds, and Anahita drops them
on upload, with the place a photo was taken). What can be believed without
checking is a file saying it **was** made with AI: nobody gains by faking that
against themselves. A file saying it came from a camera means something only
with a signature that checks out, which is roadmap row 131
([content-credentials.md](content-credentials.md)).

1. **The author says so,** with the sixth kind, in the composer as any other.
2. **The file says so.** On upload, before a photo is made again at its
   sizes and a video transcoded, the photo and video services look for the
   two plain signs: IPTC's `digitalSourceType` of `trainedAlgorithmicMedia`
   or `compositeWithTrainedAlgorithmicMedia` in its XMP, and a C2PA manifest
   whose actions say the same. Either answers the upload with
   `made_with_ai: true`; the composer then chooses Made with AI, with a line
   saying why ("This picture says it was made with AI"). The author sees it
   and may take it off; nothing is marked behind their back.
3. **A reader asks for it:** **Made with AI, not marked** is a reason to
   report a post (`needs_ai_label`), and the administrators who decide it add
   the kind, as for any warning.
4. **No guessing.** No classifier says whether a picture was made with AI:
   they are wrong often enough to mark real people's photos, and a cost to
   run.
5. **Federation.** Neither network has a field for it. Sent as a warning, it
   would hide the post behind it on Mastodon, which is more than it says, so
   it is not sent, and not read coming in. Looked at again when either
   network has one.

## Out of scope

1. A site that hides adult content from visitors, or from everybody: the
   operator's choice, if one asks for it.
2. Profiles marked as a whole, and their pictures and covers.
3. A reader's own words to be warned of, as Mastodon's filters are.
4. Labels from Bluesky's labellers on other people's posts.
5. Guessing a kind from a warning's words coming in.

## Phases

1. **Posts carry a warning:** the columns, the API, the composer and the
   reply form, the card's Warn with everybody at Warn. **Built on
   2026-11-09;** Show and Hide come with the reader's choices. Also: a
   `PATCH /…/:id/warning` of each kind, the post's menu for its author, and
   a link pasted elsewhere showing nothing of the post.
2. **The reader's choices:** the Content tab and its preferences.
3. **Federation** in and out, pictures from elsewhere kept and blurred.
4. **Moderators:** the report reason, the case's action, the post's menu,
   the notification.
5. **Made with AI:** the sixth kind, the signs read from a file on upload,
   the composer choosing it, the report reason: about one session.

About four days in all, services and web.
