# Federation

How people here are found and followed from other servers of the fediverse,
and what an installation has to do for that to happen.

**This is being built.** What is described here is what works today: a
person can choose to be found, people follow each other across servers
either way round, their public notes travel with the follow, a person can
leave again, an administrator can refuse a server, and the web app has the
switch, the lookup, and shows people and posts from other servers.
Replies, likes, reposts and pictures do not travel yet, and the avatars of
people on other servers are not shown.

## Contents

- [What it is](#what-it-is)
- [Setting it up](#setting-it-up)
- [Keeping it running](#keeping-it-running)
- [When something does not work](#when-something-does-not-work)
- [Who is on the fediverse](#who-is-on-the-fediverse)
- [What a person is called](#what-a-person-is-called)
- [What other servers are told](#what-other-servers-are-told)
- [Looking somebody up on another server](#looking-somebody-up-on-another-server)
- [Being followed from another server](#being-followed-from-another-server)
- [Following somebody on another server](#following-somebody-on-another-server)
- [Posts going out](#posts-going-out)
- [Posts coming in](#posts-coming-in)
- [What can be done with what came from elsewhere](#what-can-be-done-with-what-came-from-elsewhere)
- [The inbox](#the-inbox)
- [What is sent, and when it does not arrive](#what-is-sent-and-when-it-does-not-arrive)
- [Signing keys and signed requests](#signing-keys-and-signed-requests)
- [The API](#the-api)
- [Leaving, and what follows a person's changes](#leaving-and-what-follows-a-persons-changes)
- [Servers that are refused](#servers-that-are-refused)
- [Where the code is](#where-the-code-is)

## What it is

Mastodon, GoToSocial, Pixelfed and many others speak one protocol,
ActivityPub, and somebody on any of them can follow somebody on any other.
`activitypub-service` makes this installation one of them. It is a service
of its own because it is the one that reads what the internet sends it, and
because an installation may not want it at all.

Bluesky speaks a different protocol and will be a different service.

## Setting it up

For whoever runs the installation. Federation is off until all of this is
done, and after that it is still off **for every person** until they turn
it on for themselves: sharing with other servers is each person's own
choice, and nothing an operator sets makes it for them.

### 1. Choose the names, for good

Two names are told to other servers and kept by them for ever:

| | Example | What it is |
| --- | --- | --- |
| The handle domain | `example.org` | The part of `ana@example.org` after the `@`. The host people know the site by, which is the web app's |
| The API's address | `https://api.example.org` | Where other servers fetch people and posts, and send what they have for people here |

**Change either afterwards and everybody here becomes somebody new on
every other server, with no followers.** Settle them before the first
person turns federation on. Try things out under a name you will throw
away, never the one you mean to keep (see step 8).

### 2. Be a site that can federate

- **Reachable over https at those names,** with a certificate other
  servers trust. [Deploying](deploying.md) covers the ingress and
  cert-manager.
- **Open to read:** `SITE_READ_ACCESS=public`, which is the default. A site
  for members, or one that shows visitors a preview, does not federate:
  federating would send out what it keeps in ([Privacy](privacy.md)).

### 3. Settings

On a site whose web app is at `https://DOMAIN_NAME` and whose API is at
`https://api.DOMAIN_NAME`, with `ENV=prod`, **nothing has to be set**: the
names are worked out. Otherwise say them
([Configuration](configuration.md#site-identity)):

~~~
PUBLIC_DOMAIN=example.org
PUBLIC_WEB_URL=https://example.org
PUBLIC_API_URL=https://api.example.org
~~~

`activitypub-service` also needs what the other services have, all of it
already in `config.env` and `secrets.env`:

| Setting | Why |
| --- | --- |
| `DSN` | Its own tables |
| `GRAPH_SERVICE_ADDR` | People, posts and follows are the graph's |
| `AMQP_ADDR` | It hears there that somebody here followed, posted or changed their profile |
| `REDIS_ADDR` | Limits on lookups and on what one address may send |
| `SECURE_COOKIE_KEY`, `OAUTH_ISSUER` | Knowing who is signed in, for a person's own switch |
| `CRYPT_BACKEND`, `CRYPT_LOCAL_KEY` | What each person's signing key is kept under. **The same values auth-service has** |

And tell NodeInfo, which auth-service serves, that the installation speaks
ActivityPub, so that other servers and the directories that list servers
know:

~~~
FEDERATION_PROTOCOLS=activitypub
~~~

### 4. Run it

- **The migrations,** as for any upgrade: the migration job adds the
  service's tables.
- **The service.** On Kubernetes it is in `k8s/base`
  (`services/activitypub.yml`); with docker-compose it is started by
  `make up`. One copy is enough to begin with, and any number may run.
- **The gateway** already sends it what is its own: `/.well-known/webfinger`,
  `/.well-known/host-meta`, `/inbox`, `/ap/` and `/activitypub/`. Rebuild the
  gateway's image with the rest.

At its start the service says which it is:

~~~
on the fediverse as example.org, reached at https://api.example.org
this installation is not on the fediverse: …
~~~

### 5. Have the web app's host send servers on to the API

A server that meets `ana@example.org` asks `example.org`, the web app's
host, who that is. That host has to send the question on:

| Asked of the web app's host | Sent on to |
| --- | --- |
| `/.well-known/webfinger?…` | `https://api.example.org/.well-known/webfinger?…`, with the same question |
| `/.well-known/host-meta` | `https://api.example.org/.well-known/host-meta` |
| `/.well-known/nodeinfo` | `https://api.example.org/.well-known/nodeinfo` |

- **With the web shell,** this is done: it redirects all three
  ([The web shell](web-shell.md#what-other-servers-ask-this-host)).
- **On a static host,** add three redirects (302, keeping the query). In
  nginx:

  ~~~nginx
  location /.well-known/ {
      add_header Access-Control-Allow-Origin * always;
      return 302 https://api.example.org$request_uri;
  }
  ~~~

**Whatever is in front of the API must pass on the `Host` it was asked
for.** The host is part of what every request between servers is signed
over; a proxy that replaces it makes every signature fail, both ways. The
gateway and the manifests here pass it on.

### 6. Check it, before anybody is told

From anywhere on the internet:

~~~sh
# The installation itself, which signs what it asks of other servers.
curl -s "https://example.org/.well-known/webfinger?resource=acct:example.org@example.org" -L
curl -s -H "Accept: application/activity+json" https://api.example.org/ap/actor

# What it says it speaks.
curl -s https://api.example.org/nodeinfo/2.1 | grep protocols
~~~

The first answers with a `subject` and a link to `/ap/actor`; the second
with an `Application` that has a `publicKey`. And that it can reach and be
understood by another server, with any account you know of on one:

~~~sh
docker compose exec activitypub-service /app/app-binary lookup @someone@mastodon.example
kubectl exec deploy/activitypub-service -- /app/app-binary lookup @someone@mastodon.example
~~~

It prints who was found. This is the one command that tries everything at
once: names, certificates, the signature, and the other server's opinion
of all three.

### 7. The first person

Signed in, somebody turns it on under **Settings › Privacy › Fediverse**.
Their profile has to be open to everybody. Then, from an account on
another server, search for their handle, `@ana@example.org`, and follow
them. A public note they post arrives there within seconds.

### 8. Trying it without going live

- **On one machine:** [the federation lab](federation-lab.md) runs this
  installation and another server side by side, with names and
  certificates of their own. Nothing leaves the machine.
- **Against real servers, before a site is live:** a tunnel from your
  machine to a hostname you will never use for anything else, with the
  three `PUBLIC_` settings saying that name. Remember that the servers you
  try it against keep the people they met under that name. Turning
  federation off for each of them (below) before the tunnel is closed asks
  those servers to remove them.

## Keeping it running

- **Back up `actor_keys`, and the key it is sealed with.** A person's
  signing key is what makes them who they are to other servers. Lose the
  table, or `CRYPT_LOCAL_KEY`, and every person here is somebody whose
  followers' servers refuse what they send. Both are in the backups an
  installation cannot do without.
- **Watch what could not be sent:**

  ~~~sql
  SELECT target, attempts, last_error, created_at
  FROM activitypub_deliveries
  WHERE status = 'failed'
  ORDER BY created_at DESC;
  ~~~

  A few are ordinary: servers close. Many to one server means it is down,
  or refuses this installation. Rows that failed can be deleted when they
  have been looked at.
- **Refuse a server** that sends abuse: see
  [Servers that are refused](#servers-that-are-refused).
- **The service's log** says why anything that arrived was refused
  (`inbox: … refused: …`), and what a lookup could not do.
- **Clocks.** A request dated more than fifteen minutes from now is
  refused, here and everywhere else. A server whose clock has drifted
  federates with nobody.

## When something does not work

| What is seen | Likely why |
| --- | --- |
| Nobody here is found by their handle from another server | The web app's host does not send `/.well-known/webfinger` on to the API (step 5). Try the `curl` of step 6 |
| One person is not found | They have not turned it on; or their profile is not open to everybody; or the other server asked earlier and is remembering the answer |
| The Fediverse card says the site has no address other servers can reach | The `PUBLIC_` settings, or `DOMAIN_NAME` and `ENV`, say `localhost`, a private address, or `http` (step 3) |
| The card says the site keeps what is on it to its members | `SITE_READ_ACCESS` is not `public` (step 2) |
| A follow from another server stays "requested" there | This installation's answer did not arrive: look at `activitypub_deliveries`. Or their request was refused: look at the log |
| Everything arriving is refused with "the signature is not this key's" | A proxy in front of the API is replacing `Host`, or rewriting the path |
| Everything arriving is refused with "date is missing or too far from now" | This server's clock, or theirs |
| `lookup` from the command line answers `lookup_failed` | The other server could not be reached, or refused the signed request. The log line above it says which |
| A note did not go out | It is not one that does ([Posts going out](#posts-going-out)); or the bus was down when it was posted. Editing it sends it |

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

## What can be done with what came from elsewhere

Somebody from another server, and what they post, are looked at here and
followed, and little else. The rules are the installation's own, in
`anahita-libs/cmd/api/permissions`, so every service keeps them:

| | Allowed? |
| --- | --- |
| Reading their profile, at `GET /people/<handle>` with the whole handle (`alice@mastodon.example`) | As for any public profile. It answers with `uri` and `origin_domain`, which nothing made here has |
| Following and unfollowing them, blocking them | Yes, through the same routes as for anybody |
| Changing their profile, their settings, their pictures | Nobody, an administrator included: every route that changes a person reads them as a person of this installation, and does not find them |
| Posting on their profile | Nobody |
| Changing a post of theirs | Nobody. An administrator may remove it from this installation |
| Liking or replying to a post of theirs | Not yet: it would never reach them |
| Reposting, saving a post of theirs | Yes; it stays here |

Their avatar is not shown yet: its address on their server is kept, and
the picture is to be copied here, so that no reader's browser calls
another server.

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
   for from here; `Create`, `Update` and `Delete` of a note; `Update` and
   `Delete` of the person themselves. Everything else is answered `202`
   and ignored.

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

## Leaving, and what follows a person's changes

**Leaving.** A person leaves the fediverse when they turn it off, and also
when they stop being somebody who may be on it: their profile is no longer
open to everybody, or they are disabled, archived, deleted or purged.
Either way:

- every server that knows them (the ones with a follower of theirs, and
  the ones with somebody they follow) is sent a `Delete` of the person,
  which is the one message the fediverse has for "forget them";
- here, the follows that crossed between servers are removed, both ways
  round, and the record of what they sent;
- from that moment they are not found.

**Leaving is for good, as far as other servers go.** A server told that
somebody is deleted removes them, their posts and their followers, and
remembers that it did. In the lab, GoToSocial marked the account suspended
and did not take the same person back when they turned federation on
again. So turning it on again makes a person findable here, and whether
any server they met before will know them is that server's choice. The web
app asks before it is turned off, and says that making a profile private
has the same effect.

**A profile changed.** When somebody on the fediverse changes their name,
what they say of themselves or their pictures, their followers' servers are
sent them as they are now (`Update`).

**From the other side.** A profile changed elsewhere is read again from
its own server, and not taken from the message that says so. An account
closed elsewhere is removed here: the person, what they posted, and every
follow between them and anybody here. Only they can say so of themselves,
and a stranger saying they are gone is taken without their server being
asked anything: a large server announces every closed account to every
server it knows.

## Servers that are refused

An administrator's list (`federation_domain_blocks`). A server on it:

- is not listened to: what it sends is answered `403` before its key is
  even fetched;
- is sent nothing, including what was queued for it before it was refused;
- cannot have anybody on it looked up.

A name covers every name under it: `example.net` refuses
`social.example.net` too.

**It does not remove what already came from that server.** People from it
who are kept here stay, with their posts and the follows between them and
people here; nothing more arrives and nothing more is sent. They are
removed one by one, as any post or person is.

| Request | Does |
| --- | --- |
| `GET /activitypub/blocks` | The list |
| `POST /activitypub/blocks` `{ "domain": "spam.example", "reason": "…" }` | Refuses a server, or changes why. The domain may be typed as a name, an address or a handle |
| `DELETE /activitypub/blocks/spam.example` | Stops refusing it |

Administrators only. The reason is for the administrators who come after,
and is never sent to the server refused. A change takes up to half a
minute to take hold in every copy of the service. **There is no screen for
this in the web app yet;** until there is, it is done through the API.

**How much one address may send** is limited too: 600 messages a minute to
the inbox, counted before anything is read, and answered `429` beyond
that.

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
| Leaving, and profile changes | `cmd/api/handlers/leaving.go`; from the other side, `inbox.go`: `forget`, `refreshed`; the graph's `DeleteRemoteActor` |
| Servers that are refused | `cmd/api/handlers/blocks.go`, `data/repositories/blocks.go`, table `federation_domain_blocks` |
| What NodeInfo says is spoken | `auth-service/cmd/api/handlers/node_info.go`: `federationProtocols` (`FEDERATION_PROTOCOLS`) |
| What may be done with what came from elsewhere | `anahita-libs/cmd/api/permissions` (`remote_test.go` holds the rule); `actor-service/cmd/api/handlers/person.go`: `personTypeOf` |
| What is sent | `cmd/api/handlers/deliver.go`, `ap/activity.go`, table `activitypub_deliveries`, over `anahita-libs/federation` (`queue.go`) |
| The installation's own actor | `cmd/api/handlers/instance.go` |
| A person's choice | `cmd/api/handlers/settings.go`, table `activitypub_accounts` |
| The gateway's routes | `nginx/nginx.conf`: `/.well-known/webfinger`, `/.well-known/host-meta`, `/inbox`, `/ap/`, `/activitypub/` |
| `ACTIVITYPUB_LAB` | Lets the service fetch from private addresses. For [the lab](federation-lab.md) only; refused with `ENV=prod` |
| What it shares with the Bluesky service to come | `anahita-libs/federation`, `anahita-libs/site` |
