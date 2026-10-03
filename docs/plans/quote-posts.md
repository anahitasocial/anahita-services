# Quote posts

*Roadmap item 25, planned 2026-10-02. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

## Context
A quote post is a new note that embeds another post beneath the author's own words. It sits between a repost (no words of your own) and a reply (stays in the other person's thread).

Both networks now have it, and both let the quoted person keep control, which is the part to copy:
- **Bluesky:** a post embeds a record (`app.bsky.embed.record`). The quoted author can detach their post from a quote, and can turn quoting off per post (a "postgate").
- **Mastodon (4.5):** quote posts with consent. Each post carries a quote policy (anyone, followers, nobody), a quote is approved against that policy, and the quoted author can revoke it later.

## What exists
- Reposts: `node.base.repost-service.repost.v1` nodes created through `POST /reposts/:parent_id/` (`feed-service/cmd/api/routes.go:37-39`). They carry no text.
- Notes: `text-service`, with `parent_id` used for replies after item 7.
- Nothing links a note to a post it quotes.

## Decisions
- **A quote is an ordinary note** with a `quote_id` pointing at the quoted post. It is owned by its author, has its own audience, likes and replies, and appears in feeds as any note does. Any medium (note, article, topic, photo, video) can be quoted; only notes can quote.
- **The quoted person stays in control:**
  - a per-person default, "Who can quote my posts": **anyone** (default), **followers**, or **nobody**;
  - the same choice per post, in the composer's post options;
  - the quoted author can **remove their post from a quote** at any time. The quote stays, showing "Post removed by its author".
- **Only public posts can be quoted** in the first version. Quoting a followers-only post would either leak it or produce embeds most readers can't see.
- **A quote never widens visibility.** The embed is rendered for each viewer only if that viewer can see the quoted post.

## Design
**Data**
- `nodes.quote_id bigint NULL` (indexed) on the quoting note, and `nodes.quote_count` on the quoted node (the column already exists and is returned, but nothing writes it), updated with the modifier set (the comment-count update fails today for want of one; see `notes-as-replies.md`).
- `nodes.quote_policy varchar(16) NULL` on media (`anyone` | `followers` | `nobody`; NULL means the owner's default), and `default_quote_policy` in the person's settings.
- `quote_detached_at datetime NULL` on the quoting note, set when the quoted author removes their post from it.
- One migration, mirrored in `schema/init.sql` and `legacy/upgrade.sql`.

**Creating a quote**
- `POST /notes/` accepts `quote_id`. text-service checks, in this order:
  1. the viewer can read the quoted post, and it is public;
  2. neither side blocks the other;
  3. the quoted post's policy allows this viewer (`followers` means the viewer follows the quoted author);
  4. the quoted post is not itself detached, deleted or archived.
- A refusal returns 403 `quote_not_allowed` with the reason, so the web app can say "This person doesn't allow quotes".
- People can always quote their own posts.
- Quoting a repost quotes the original. Quoting a quote is allowed; the embed shows one level only.

**Reading**
- Media responses for a quoting note gain `quote`: the embedded post (author, excerpt, first image, link), hydrated in the same batch as the feed, or a placeholder:
  - `{ "state": "unavailable" }` when the viewer can't see it, or it was deleted;
  - `{ "state": "detached" }` when its author removed it.
- `GET /{ns}/:id/quotes` lists the quotes of a post, filtered for the viewer (used by item 22's insights).

**Control for the quoted author**
- `DELETE /notes/:quote_note_id/quote`, callable by the quoted post's author: sets `quote_detached_at`, lowers `quote_count`.
- Changing a post's policy to `nobody` stops new quotes; existing ones stay until detached.

**Notifications:** `quote_add` to the quoted author ("X quoted your post"), through notification-service, respecting blocks.

**Feeds:** a quote is a top-level note, so it shows in followers' feeds and on the Posts tab (item 24). It is not a repost and is not hidden by the reposts setting.

**Federation (item 8, phase 2)**
- AT Protocol: `app.bsky.embed.record` out and in; our policy maps to a postgate with `disableRule` when `nobody`, and a detach maps to the postgate's `detachedEmbeddingUris`.
- ActivityPub: Mastodon's quote posts (the `quote` property with a quote authorization, and `interactionPolicy.canQuote` from our policy). Also emit the older `quoteUrl` / `_misskey_quote` fields that Misskey-family servers read. `followers` has an exact equivalent on Mastodon; on Bluesky it becomes `nobody`, which is the safer side.
- Until phase 2, quotes of local posts are local; a quote of a remote post is stored with its `uri` and shown as a link card.

## Web (anahita-web)
- **Repost button becomes a small menu:** Repost, Quote. Quote is disabled with a tooltip when the post's policy forbids it.
- **Composer in quote mode:** the quoted post shown as a compact embedded card under the text field, with a remove ✕.
- **Rendering:** the embedded card under the quoting note's text (author, excerpt, first image), linking to the quoted post; the two placeholder states.
- **Counts:** a quote count next to the repost count, opening the insights panel on the Quotes tab (item 22).
- **Controls:** "Remove my post from this quote" in the menu of a quote, shown to the quoted author. "Who can quote this post" in the composer's post options and the post's edit dialog. "Who can quote my posts" in Settings → Privacy.
- i18n in en-GB and fr-FR.

## Estimate

| Part | Time |
| --- | --- |
| Columns and migration, create checks, policy resolution, detach, counts, quotes list, hydration with placeholders, tests | 6–8 h |
| Notification | 1 h |
| Web: repost menu, composer quote mode, embedded card, placeholders, policy controls, detach | 5–6 h |
| **Total** | **about 12–15 h, 3–4 sessions** |

## Verification
- Quoting a public post works; the quote shows in the author's followers' feeds with the embed.
- Quoting a followers-only post returns 403. So does quoting when the policy is `nobody`, or `followers` and the viewer isn't one, or when either side blocks the other.
- After the quoted author removes their post from a quote, the quote shows "Post removed by its author", the count drops, and the quoting author can't re-attach it.
- When the quoted post is deleted, or later made followers-only, a stranger sees the unavailable placeholder; a follower still sees the embed.
- A quote of a quote shows one embedded level.
- The quoted author gets one notification per quote, and none from a person they blocked.

## Dependencies
- **Soft:** item 13 (the composer's post options row, where the per-post policy sits), item 7 (replies; a quote is a top-level note, not a reply), item 22 (the Quotes tab in insights), item 24 (quotes show on the Posts tab), item 4b (quotes can be reported like any note).
- **Before item 8 phase 2**, so quotes federate from the start of that phase.

## Documentation

A phase is finished only when these pages match the code. anahita-services pages are under its `docs/`, anahita-web pages under its `docs/`. When a page is added, add it to that repo's `docs/README.md` contents too.

- **services `permissions.md`**: a *Quotes* section (who can quote, the three policies, removing a post from a quote, why only public posts).
- **services `architecture.md`**: `quote_id`, `quote_count`, the quote placeholders in responses.
- **web `architecture.md`**: the repost menu, quote mode in the composer, the embedded card.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-10-02 |
| Implementation | not started |
