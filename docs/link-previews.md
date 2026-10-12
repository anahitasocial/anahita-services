# Link previews

A note that has a link in it can carry a card for that link: the page's
title, a line about it, the site it is on and a picture. The card is made
when the note is written and is kept with it, so it reads the same later
even when the page it points to has changed or gone.

## Contents

- [Two halves](#two-halves)
- [Fetching a page safely](#fetching-a-page-safely)
- [What is read from the page](#what-is-read-from-the-page)
- [The picture](#the-picture)
- [How long a preview is kept](#how-long-a-preview-is-kept)
- [Endpoints](#endpoints)
- [The card on a note](#the-card-on-a-note)
- [Running it, and not running it](#running-it-and-not-running-it)
- [What is not built](#what-is-not-built)

## Two halves

| Part | Where | What it does |
| --- | --- | --- |
| `link-preview-service` | port 8101, `/link-previews` | Fetches a page from the internet, reads its title and picture, and keeps the result in Redis |
| `anahita-libs/linkpreviews` | a package | The preview itself, the Redis store, how an address is normalised, and how a card is written into and read out of a note's `meta` |
| `text-service` | `POST` and `PATCH /notes` | Copies a kept preview onto the note |

Only `link-preview-service` connects to addresses people type. It is a
service of its own for that reason: it has no database access, and what it
may connect to is decided in one file, `fetch/safe.go`.

`text-service` never fetches anything and never takes a title or a picture
from the client. It is given an address, looks it up in the store, and
copies what `link-preview-service` put there. A client cannot write a card
that says something the page did not.

## Fetching a page safely

The address comes from a person, so it is treated as hostile.

- `http` and `https` only, on ports 80 and 443 only.
- The name is resolved, and the connection is refused when the address it
  is about to be made to is private, loopback, link-local, multicast,
  unspecified, carrier-grade NAT, or a cloud metadata address. The check is
  on the dialer (`Control`), so it is made on the address actually
  connected to: a name that resolves to something else the second time
  gains nothing.
- The same check runs again for every redirect. At most 3 are followed.
- No proxy from the environment is used.
- It says it is `Anahita/1.0` and no more. Naming the installation's address
  as well, as a crawler usually does, gets the connection dropped by some
  large sites' edges.
- 5 seconds for the whole request.
- At most 1 MB of a page is read, and only `text/html`.
- At most 5 MB of a picture is read.

## What is read from the page

In this order, the first that is there: Open Graph (`og:title`,
`og:description`, `og:image`, `og:site_name`), then `twitter:` tags, then
`<title>` and `<meta name="description">`. Reading stops at the end of
`<head>`.

A page with no title has no preview. Titles are cut to 200 characters and
descriptions to 300.

## The picture

The picture is fetched by the same fetcher, decoded, made no wider than
800 pixels and encoded again, then stored through `file-grpc-service` under
`link-previews/`, named by the SHA-256 of its bytes. What is stored is
always a picture this installation encoded itself.

A reader's browser asks this installation for the picture, never the site
the link points to, so reading a note tells that site nothing.

A preview whose picture could not be fetched or read is kept without one.

## How long a preview is kept

| What | Kept for |
| --- | --- |
| A preview | 7 days |
| That a page could not be read, or said nothing of itself | 1 hour |

Both are in Redis under the SHA-256 of the normalised address (lower-case
scheme and host, no fragment), so the same link pasted by two people is
fetched once. The card on a note is a copy and does not expire.

## Endpoints

| Method and path | Who | What |
| --- | --- | --- |
| `GET /link-previews?url=` | signed in | The preview: `url`, `title`, `description`, `site_name`, `image`. `204` when there is nothing to show, `400` when it is not an http or https address, `429` past 30 a minute |
| `GET /link-previews/image/:name` | anybody | Redirects to the stored picture. `:name` is 64 hexadecimal characters and `.jpg`, `.png`, `.gif` or `.webp`; anything else is `404` |

`image` is a path under the API, `/link-previews/image/<name>`.

## The card on a note

`POST /notes/:owner_id/` and `PATCH /notes/:id` take `link_preview_url`. When that
address is in the note's body and a preview of it is in the store, the
preview is copied into the note's `meta` under `link_preview`. Sent empty,
or with an address that is not in the body, a new note has no card.

An edited note keeps the card it has for as long as its link is still in
the body, whatever is sent, and is given another only when the address sent
has a preview in the store. With the link taken out of the note, the card
goes too.

Wherever the note is sent, it has `link_preview` with the same fields as
above. A repost carries the card of the note it reposts.

## Running it, and not running it

`make link_preview` builds and starts it. It needs Redis and
`file-grpc-service`, and has no settings of its own.

It is optional. Without it, asking for a preview fails, the composer shows
no card, and notes are written without one. Cards already on notes are
still shown; only their pictures need the service.

## What is not built

- oEmbed, and a favicon for a page with no picture.
- Cards on anything but notes, and for any link but the first.
- Blocking previews of chosen domains: it comes with federation's domain
  blocklist.
