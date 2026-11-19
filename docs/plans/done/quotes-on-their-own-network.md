# Quotes on their own network (roadmap row 162)

**Done 2026-11-19**, all of it, in services alone: the web app asks the
server whether Quote is offered, and needed no change.

## Context

The user, 2026-11-19: somebody on Mastodon who lets a post be quoted has
consented to a quote on the fediverse, not to their post being carried to
Bluesky, though it is public; and the same the other way round. Anahita is
on both, so a quote is always kept here; past that, it goes only to the
quoted post's own network. A post crosses from one network to the other
only when somebody pastes its address into a post themselves.

Until now (row 156, `docs/federation.md`, Quotes) a quote went to every
network its author is on, and where the quoted post was not, it ended with
"↪" and the quoted post's address: on Bluesky with a card of it (its
words, its author's name and handle, its first picture), sent whether or
not its author on Mastodon had let the quote; on the fediverse as a link
Mastodon makes a card of. A post here that its author keeps off a network
reached that network the same way, through somebody else's quote.

## What changes

A quote goes out to a network only when the post it quotes is on that
network, by its author's choice:

| The quoted post | The quote goes to |
| --- | --- |
| 1. From the fediverse | The fediverse, as a quote by its author's consent, as now. Not to Bluesky |
| 2. From Bluesky | Bluesky, as Bluesky's own quote, as now. Not to the fediverse |
| 3. Here, and on both networks | Both, as a quote on each, as now |
| 4. Here, and on one network | That network alone |
| 5. Here, and on neither, or not public | Neither: the quote stays here |

Always within the networks the quoting person is on. Kept as they are:

1. A quote here is shown as a quote here in every case.
2. A post's address pasted into its words is a link with its card, and goes
   wherever the post goes.
3. Quotes coming in are kept as they are now.

Taken out: the "↪" line and the Bluesky card of a quoted post
(`EndWithQuote`, `quotedBy`, `quotedPicture`, the card in
`atproto-service/cmd/api/handlers/out.go`), and the line in
`ap.Note.EndWithQuote` for a post not on the fediverse. The `quote-inline`
"RE:" paragraph stays: it is part of a quote on the fediverse.

## Bluesky's own consent

A Bluesky author can turn quoting off (`app.bsky.feed.postgate`, an
`embeddingRules` `disableRule`, the record named as the post), and can
detach one quote (`detachedEmbeddingUris`). Not read until now. With this
row:

1. A post from Bluesky has its postgate read as Jetstream tells of it
   (`app.bsky.feed.postgate`, beside posts); quoting turned off makes its
   quote policy here "nobody", and Quote is not offered, as for a post
   here. A post read when its author is first followed is not looked up
   for one.
2. A quote already made of a post whose author turns quoting off, or
   detaches it, keeps its words here and on Bluesky, as Bluesky shows it,
   without the quoted post.

## Where it is decided

1. **atproto-service** (`out.go`): a quote is not sent when the quoted
   post is not on Bluesky (`quotedRef` nil once `quotedWait` is past); a
   quoted post written here and still being sent is waited for, as now.
2. **activitypub-service** (`notes.go`, `withQuote`): a quote is not sent
   when the quoted post is from Bluesky, or here and not sent to the
   fediverse.
3. Neither service asks the other: each knows from the quoted post's
   address (`https://` or `at://`) and from what it sent of it itself.
4. A quote changed here afterwards follows the same rule.

## Consent on the fediverse, before anything goes out

Until now a quote of a fediverse post went out at once, naming the quoted
post, with a `QuoteRequest` sent beside it; Mastodon showed it as pending,
and a `Reject` sent it again without the quote. The quoted post never
showed without consent, but a quote had gone out before the answer. And
the policy every Mastodon post carries (`interactionPolicy.canQuote`:
anyone, followers, its author alone) was written on posts here and never
read on posts that came in. With this row:

1. **The policy is read when a post comes in** and kept as its quote
   policy here. A post its policy does not let a person quote is not
   offered to them to quote, as for a post here.
2. **Asked first, then sent.** The quote goes to the fediverse only after
   the author's `Accept`, with their stamp; usually seconds, as Mastodon
   answers by the author's setting. A quote whose author's server never
   answers never goes out: no answer is not consent.
3. **On a `Reject`** nothing goes to the fediverse, and here the quote
   keeps its words without the quoted post (taken out as an author takes
   theirs out). Its writer is not told.

The composer says nothing of where a quote goes (the user, 2026-11-19):
the rules are clear and followed, and a person can do nothing other than
respect the consent, so there is nothing to tell them before they post.

## Docs

1. anahita-services `docs/federation.md`: Quotes (the table), Posts going
   out, and the Bluesky paragraph on quoting; `docs/permissions.md`, Quotes,
   for the postgate.
2. anahita-web: nothing; Quote is offered as the server says.
3. anahita.io: a new blog post when this is built, saying that we changed
   our mind and refined the design (the user, 2026-11-19). The post
   "Quotes that reach Mastodon and Bluesky" stays as it was written; it says
   a declined quote goes out without the quote, which this row changes.
   Its two screenshots, of a channel's post rather than of a quote, were
   taken out the same day.

## Estimate

1–2 sessions, services and web.
