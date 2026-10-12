# Federation

How people here are found and followed from other servers of the fediverse,
and what an installation has to do for that to happen.

**This is being built.** What is described here is what works today: a
person can choose to be found, people follow each other across servers
either way round, and their public notes travel with the follow. The web
app cannot yet show somebody from another server or what they post, so
none of it can be seen there yet. Replies, likes, reposts, pictures and
profile changes do not travel yet.

## Contents

- [What it is](#what-it-is)
- [Turning it on](#turning-it-on)
- [Who is on the fediverse](#who-is-on-the-fediverse)
- [What a person is called](#what-a-person-is-called)
- [What other servers are told](#what-other-servers-are-told)
- [Looking somebody up on another server](#looking-somebody-up-on-another-server)
- [Being followed from another server](#being-followed-from-another-server)
- [Following somebody on another server](#following-somebody-on-another-server)
- [Posts going out](#posts-going-out)
- [Posts coming in](#posts-coming-in)
- [The inbox](#the-inbox)
- [What is sent, and when it does not arrive](#what-is-sent-and-when-it-does-not-arrive)
- [Signing keys and signed requests](#signing-keys-and-signed-requests)
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
| `/ap/actor` | The installation itself, which signs what it asks of other servers in nobody's name. It has a key and nothing else: nobody follows it and it posts nothing |

WebFinger answers for a handle on the API's host as well as on the handle's
own domain (`ana@api.example.org` as well as `ana@example.org`), and says in
its answer which is the real one. A server that first met somebody by their
address asks the host in that address who they are.

## Looking somebody up on another server

A person here types a handle, `@alice@mastodon.example`, or pastes the
address of a profile. The service asks that server who it is and keeps them
as a person of another server: a node of type
`node.actor.activitypub-service.person.v1`, with their address as its `uri`
([architecture](architecture.md#what-a-node-is-called-elsewhere)), and a row
in `activitypub_actors` with where to write to them and the key they sign
with.

**What another server says is believed only about itself.** A description is
kept only when the address it gives for itself is on the server it was
fetched from, its key is its own, and its inbox is an https address. It is
never believed about this installation. What somebody says of themselves
arrives as HTML and is kept as Markdown with nothing in it that runs.

**It goes only where it is safe to.** The service is told where to go by
people and by other servers, so it fetches through the same guarded client
as link previews (`anahita-libs/safehttp`): https only, public addresses
only, decided at the moment of connecting, a megabyte at most.

A lookup is limited to 30 in ten minutes for each person.

## Being followed from another server

Somebody on another server follows somebody here by sending a `Follow` to
the inbox.

| The person here | What happens |
| --- | --- |
| Is on the fediverse | They are followed at once: an ordinary follow in the graph, from the node kept for the person elsewhere. The other server is sent an `Accept` |
| Has blocked whoever asks | Nothing is made, and the other server is sent a `Reject` |
| Is not on the fediverse | Nothing, and nothing is said |

**Nobody on the fediverse approves their followers.** Only a profile open
to everybody is on it, and such a profile is followed by whoever chooses
to, from here or from elsewhere. Letting people *ask* to follow is for a
profile that is not open to everybody, and such a profile does not
federate. So a person's description always says a follow does not wait.

Taking a follow back (`Undo` of the `Follow`) removes it, or the request.
Only whoever followed can take their own follow back.

What the other server calls each follow is kept in `activitypub_follows`,
because accepting it or undoing it means quoting it back.

A person followed from elsewhere is told as they are of any follower.
**The web app does not yet know how to show somebody from another server,**
so until it does such a notification has a name and no profile behind it.

## Following somebody on another server

Somebody here follows a person kept from another server the way they follow
anybody: the same button, the same request to `socialgraph-service`.
Nothing there was changed. `activitypub-service` hears on the bus that it
happened and asks the other server, with a signed `Follow`.

| Then | What happens here |
| --- | --- |
| The other server says yes (`Accept`) | The follow stands. It is recorded as accepted |
| It says no (`Reject`), then or later | The follow here is taken away |
| It says nothing yet (the person there approves their followers) | The follow is there and recorded as pending. What that person posts will not arrive until they say yes |
| The person here unfollows | The other server is sent an `Undo`, quoting the follow |

- **Only somebody who is on the fediverse can follow across it.** The other
  server has to be able to find out who is asking. A follow of somebody
  elsewhere by a person who is not, is taken away again as soon as it is
  heard of; the web app is not to offer it.
- **An answer is matched to the question.** A yes or a no does something
  only when it is from the person who was asked, about a follow this
  installation is waiting on, by the name that follow was sent under. Nobody
  can be made to follow somebody by being told they were accepted.
- **The bus can lose a message.** A follow the other server was never asked
  about stays as it is here and carries nothing. Unfollowing and following
  again puts it right; a sweep that does so by itself is not written yet.

## Posts going out

What is sent to another server cannot be called back, so what is sent is
narrow, and is asked of the post as it is at that moment:

- a **note**, written here, by somebody who is on the fediverse;
- on **their own profile**: not in a group, not on somebody else's;
- **open to everybody**: not to members, not to followers;
- **not a reply**;
- and a visitor may see it, by the graph's own reckoning.

Such a note goes to every server that has a follower of its author, once
for each server. It is sent as HTML made from its Markdown, with nothing in
it that runs; the hashtags and people it names are links, and are listed
beside the text; its language is said when its author said one. It can also
be fetched, at `/ap/notes/<uuid>`.

| Then | Other servers are sent |
| --- | --- |
| The note is changed | An `Update`, with the note as it is now |
| It stops being one that goes out (made for followers only, say) | A `Delete`: it is taken back |
| It is deleted | A `Delete` |

A note that never went out is deleted at nobody. Which notes went out is
kept in `activitypub_objects`: a deleted note's node is gone by the time
anybody hears of it, and that table is what still knows what it was called.

**Not sent:** articles, topics, photos and videos; anything in a group;
replies; a quoted post (the note that quotes goes, without what it quotes);
the card for a link. Somebody who starts following is sent what is posted
from then on, and nothing from before.

## Posts coming in

A note from another server is kept when all of these are so:

- **somebody here follows whoever wrote it.** This installation is not a
  copy of the fediverse: what nobody here asked for is not stored;
- it is **its sender's own**, at an address on their own server;
- it is **addressed to everybody**;
- it is **not a reply**, and has some text.

It is kept as an ordinary note, owned by the node kept for its author, with
its address as its `uri`. Its HTML is kept as Markdown with nothing in it
that runs, at most 20,000 characters, dated when its author says it was
written, in the language it says. Because it is an ordinary note on the
profile of somebody followed, it is in the feed of whoever follows them
with nothing changed in the feeds.

Changed there, it is changed here; taken away there, it is taken away here,
and only by its author: the graph removes a note from another server only
in the name of whoever it belongs to
(`graph-grpc-service/data/repositories/nodes_remote.go`).

**Not kept:** pictures and other attachments, replies, boosts, polls, and
what was posted before the follow.

## The inbox

`POST /inbox`, and `POST /ap/users/<uuid>/inbox` for each person: the same
door. It is the one place in the installation where anybody on the internet
may send anything, so nothing that arrives is acted on, or read for what it
says, until it is known who sent it:

1. **It must be signed,** and the signature must check against the key of
   whoever it names, read from that person's own server (or kept from the
   last time, and read again if the kept one no longer checks).
2. **Whoever the message says did the thing must be whoever signed it.**
   Nobody is believed about what somebody else did.
3. **Only then is it read,** and only what this installation understands is
   done: `Follow` and its `Undo`; `Accept` or `Reject` of a follow asked
   for from here; `Create`, `Update` and `Delete` of a note. Everything
   else is answered `202` and ignored.

A message of more than a megabyte is refused. What cannot be checked is
answered `401`, and nothing is kept of whoever sent it.

## What is sent, and when it does not arrive

What is to be sent to another server is written to a table first
(`activitypub_deliveries`) and sent from there, signed by the person it is
sent for. If the other server is down it is tried again after 1, 4, 16, 64
and 256 minutes and then every six hours, for two days; if the other server
says the inbox is gone, or that what was sent is wrong, it is given up at
once. What is given up is kept in the table with the reason.

~~~sql
SELECT target, attempts, last_error, created_at
FROM activitypub_deliveries
WHERE status = 'failed'
ORDER BY created_at DESC;
~~~

The sending runs inside `activitypub-service`. Any number of copies of the
service may run: a row is handed to one of them at a time.

## Signing keys and signed requests

Whatever is sent in somebody's name is signed with a key of theirs, and the
other server checks it against the public half, which it reads from their
description. What the installation asks for in nobody's name, such as a
lookup, it signs as itself (`/ap/actor`): many servers answer nobody who
does not say who is asking.

The signature is over the request's target, its host, its date and, when it
has a body, a digest of the body (`activitypub-service/httpsig`). Each of
the four is one an attack goes through without, and each has a test. A
request dated more than fifteen minutes from now is refused.

**Behind a proxy, the `Host` the service is given must be the one that was
asked for.** The host is part of what is signed. The gateway and the
manifests here pass it on; a proxy put in front that does not will make
every signature fail, in both directions.

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
| `GET /activitypub/lookup?q=@alice@mastodon.example` | Finds somebody on another server, by handle or by the address of their profile, and keeps them |

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

A lookup answers with who was found:

~~~json
{
  "id": 170292,
  "type": "node.actor.activitypub-service.person.v1",
  "name": "Alice",
  "alias": "alice@mastodon.example",
  "body": "Grows **beans**",
  "uri": "https://mastodon.example/users/alice",
  "origin_domain": "mastodon.example",
  "url": "https://mastodon.example/@alice",
  "locked": true
}
~~~

or with why not: `400 lookup_unreadable` (not a handle or an https address),
`404 lookup_is_here` (somebody on this installation), `404 lookup_not_found`,
`404 lookup_not_a_person` (a group, a post), `502 lookup_failed` (the other
server could not be reached or understood), `429 too_many_lookups`.

**From the command line,** for whoever runs the service and wants to know
whether it and another server can reach and understand each other:

~~~sh
docker compose exec activitypub-service /app/app-binary lookup @alice@mastodon.example
~~~

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
| Signing and checking requests | `httpsig/` |
| Reading from other servers | `fetch/`, over `anahita-libs/safehttp` |
| Who somebody elsewhere is, and what is believed of it | `remote/` |
| Looking somebody up | `cmd/api/handlers/lookup.go`, table `activitypub_actors` |
| The inbox, and being followed | `cmd/api/handlers/inbox.go`, table `activitypub_follows` |
| Following from here | `cmd/api/handlers/events.go`, listening for `edge.follow.*` on the bus (`AMQP_ADDR`) |
| Posts going out | `cmd/api/handlers/notes.go`, `ap/note.go`, listening for `node.medium.*`, table `activitypub_objects` |
| Posts coming in | `cmd/api/handlers/inbox.go`: `received`, `withdrawn` |
| What is sent | `cmd/api/handlers/deliver.go`, `ap/activity.go`, table `activitypub_deliveries`, over `anahita-libs/federation` (`queue.go`) |
| The installation's own actor | `cmd/api/handlers/instance.go` |
| A person's choice | `cmd/api/handlers/settings.go`, table `activitypub_accounts` |
| The gateway's routes | `nginx/nginx.conf`: `/.well-known/webfinger`, `/.well-known/host-meta`, `/inbox`, `/ap/`, `/activitypub/` |
| `ACTIVITYPUB_LAB` | Lets the service fetch from private addresses. For [the lab](federation-lab.md) only; refused with `ENV=prod` |
| What it shares with the Bluesky service to come | `anahita-libs/federation`, `anahita-libs/site` |
