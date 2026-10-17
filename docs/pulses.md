# Pulses

A Pulse is a website's feed as a profile to follow, here and from the
fediverse. Any site with an RSS, Atom or JSON feed on a domain its owner
proves is theirs can be connected: a WordPress or other site, a static site,
an app's changelog, a podcast. Each new item of the feed becomes an article
of the Pulse, its title, an excerpt and a link to the original, and people
reply, like, repost and quote it as any post. Nobody posts on a Pulse; its
administrators answer under its posts as the Pulse.

Why it is built this way is in the plan, [plans/pulse.md](plans/pulse.md).

## Contents

- [Who does what](#who-does-what)
- [Connecting a website](#connecting-a-website)
- [Proving a domain](#proving-a-domain)
- [Reading the feed](#reading-the-feed)
- [Its posts](#its-posts)
- [Replies as the Pulse](#replies-as-the-pulse)
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
| `pulse-service` | Connecting a website: finds its feed, gives the token, checks the proof, makes the Pulse. Takes WebSub hubs' calls. The only service besides link-preview-service that fetches addresses people type, and it goes only to public addresses on ports 80 and 443 (`anahita-libs/safehttp`) |
| `pulse-worker` | Reads every Pulse's feed and posts what is new; looks at every proof again once a day; pauses and starts Pulses again. Answers no requests |
| `actor-service` | Serves the Pulse as a profile under `/pulses/`, as it serves groups: read, change, administrators, delete |
| `text-service` | Under a Pulse's post, makes a reply by one of its administrators the Pulse's |
| `activitypub-service` | The Pulse as an `Organization`, always on the fediverse; its articles as `Article`s |

A Pulse's node is `node.actor.actor-service.pulse.v1`, its service
`actor-service`. Both services are optional: left out, there are no Pulses,
and nothing else changes.

## Connecting a website

"Connect your website", never "create". In the web app at `/pulses/connect`,
for somebody `PULSES_FROM` allows (administrators unless the installation
says otherwise), in three calls:

1. `POST /pulses/_connect` with the site's address (`example.com`, or its
   feed's). The feed is the address itself, or the first feed the page names
   in its head with `<link rel="alternate">`. It must be on the site's domain
   or under it: `blog.example.com` is example.com's, `feeds.example.net` is
   not. A feed that already has a Pulse is refused. The answer is the token,
   where to put it, and a name and handle to begin with: the feed's title,
   and the domain's first part (`@example`).
2. `POST /pulses/_connect/:id/check` looks for the token, as often as asked.
3. `POST /pulses/_connect/:id` with a name, a handle and a description makes
   the Pulse: public, with whoever proved the domain as its first
   administrator and first follower. What the feed holds now is marked seen,
   and only what is published afterwards is posted.

A connection is the person's who began it: anybody else is told it is not
there.

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

Every 30 minutes (`PULSE_READ_EVERY`), with the `ETag` and `Last-Modified`
the server sent the last time, so an unchanged feed is not sent again. At
most 5 MB, 10 seconds and 3 redirects. A feed that fails is read again 15
minutes later, then 30, an hour, and so on to once a day; how often it has
failed and why are kept in `pulses`. A Pulse is taken by one worker at a
time, so more than one may run.

## Its posts

Each new item becomes an article owned by the Pulse:

| Of the article | From the item |
| --- | --- |
| Title | Its title, or the opening of its text when it has none (100 characters) |
| Content | Its excerpt (the first 300 characters of its text, plain, cut at a word), the original's address on a line of its own, and its categories as hashtags, one word each (`Open Science` is `#OpenScience`), five at most |
| Card | The original's, from link-preview-service |
| Date | The item's, unless it says the future |
| Audience | Public |

It is made as the Pulse's first administrator. At most 10 an hour
(`PULSE_HOURLY_CAP`), oldest first; the rest wait for the next hour. Items
are known by their id, or their address when they have none, and none is
posted twice. One whose title or excerpt changes changes its article; one
that leaves the feed leaves its article as it is, since feeds seldom say
that something was deleted.

## Replies as the Pulse

Under a Pulse's own post, a reply in words by one of its administrators is
the Pulse's: its owner and author are the Pulse, so the site answers its
readers as itself. Which administrator wrote it is kept in the reply as
`written_by`, never sent with it; a report about it shows it to those who
handle reports. The Pulse's replies are changed by whoever replies as it.
Anywhere else, and by anybody else, a reply is its writer's.

## On the fediverse

A Pulse is on the fediverse whenever the installation is, with nobody to
turn it on: WebFinger finds it by its handle (`@garden@example.org`), and it
is an `Organization`, not a `Service`, which Mastodon marks as automated.
Its articles go out as `Article`: the title as `name`, the excerpt as
`summary`, the content, and the article's page as `url`, attributed to the
Pulse. Replies to them come back into its threads, and its own replies go
out, as for a person's notes ([Federation](federation.md)). It is not on
Bluesky.

## WebSub

A feed that names a hub (`rel="hub"`) is subscribed to, its own address
(`rel="self"`) as the topic, for ten days and asked again before they end.
The hub calls back at `/pulses/_websub/:id/:signature`, the signature an HMAC
of the Pulse's id with `SECURE_COOKIE_KEY`: a check is answered with its
challenge, and a push brings the next read forward to now, at most once a
minute. What a hub sends is not read; the feed is always read from the site.
Only where a hub could call back: an installation reachable from the
internet over https. WordPress names a hub only with a plugin.

## Pausing and starting again

| Paused when | Started again when |
| --- | --- |
| The domain's proof has been gone a week (`proof_gone`) | It is found again, at the next daily look |
| The Pulse is switched off, deleted (in its grace period) or archived (`profile_gone`) | It is switched on or restored; looked at once an hour |

A paused Pulse's feed is not read.

## Settings

| Variable | Read by | Default | Means |
| --- | --- | --- | --- |
| `PULSES_FROM` | pulse-service, auth-service (NodeInfo `pulsesFrom`) | `administrators` | The least role that may connect a website: `registered`, `administrators` or `super-administrators`. Give both the same value |
| `PULSE_HOURLY_CAP` | pulse-worker | 10 | The most posts a Pulse makes in an hour |
| `PULSE_READ_EVERY` | pulse-worker | 30 | How often a feed is read, in minutes; 15 at least |
| `LINK_PREVIEW_URL` | pulse-worker | `http://link-preview-service:80` | Where the original's card is asked for; unset, articles have none |
| `SECURE_COOKIE_KEY`, `DOMAIN_NAME`, `PUBLIC_*` | pulse-service, pulse-worker | | The key the WebSub signature is made with, and the address hubs call back at |

## The tables

All three are pulse-service's (migrations `20261120000001` and
`20261121000001`).

| Table | Holds |
| --- | --- |
| `verified_domains` | A domain being proved or proved: who proves it, for which profile, the token, how and when it was proved, last looked at, failing since |
| `pulses` | One row per Pulse: its feed (unique), its site, its domain, its state, what the feed's server said last, the WebSub hub, when it is read next, failures, this hour's posts |
| `pulse_items` | Every item seen, by its key, with the article it became and a hash of its title and excerpt: `seen`, `waiting` or `posted` |

## Endpoints

| Endpoint | Who | What |
| --- | --- | --- |
| `POST /pulses/_connect` | `PULSES_FROM` | The site's feed, and the token |
| `POST /pulses/_connect/:id/check` | whoever began it | `{ proved, method }` |
| `POST /pulses/_connect/:id` | whoever began it | The Pulse: `{ id, alias }` |
| `GET`, `POST /pulses/_websub/:id/:signature` | a hub | A check, and a push |
| `GET /pulses/`, `GET /pulses/:id` | anybody | The Pulses, and one (actor-service) |
| `PATCH /pulses/:id` | its administrators | Its name and description; its handle and site stay as connected |
| `DELETE /pulses/:id`, `POST /pulses/:id/restore`, `/admins/` | as for a group | |

Refusals say why as `error`: `not_a_domain`, `no_feed`, `unreadable`,
`feed_not_on_domain` (400); `feed_taken`, `handle_taken`, `not_proved` (409).
The full reference is [`docs/api/`](api/index.html#pulse-service).

## What is not built

- A reply with pictures, a GIF or a video by an administrator under a
  Pulse's post is still their own, since its files are uploaded for them.
- An administrator is not told when their Pulse is paused, or why.
- A Pulse is not suggested in recommendations.

## Where the code is

| What | Where |
| --- | --- |
| Proving a domain | `anahita-libs/domains` |
| Connecting a website, WebSub calls | `pulse-service/cmd/api` |
| Finding and reading feeds | `pulse-service/feeds` |
| The worker | `pulse-service/worker`, `pulse-service/cmd/worker` |
| WebSub | `pulse-service/websub` |
| The Pulse as a profile | `actor-service/cmd/api/handlers/pulse.go` |
| Replies as the Pulse | `anahita-libs/cmd/api/permissions/reply.go` (`RepliesAs`), `anahita-libs/cmd/api/handlers/reply_owner.go` |
| On the fediverse | `activitypub-service` (`isPulse`, `isPulseArticle`, `AsArticle`) |
| The web app | `src/containers/pulses`, `src/api/pulseConnect.js` (anahita-web) |
