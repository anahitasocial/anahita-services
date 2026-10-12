# Federation

How people here are found and followed from other servers of the fediverse,
and what an installation has to do for that to happen.

**This is being built.** What is described here is what works today: a
person can choose to be found, and another server can find and describe
them. Following, and posts going out and coming in, are not built yet.

## Contents

- [What it is](#what-it-is)
- [Turning it on](#turning-it-on)
- [Who is on the fediverse](#who-is-on-the-fediverse)
- [What a person is called](#what-a-person-is-called)
- [What other servers are told](#what-other-servers-are-told)
- [Signing keys](#signing-keys)
- [The API](#the-api)
- [Turning it off](#turning-it-off)
- [Where the code is](#where-the-code-is)

## What it is

Mastodon, GoToSocial, Pixelfed and many others speak one protocol,
ActivityPub, and somebody on any of them can follow somebody on any other.
`activitypub-service` makes this installation one of them. It is a service
of its own because it is the one that reads what the internet sends it, and
because an installation may not want it at all.

Bluesky speaks a different protocol and will be a different service.

## Turning it on

Three things, all of them the installation's:

1. **Run `activitypub-service`.** It is in `docker-compose.yml` and in
   `k8s/base`. Left out, nobody here is on the fediverse and nothing else
   changes.
2. **Be reachable.** Other servers have to reach the API at a name, over
   https: see `PUBLIC_DOMAIN`, `PUBLIC_WEB_URL` and `PUBLIC_API_URL` in
   [Configuration](configuration.md#site-identity). On a developer's machine
   these say `localhost`, and the service says at its start that the
   installation is not on the fediverse; [the lab](federation-lab.md) is how
   it is tried there.
3. **Be open to read.** With `SITE_READ_ACCESS` at `preview` or
   `registered`, what is on the site is kept from visitors, and federating
   would send it out all the same. So only a site that is `public`
   federates ([Privacy](privacy.md)).

And one thing on the web app's host: it has to send what other servers ask
it under `/.well-known` on to the API. `web-shell` does; see
[The web shell](web-shell.md#what-other-servers-ask-this-host).

**Choose the names before anybody is told them.** What a person is called on
other servers is made of the handle domain and the API's address, and those
servers keep it for good. Change either afterwards and everybody here
becomes somebody new there, with no followers.

## Who is on the fediverse

Nobody, until they say so. What is sent to another server cannot be called
back: it is copied by servers this one has never heard of.

A person is on the fediverse when all of these are so:

- they turned it on for themselves;
- they are a person, made here, enabled, and not on their way to being
  deleted;
- their profile is open to everybody (`public`): not to members only, and
  not to followers;
- the graph agrees that a visitor may see them.

The moment one of them stops being so, they are not found: every address of
theirs answers `404`, in the same words as an address nobody has. A server
is never told *why* somebody is not there.

Groups and events are not on the fediverse yet.

## What a person is called

| | Example | Made of |
| --- | --- | --- |
| Their handle | `ana@example.org` | Their username and `PUBLIC_DOMAIN` |
| Their address | `https://api.example.org/ap/users/0962789f-…` | `PUBLIC_API_URL` and the node's UUID |

The handle is what people type. The address is what servers keep, and it is
made of the UUID so that it never changes: a username can be changed, and a
person who changed theirs must still be the same person to everybody who
follows them.

## What other servers are told

No more than the person's public profile already shows a visitor.

| Address | Answers |
| --- | --- |
| `/.well-known/webfinger?resource=acct:ana@example.org` | Where the person is: their address, and their page. Also answered for their address and for their page's |
| `/.well-known/host-meta` | Where WebFinger is, for servers that ask the older way |
| `/ap/users/<uuid>` | Who they are: name, username, what they say of themselves as HTML that is safe to show, their avatar and cover at [addresses that stay](architecture.md#what-a-node-is-called-elsewhere), when they joined, whether a follow waits to be approved, and the public half of their signing key |
| `/ap/users/<uuid>/followers`, `/following` | How many, and never who |

## Signing keys

Whatever is sent in somebody's name is signed with a key of theirs, and the
other server checks it against the public half, which it reads from their
description.

- A key is made when a person turns federation on (RSA, 2048 bits, which is
  what every server understands).
- The private half is kept in `actor_keys`, encrypted by the means
  auth-service keeps its secrets: `CRYPT_BACKEND` and `CRYPT_LOCAL_KEY`
  ([Configuration](configuration.md)). **Losing that key loses every
  person's signing key,** and with it their followers elsewhere: it is in
  the backups an installation cannot do without.
- Everybody is told the same key. A server that was told one will refuse
  what is signed with another.

## The API

Signed in, for the person themselves:

| Request | Does |
| --- | --- |
| `GET /activitypub/settings` | Whether they are on the fediverse, whether they could be, and what they are or would be called |
| `PUT /activitypub/settings` `{ "enabled": true }` | Turns it on or off |

~~~json
{
  "available": true,
  "enabled": false,
  "handle": "ana@example.org",
  "actor": "https://api.example.org/ap/users/0962789f-aeca-4850-ac9b-41129e3b6f46"
}
~~~

When `available` is false, `reason` says why:

| `reason` | Means |
| --- | --- |
| `site_private` | The site is for members, or shows visitors a preview |
| `site_unreachable` | The site has no address another server could reach |
| `profile_not_public` | Their profile is not open to everybody |

Turning it on when it is not available is refused with `409` and the reason.
Turning it off is always allowed.

## Turning it off

A person who turns it off is not found from that moment. Servers that
already know them keep what they have until they next ask. Telling those
servers to forget is part of what is not built yet.

An installation that stops running the service is the same for everybody on
it.

## Where the code is

| What | Where |
| --- | --- |
| The service | `activitypub-service/` |
| Who is on the fediverse | `cmd/api/handlers/handlers.go`: `siteFederates`, `mayFederate`, `federating` |
| WebFinger and host-meta | `cmd/api/handlers/discovery.go` |
| A person's description | `cmd/api/handlers/actor.go`, `ap/actor.go` |
| The addresses of things | `ap/addresses.go` |
| Keys | `keys/`, `cmd/api/handlers/keys.go`, table `actor_keys` |
| A person's choice | `cmd/api/handlers/settings.go`, table `activitypub_accounts` |
| The gateway's routes | `nginx/nginx.conf`: `/.well-known/webfinger`, `/.well-known/host-meta`, `/ap/`, `/activitypub/` |
| What it shares with the Bluesky service to come | `anahita-libs/federation`, `anahita-libs/site` |
