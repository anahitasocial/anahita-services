# Language on posts

*Roadmap item 20, planned 2026-09-28. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

## Context
- **The column already exists:** `nodes.language varchar(50)` (`migration-job/schema/init.sql:75`). The read path selects it (`graph-grpc-service/data/repositories/helpers.go:94`) and it's on `entities.Node.Language` (`anahita-libs/data/entities/node.go`), but **nothing writes it**.
- **Why it's worth filling, even with auto-detection available:**
  1. **Federation:** ActivityPub posts carry `contentMap` / `language`, and Mastodon users filter timelines by language. Bluesky posts carry `langs` (up to 3 BCP-47 codes), which feeds and apps filter on. Without it, our posts arrive untagged.
  2. **Short posts defeat detection:** a few words, emoji, names, or close languages (es/pt, no/da, fr-CA vs fr) are often guessed wrong. The author's choice is authoritative; detection is a default and a backfill tool.
  3. **Accessibility:** a `lang` attribute on each post's HTML makes screen readers pronounce it correctly.
  4. **Translation:** with the source language known, the UI shows "Translate" only when it differs from the viewer's language, and services translate better with an explicit source.
  5. **Feeds and search:** item 14's feed definition has a `language` filter waiting for this.

## Decisions (item 20)
- **A picker with a smart default, plus offline detection.**
- **The Translate button is a later, separate item.**

## Design
**Format:** BCP-47 language tags, normalised to a primary 2-letter ISO 639-1 code where one exists (`en`, `fr`, `fa`, `es`). Regional tags are allowed when chosen explicitly (`pt-BR`, `fr-CA`). `und` means undetermined.
- One language per post in v1. Bluesky allows up to 3, which we can add later as `languages`.
- The column stays; add an index for item 14's feed filter and search.

**Person setting "Posting language"**
- A new person field, `posting_language`, defaulting to the person's UI language.
- The web app already has en-GB and fr-FR through i18next, and it can also read the browser language on first use.
- It's shown in Settings → Account and during onboarding (a small optional step, or silently from the browser).
- **Also set `language` on the person node** as their preferred UI language, for notifications and emails. That's a separate field from `posting_language`.

**Setting it on posts**
- Media create and edit accept `language`, validated as a well-formed tag against a known list. Replies (item 7) and reposts inherit nothing; they get their own.
- **Server fallback when the client sends none** (API clients, federation inbound without a tag, or backfill): detect with **`github.com/pemistahl/lingua-go`**.
  - It's offline, accurate, and makes **no external calls**, which suits sovereignty (item 9).
  - Load only the ~40 most common languages to keep memory reasonable. Configurable with `LANGUAGE_DETECT_LANGS`.
  - Accept the result only above a confidence threshold, and only for texts of at least about 20 characters. Otherwise use `und`.
  - It lives in `anahita-libs/language`, used by text-service, photo-service (captions), video-service and the federation inbound path.
- **Inbound federation (item 8):** use the remote post's own `contentMap` / `language` or `langs` when present. Detect only when it's missing.

**Backfill:** a one-off job (the sweeper-style binary, or a `migration-job` command) that detects the language of existing media with text (about 17,000 notes and comments today, and more articles and topics). It skips very short texts (`und`) and writes in batches. It's idempotent: it only fills rows where `language IS NULL`.

**Outbound (item 8):**
- ActivityPub: `contentMap: { "<lang>": "<html>" }` plus `language`.
- Bluesky: `langs: ["<lang>"]`.
- Nothing is sent when the language is `und`.

**Web shell (item 10):** set `<html lang>` to the instance default, and `og:locale` from the post's language.

## Web (anahita-web)
- **Composer:** a compact language chip ("EN") next to the audience picker (item 13) and the licence picker (item 19).
  - It defaults to the person's posting language.
  - While typing, a debounced call to `GET /language/detect?text=…` (served from `anahita-libs`, limited to logged-in users and rate-limited) suggests "Looks like French — switch?" when detection is confident and differs. It **never switches silently**.
  - The chip menu lists common languages first, then all of them, with search.
- **Rendering:** each post body (`NodeBody.jsx`) gets `lang={node.language}` (omitted for `und`). Mixed-language pages then read correctly with screen readers.
- **Settings:** a "Posting language" select.
- **Feeds (item 14):** the editor's language filter becomes active.

## Later: the Translate item (recorded, not planned in detail)
- A pluggable `translation-service`, **self-hosted LibreTranslate by default**, with DeepL (EU-based) as an optional provider through config.
- It caches translations per (node, target language) and invalidates them on edit.
- A "Translate" button appears only when `post.language` ≠ the viewer's UI language and isn't `und`.
- Rate-limited per person. **Admins choose whether it's enabled.**

## Estimate

| Part | Time |
| --- | --- |
| `anahita-libs/language` (validation, lingua-go detection with threshold), write `language` on create and edit in text, photo and video services, index | 3–4 h |
| Person `posting_language` + UI-language field, detect endpoint, rate limit | 1–2 h |
| Backfill job | 1–2 h |
| Web: composer chip + suggestion, `lang` attribute on bodies, settings select, i18n | 3–4 h |
| **Total** | **about 8–12 h, 2–3 sessions** |

Federation wiring is a few lines inside item 8.

## Verification
- **Composer:** a French speaker with posting language `fr` writes a post and it's saved as `fr`. Typing a clearly English paragraph shows the switch suggestion, and ignoring it keeps `fr`.
- **Detection:** a 5-word post created through the API with no language is saved as `und`. A long English article gets `en`.
- **Backfill:** after the job, no media with more than 20 characters of text has a NULL language, and a second run changes nothing.
- **Accessibility:** the rendered HTML has `lang="fr"` on French posts inside an English UI.
- **Federation (item 8):** the ActivityPub Note has `contentMap.fr`, the Bluesky post has `langs: ["fr"]`, and a Mastodon account filtering out French doesn't see it.
- **Feeds (item 14):** a feed with `language: "fr"` shows only French posts.

## Dependencies
- **Soft:**
  - item 13 and item 19 (the composer chips sit together; build them as one "post options" row);
  - item 14 (language filter);
  - item 8 (`contentMap`/`langs` in and out);
  - item 10 (`og:locale`);
  - item 9 (offline detection keeps it sovereign);
  - the future Translate item depends on this one.

## Documentation

A phase is finished only when these pages match the code. anahita-services pages are under its `docs/`, anahita-web pages under its `docs/`. When a page is added, add it to that repo's `docs/README.md` contents too.

- **services `configuration.md`**: `LANGUAGE_DETECT_LANGS`.
- **services `architecture.md`**: the `anahita-libs/language` package and the backfill job.
- **web `customising.md`**, *Languages*: the difference between the interface language and a person's posting language.
- **web `architecture.md`**, *Translations*: the `lang` attribute on post bodies and the composer language chip.

## What was built (2026-10-05)

A smaller form of this plan, chosen with the user after asking whether the
feature was needed at all. They looked at Bluesky's composer and decided it
was; the recommendation was to build what an author's choice needs and leave
out what guesses.

**Built.**
- `anahita-libs/language`: tags are checked and spelled the usual way.
  Two-letter codes are held to ISO 639-1; a region is allowed; `und` is
  accepted.
- `language` on creating and editing a note, article, topic and photo, and in
  every post's response. The column and its plumbing through the graph were
  already there; nothing had written to it.
- `language` on a person, as the language they usually post in, and in
  `/oauth/userinfo` so the composer can start on it.
- Web: a language button in the composer beside the audience button, the
  `lang` attribute on post text, and "Language you post in" in the profile
  form and the onboarding profile step.

**Left out, on purpose,** and listed in `docs/post-language.md`: detection
(lingua-go was never added; its language data would have added on the order
of a hundred megabytes to each service that used it, which was not measured),
the "looks like French" suggestion, the backfill of old posts, the
`/language/detect` endpoint, the index, and changing a post's language from
the web app's edit form.

**Where it differs from the plan.**
- **One field on a person, not two.** The plan had `posting_language` and a
  separate UI language on the node. The interface language is the web app's
  own business; the node's `language` is the posting language.
- **The composer starts on the language last posted in**, ahead of the
  profile's. Somebody who switched to French for a post is probably still
  writing French.
- **Names of languages come from the browser** (`Intl.DisplayNames`), in the
  language the app is read in, so there is no list of names to translate.

**Tests.** The tag rules; reading and refusing a language in requests; a guard
that all four post handlers read it through the one function, for creating
and for editing; the viewer's claim. In the web app, where the composer
starts.

**Not checked.** Posting with a language through the running stack, and the
button, the setting and the `lang` attribute in a browser.

**Found when the user tried it (2026-10-05).** A note posted in Persian was
saved with no language. "The plumbing through the graph was already there"
above was wrong: the column, the request field and the repository's insert
were, but graph-grpc's handlers copy a post's fields one by one and
`language` was not among them, for creating or for editing. The same was true
of a person's posting language. So nothing had ever been saved, and no test
went as far as the handler. Both lists now carry it, the post lists were
moved into `medium_fields.go` beside the actors' `actor_fields.go`, and a
test walks each list so a field left off it fails. Posts made before this
fix have no language.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-09-28 |
| Implementation | done 2026-10-05, in the smaller form above |
