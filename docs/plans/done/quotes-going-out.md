# Quotes going out (roadmap row 156)

## Context

The user, 2026-11-16, after posting a quote on banu.io: "neither mastodon
nor bluesky show the post I quote posted". Until now a quote went out as
the note around it alone (docs/federation.md, "Not sent"), so on Mastodon
and Bluesky it read as a plain post with nothing to say it quoted anything.

Mastodon shows quotes since 4.4 and makes them since 4.5, by FEP-044f: the
quoting note names the quoted post in `quote`, and the quoted post's server
approves each quote with a `QuoteAuthorization`; a post whose server says
nothing of who may quote it is shown as not quotable. Bluesky embeds a
quoted post that is on Bluesky (`app.bsky.embed.record`).

## Part 1: a line that leads to it (done 2026-11-16)

A post that quotes ends with a line, "↪" and the quoted post's address
(`federation.QuotedAddress`, `federation.QuotedLine`):

| The quoted post | Led to |
| --- | --- |
| Here, and public | Its page here (`/notes/:id`, `/photos/:id`, …) |
| Here, and not public | Nowhere: no line, so nothing of it goes out |
| From the fediverse | Its own address there |
| From Bluesky | Its page on bsky.app |

On the fediverse the line is a paragraph of the note (`ap.Note.EndWithQuote`),
and Mastodon makes a card of the link. On Bluesky the address alone is the
link (`bsky.OutPost.EndWithQuote`), and the post carries a card of the
quoted post when it carries nothing else, since Bluesky draws no card of
its own, with the quoted post's picture (added the same day, after the
first card on banu.io came without one). Incoming quotes already end with such a line ("↪ bsky.app").

## Part 2: quotes as quotes (done 2026-11-16)

1. **Bluesky (done 2026-11-16):** a quoted post that is on Bluesky, one of
   ours written there or one from there, embedded as a record
   (`bsky.OutPost.WithQuote`: `app.bsky.embed.record`, or `recordWithMedia`
   beside the post's own pictures, video or card), with no line; the line
   and the card for a post that is not. One of ours written there and not
   yet readable is waited for, ten minutes at most, then led to by the line.
2. **The fediverse (done 2026-11-16):** a quote names the quoted post
   (`quote`, with `quoteUri` and `_misskey_quote`), its words ending with
   `RE:` and its address in a `quote-inline` paragraph. For a post here, a
   stamp this installation makes from the two posts, served at
   `/ap/quotes/:uuid`. For a post elsewhere, a `QuoteRequest` to its author,
   asked once; their `Accept` sends the quote again with their stamp, their
   `Reject` without the quote (`activitypub_quote_requests`). Every post
   says who may quote it (`interactionPolicy.canQuote`), by its quote
   policy or its author's. A `QuoteRequest` from elsewhere is answered by
   that policy, with a kept stamp (`activitypub_quote_stamps`) or a
   `Reject`.
3. **Quotes coming in (done 2026-11-16):** kept as quotes, the quoted post
   inside, when approved: a self-quote, a stamp given here, or a stamp
   read from the quoted author's server and checked. The quoted post is
   read and kept once when it is not here; what it quotes in turn is not
   followed. The graph keeps `quote_id` on a post from elsewhere.

## Verification

1. Unit tests: `anahita-libs/federation/quoted_test.go`,
   `activitypub-service/ap/note_quote_test.go`,
   `atproto-service/bsky/outgoing_quote_test.go`, and in
   `atproto-service/cmd/api/handlers/out_test.go` a quote's card with its
   picture, and quotes of posts on Bluesky as records;
   `activitypub-service/cmd/api/handlers/quotes_test.go`, and on MySQL
   `activitypub-service/data/repositories/quotes_integration_test.go` and
   the graph's `TestWhatIsKeptFromAnotherServer`.
2. On banu.io: a note quoting a public post shows the line and its card on
   Mastodon and on Bluesky.
