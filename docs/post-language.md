# The language of a post

[Documentation](README.md) › The language of a post

Every post can say which language it is written in. This page covers what is
stored, how it is chosen, and what uses it.

## Contents

- [What it is for](#what-it-is-for)
- [The tags](#the-tags)
- [On a post](#on-a-post)
- [On a person](#on-a-person)
- [What the web app does](#what-the-web-app-does)
- [What is not built](#what-is-not-built)

## What it is for

- **Screen readers.** A French post on an English page is read with French
  pronunciation, because its text is marked with its language.
- **Other fediverse servers.** Mastodon and Bluesky both file a post under its
  language, and their users filter by it. Once this installation federates,
  its posts arrive tagged.
- **Later:** a translate button, which needs to know what it is translating
  from, and a language filter in custom feeds.

The author chooses. **Nothing guesses a language.** A post whose author did
not say has none, and that is deliberate: other servers show an untagged post
to everybody, while a wrongly tagged one is hidden from the people who could
read it.

One language per post. A post that mixes two is tagged with whichever the
author picks.

## The tags

BCP 47, kept simple: a language, and optionally a region.

| Tag | |
| --- | --- |
| `en`, `fr`, `fa` | A language, by its two-letter ISO 639-1 code |
| `fr-CA`, `pt-BR`, `es-419` | A language as written in a region |
| `yue` | A three-letter code, for a language with no two-letter one |
| `und` | Undetermined: the author would rather not say |

A tag is stored in its usual spelling whatever case it arrived in (`FR-ca`
becomes `fr-CA`). A two-letter code that is not a language, a language's name
instead of its code, or anything longer than a language and a region is
refused with 400 `language_invalid`.

The rules are in `anahita-libs/language`.

## On a post

`language` is accepted when a note, article, topic or photo is created
(`POST /notes/:owner_id/` and the others) and when it is edited
(`PATCH /…/:id`). It is optional. Left out when creating, the post has no
language; left out when editing, it keeps the one it has.

Every post in a response carries `language` when it has one.

It is stored in `nodes.language`, which the PHP application's schema already
had and nothing wrote to.

## On a person

A person can say which language they usually post in: `language` on
`PATCH /people/:id`. It is the same column on the person's own node, and it is
what a composer should start on.

The signed-in viewer is told theirs: `/oauth/userinfo` carries `language` when
it is set, and leaves it out when it is not.

This is the language somebody **writes** in. The language the interface is
shown in is a separate thing, chosen in the web app.

## What the web app does

- **A language button in the composer**, beside the audience button, showing
  the code (`EN`). It opens a list of about sixty languages, named in the
  language the app is being read in.
- **Where it starts**, in order: the language this person last posted in, in
  this browser; the posting language on their profile; the browser's language;
  English.
- **Posts are marked with their language** where they are drawn, so a screen
  reader reads each in the right one.
- **Profile settings** have "Language you post in". Left on "My browser's
  language", nothing is stored.

## What is not built

Decided on 2026-10-05, when the fuller plan was weighed against what uses a
language today:

- **Detecting the language from the text**, as a fallback when a client sends
  none or as a "looks like French" suggestion in the composer.
- **Tagging posts written before this existed.** They have no language. It
  could only be guessed, and guessing is what the point above leaves out.
- **Changing a post's language in the web app** after it is posted. The API
  accepts it; the edit form does not offer it yet.
- **An index on the column.** Nothing filters by language yet. Custom feeds
  will, and should add it then.
- **More than one language per post.** Bluesky allows three.

They belong with federation, a translate button or custom feeds, whichever
first needs them.
