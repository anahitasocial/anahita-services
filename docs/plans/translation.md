# Translating posts (item 43)

Asked for by the user on 2026-10-12, once posts from Mastodon were being
read here: a **Translate** button under a post that is in another language,
as Mastodon has. Planned, not built. It takes the place of the "Translate
item" sketched at the end of [content-language.md](done/content-language.md).

## What Mastodon does

A Mastodon server translates through one of two services, whichever its
administrator configured, and shows "Translate" under a post whose language
is not the reader's:

| Service | What it is | How a server uses it |
| --- | --- | --- |
| **DeepL** | A company in Germany. The better translations of the two, for the languages it has (about thirty). Paid by the character, with a free tier | An API key |
| **LibreTranslate** | Open source, run by whoever wants it, on models that are open too. Rougher translations, more languages, nothing leaves the installation | The address of an instance, the administrator's own or somebody else's |

Only posts that are open to everybody are translated, and a translation is
kept for a while so that a post read by many is translated once.

## What is wanted here

**More than one service, chosen by the installation.** The user's words:
"whichever is best and used by Mastodon services. Perhaps we can build
integration with multiple services." So a small contract that a service is
written to, and the two Mastodon uses to begin with:

- **LibreTranslate** as what the documentation leads with, because an
  installation can run it itself: the same reason storage is any
  S3-compatible store and not one company's
  ([cloud-neutral.md](done/cloud-neutral.md));
- **DeepL** for an installation that would rather pay for better;
- room for others without touching what calls them: another translation
  API, or a model the installation runs.

## Sketch

- **A `translation-service`,** optional like the recommendation and
  ActivityPub services: where it is not running, the web app shows no
  button.
- **One request:** `POST /translations` with the post and the language
  wanted; the answer is the text in that language, the language it was
  taken to be in, and which service made it, which the web app says under
  the translation as Mastodon does.
- **Providers behind one interface** (`Translate(text, from, to)`, and the
  languages it has), chosen by a setting, with a second to fall back on
  when the first does not have the language or is down.
- **Kept:** a translation is stored by post, language and the post's
  `updated_at`, so a changed post is translated again and an unchanged one
  never twice.
- **Only what the reader can already read,** and, for a service outside
  the installation, only what is open to everybody: a post for followers
  is not sent to a company. With a LibreTranslate the installation runs
  itself, that limit can be lifted by the administrator.
- **Limits** for each reader, with the shared limiter.
- **The web app:** "Translate" under a post or a reply whose language
  (kept since item 20, and sent by other servers with their posts) is not
  the reader's; "Show original" to go back.

## To decide when it is built

- Whether a post from another server is sent to a translation service at
  all without the installation saying so, since it was written elsewhere.
- Whether an installation may offer both services and let each reader
  choose.
- Recipes ([recipes.md](recipes.md)): which kinds of site get a
  LibreTranslate of their own to begin with.

## Depends on

Language on posts (item 20, done). Nothing else.
