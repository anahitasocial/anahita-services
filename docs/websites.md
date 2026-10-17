# Websites

A website, as a kind of profile here, is a site's feed to follow, here and
from the fediverse. Any site with an RSS, Atom or JSON feed on a domain its owner
proves is theirs can be connected: a WordPress or other site, a static site,
an app's changelog, a podcast. Each new item of the feed becomes a note of
the website, its title, an excerpt and a link to the original, and people
reply, like, repost and quote it as any post. Nobody posts on a website; its
administrators answer under its posts as the website.

Why it is built this way is in the plan, [plans/website.md](plans/website.md).

## Contents

- [Who does what](#who-does-what)
- [Connecting a website](#connecting-a-website)
- [Proving a domain](#proving-a-domain)
- [Reading the feed](#reading-the-feed)
- [Its posts](#its-posts)
- [Replies as the website](#replies-as-the-website)
- [On the fediverse](#on-the-fediverse)
- [WebSub](#websub)
- [Pausing and starting again](#pausing-and-starting-again)
- [Settings](#settings)
- [The tables](#the-tables)
- [Endpoints](#endpoints)
- [What is not built](#what-is-not-built)
- [Where the code is](#where-the-code-is)

## Who does what

| Piece | Does |
| --- | --- |
| `syndication-service` | Connecting a website: finds its feed, gives the token, checks the proof, makes the website. Takes WebSub hubs' calls. The only service besides link-preview-service that fetches addresses people type, and it goes only to public addresses on ports 80 and 443 (`anahita-libs/safehttp`) |
| `syndication-worker` | Reads every website's feed and posts what is new; looks at every proof again once a day; pauses and starts websites again. Answers no requests |
| `actor-service` | Serves the website as a profile under `/websites/`, as it serves groups: read, change, administrators, delete |
| `text-service` | Under a website's post, makes a reply by one of its administrators the website's |
| `activitypub-service` | The website as an `Organization`, always on the fediverse, and its notes |

A website's node is `node.actor.actor-service.website.v1`, its service
`actor-service`. Both services are optional: left out, there are no websites,
and nothing else changes.

## Connecting a website

"Connect your website", never "create". In the web app at `/websites/connect`,
for somebody `WEBSITES_FROM` allows (administrators unless the installation
says otherwise), in three calls:

1. `POST /websites/_connect` with the site's address (`example.com`, or its
   feed's). The feed is the address itself, or the first feed the page names
   in its head with `<link rel="alternate">`. It must be on the site's domain
   or under it: `blog.example.com` is example.com's, `feeds.example.net` is
   not. A feed that already has a website is refused. The answer is the token,
   where to put it, and a name and handle to begin with: the feed's title,
   and the domain's first part (`@example`).
2. `POST /websites/_connect/:id/check` looks for the token, as often as asked.
3. `POST /websites/_connect/:id` with a name, a handle and a description makes
   the website: public, with whoever proved the domain as its first
   administrator and first follower. The feed's ten newest items are posted
   at once, each with its own date, so that a new website does not open empty;
   the rest are marked seen, and from then on only what is published is
   posted.

A connection is the person's who began it: anybody else is told it is not
there.

**A private feed is refused** (`private_feed`): one behind a login, which
answers 401 or 403, and one whose address carries a secret, a user and
password, or a parameter such as `token`, `key`, `secret` or `auth` (a paid
podcast's feed, a private repository's). A website's posts are public and go
to the fediverse, where they cannot be called back, so what a feed keeps for
some is never made public here.

## Proving a domain

The token, `anahita-` and 32 hexadecimal characters, goes in either place:

| Where | What |
| --- | --- |
| A TXT record | `_anahita.<domain>`, holding the token |
| A file | `https://<host>/.well-known/anahita-verification.txt`, the token on a line of its own |

The domain is the site's host without a leading `www.`; the file is read from
the host as given. The record is looked at first. The proof is
`anahita-libs/domains`, built to be shared: people and groups are to prove
domains the same way ([roadmap row 63](plans/verified-domains.md)).

## Reading the feed

Every 30 minutes (`WEBSITE_READ_EVERY`), with the `ETag` and `Last-Modified`
the server sent the last time, so an unchanged feed is not sent again. At
most 5 MB, 10 seconds and 3 redirects. A feed that fails is read again 15
minutes later, then 30, an hour, and so on to once a day; how often it has
failed and why are kept in `websites`. A website is taken by one worker at a
time, so more than one may run.

## Its posts

Each new item becomes a note of the website (the user, 2026-10-16: a site's
item, shown here, is a short post pointing elsewhere, as a person sharing a
link is, and not writing done here):

| Of the note | From the item |
| --- | --- |
| Its words | The item's title in bold, as a note has no title of its own; its excerpt (the first 300 characters of its text, plain, cut at a word); the original's address; its categories as hashtags, one word each (`Open Science` is `#OpenScience`), five at most. What Markdown would read as marks in the title or the excerpt is kept as written |
| Card | The original's, from link-preview-service: the one card a note carries |
| Date | The item's, unless it says the future |
| Audience | Public |

Its author is the website, so it is shown and sent as the website's; it is
made as the website's first administrator, who is kept in it as
`written_by`, as for its replies. At most 10 an hour (`WEBSITE_HOURLY_CAP`),
oldest first; the rest wait for the next hour. Items are known by their id,
or their address when they have none, and none is posted twice. One whose
title or excerpt changes changes its note; one that leaves the feed leaves
its note as it is, since feeds seldom say that something was deleted.

**A website's own notes follow its site.** Nobody changes their words or
narrows who sees them, its administrators and site administrators included:
both are refused with `follows_its_website`, and the web app offers neither.
They are deleted, and replies under them hidden, as any post's; who may
reply and quote stays the administrators' to choose.

## Replies as the website

Under a website's own post, a reply in words by one of its administrators is
the website's: its owner and author are the website, so the site answers its
readers as itself. Which administrator wrote it is kept in the reply as
`written_by`, never sent with it; a report about it shows it to those who
handle reports. The website's replies are changed by whoever replies as it.
Anywhere else, and by anybody else, a reply is its writer's.

## On the fediverse

A website is on the fediverse whenever the installation is, with nobody to
turn it on: WebFinger finds it by its handle (`@garden@example.org`), and it
is an `Organization`, not a `Service`, which Mastodon marks as automated.
Its notes go out as `Note`s, the website's, with their card, which is how
Mastodon shows a post best: an `Article` would be shown there as its title
and a link. Replies to them come back into its threads, and its own replies
go out, as for a person's notes ([Federation](federation.md)). It is not on
Bluesky.

## WebSub

A feed that names a hub (`rel="hub"`) is subscribed to, its own address
(`rel="self"`) as the topic, for ten days and asked again before they end.
The hub calls back at `/websites/_websub/:id/:signature`, the signature an HMAC
of the website's id with `SECURE_COOKIE_KEY`: a check is answered with its
challenge, and a push brings the next read forward to now, at most once a
minute. What a hub sends is not read; the feed is always read from the site.
Only where a hub could call back: an installation reachable from the
internet over https, and open to read: a members-only site refuses hubs. WordPress names a hub only with a plugin.

## Pausing and starting again

| Paused when | Started again when |
| --- | --- |
| The domain's proof has been gone a week (`proof_gone`) | It is found again, at the next daily look |
| The website is switched off, deleted (in its grace period) or archived (`profile_gone`) | It is switched on or restored; looked at once an hour |

A paused website's feed is not read.

## Settings

| Variable | Read by | Default | Means |
| --- | --- | --- | --- |
| `WEBSITES_FROM` | syndication-service, auth-service (NodeInfo `websitesFrom`) | `administrators` | The least role that may connect a website: `registered`, `administrators` or `super-administrators`. Give both the same value |
| `WEBSITE_HOURLY_CAP` | syndication-worker | 10 | The most posts a website makes in an hour |
| `WEBSITE_READ_EVERY` | syndication-worker | 30 | How often a feed is read, in minutes; 15 at least |
| `LINK_PREVIEW_URL` | syndication-worker | `http://link-preview-service:80` | Where the original's card is asked for; unset, notes have none |
| `SECURE_COOKIE_KEY`, `DOMAIN_NAME`, `PUBLIC_*` | syndication-service, syndication-worker | | The key the WebSub signature is made with, and the address hubs call back at |

## The tables

All three are syndication-service's (migrations `20261120000001` and
`20261121000001`).

| Table | Holds |
| --- | --- |
| `verified_domains` | A domain being proved or proved: who proves it, for which profile, the token, how and when it was proved, last looked at, failing since |
| `websites` | One row per website: its feed (unique), its site, its domain, its state, what the feed's server said last, the WebSub hub, when it is read next, failures, this hour's posts |
| `website_items` | Every item seen, by its key, with the note it became and a hash of its title and excerpt: `seen`, `waiting` or `posted` |

## Endpoints

| Endpoint | Who | What |
| --- | --- | --- |
| `POST /websites/_connect` | `WEBSITES_FROM` | The site's feed, and the token |
| `POST /websites/_connect/:id/check` | whoever began it | `{ proved, method }` |
| `POST /websites/_connect/:id` | whoever began it | The website: `{ id, alias }` |
| `GET`, `POST /websites/_websub/:id/:signature` | a hub | A check, and a push |
| `GET /websites/`, `GET /websites/:id` | anybody | The websites, and one (actor-service) |
| `PATCH /websites/:id` | its administrators | Its name and description; its handle and site stay as connected |
| `DELETE /websites/:id`, `POST /websites/:id/restore`, `/admins/` | as for a group | |

Refusals say why as `error`: `not_a_domain`, `no_feed`, `unreadable`,
`feed_not_on_domain`, `private_feed` (400); `feed_taken`, `handle_taken`, `not_proved` (409).
The full reference is [`docs/api/`](api/index.html#syndication-service).

## What is not built

- A reply with pictures, a GIF or a video by an administrator under a
  website's post is still their own, since its files are uploaded for them.
- An administrator is not told when their website is paused, or why.
- A website is not suggested in recommendations.

## Where the code is

| What | Where |
| --- | --- |
| Proving a domain | `anahita-libs/domains` |
| Connecting a website, WebSub calls | `syndication-service/cmd/api` |
| Finding and reading feeds | `syndication-service/feeds` |
| The worker | `syndication-service/worker`, `syndication-service/cmd/worker` |
| WebSub | `syndication-service/websub` |
| The website as a profile | `actor-service/cmd/api/handlers/website.go` |
| Replies as the website | `anahita-libs/cmd/api/permissions/reply.go` (`RepliesAs`), `anahita-libs/cmd/api/handlers/reply_owner.go` |
| On the fediverse | `activitypub-service` (`isWebsite`) |
| A website's notes follow its site | `anahita-libs/cmd/api/permissions/medium.go` (`IsWebsiteItem`) |
| The web app | `src/containers/websites`, `src/api/websiteConnect.js` (anahita-web) |
