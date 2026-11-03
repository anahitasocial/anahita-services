# Content warnings (roadmap row 122)

Asked for by the user on 2026-11-03: "In Fediverse, both activitypub and
atproto, is there a way to flag a post as 18+ or violent content? Can we
blur them with a warning so viewing them requires clicking on a view
button?", and then: "We don't want to censor, but we want to give viewers
consent." Shaped with the user the same day, after Bluesky's moderation
settings and Mastodon's content warnings. Not built.

## Contents

- [Why](#why)
- [What the networks do](#what-the-networks-do)
- [The kinds](#the-kinds)
- [What an author does](#what-an-author-does)
- [What a reader chooses](#what-a-reader-chooses)
- [What a post looks like](#what-a-post-looks-like)
- [When a moderator adds one](#when-a-moderator-adds-one)
- [Federation](#federation)
- [Storage and the API](#storage-and-the-api)
- [Out of scope](#out-of-scope)
- [Phases](#phases)

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

Bluesky's four, so that labels cross both ways one for one, and a fifth for
the warnings in words that the fediverse writes.

| Kind (`warning`) | Said to people | AT label | ActivityPub |
| --- | --- | --- | --- |
| `adult` | Adult content: explicit sexual images | `porn` | `sensitive`, `summary` "Adult content" |
| `suggestive` | Sexually suggestive: no nudity | `sexual` | `sensitive`, `summary` "Sexually suggestive" |
| `graphic` | Graphic media: violence, gore, disturbing images | `graphic-media` | `sensitive`, `summary` "Graphic media" |
| `nudity` | Non-sexual nudity: artistic nudes, for example | `nudity` | `sensitive`, `summary` "Nudity" |
| `other` | Other warnings, in the author's words: spoilers, grief… | none | `sensitive`, `summary` the author's words |

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
at **Warn**, for everybody, so that seeing any of it is chosen. Somebody not
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

- Two columns on `nodes`, for a medium: `warning` (one of the five keys, or
  empty) and `warning_text` (up to 100 characters, for `other`), and
  `warning_by_moderator`. One migration, in `init.sql` and `upgrade.sql` too.
- Sent with every post as `warning: { kind, text, by_moderator }`, or no
  `warning`.
- Taken when a post is made or changed as `warning` and `warning_text`,
  refused (`warning_not_allowed`) when a moderator's warning would be taken
  off by its author.
- The reader's choices in their settings, as their other choices are kept:
  `GET` and `PATCH /people/:id/content-preferences`, `{ adult: "warn", … }`.
- `POST /abuse-reports/cases/:id/warn` `{ kind, text }`, and the same on a
  post for an administrator.

## Out of scope

- A site that hides adult content from visitors, or from everybody: the
  operator's choice, if one asks for it.
- Profiles marked as a whole, and their pictures and covers.
- A reader's own words to be warned of, as Mastodon's filters are.
- Labels from Bluesky's labellers on other people's posts.
- Guessing a kind from a warning's words coming in.

## Phases

1. **Posts carry a warning:** the columns, the API, the composer and the
   reply form, the card's Warn, Show and Hide with everybody at Warn.
2. **The reader's choices:** the Content tab and its preferences.
3. **Federation** in and out, pictures from elsewhere kept and blurred.
4. **Moderators:** the report reason, the case's action, the post's menu,
   the notification.

About three days in all, services and web.
